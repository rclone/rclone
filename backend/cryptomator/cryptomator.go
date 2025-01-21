// Package cryptomator provides wrappers for Fs and Object which implement Cryptomator encryption
package cryptomator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/dircache"
)

const (
	dirIDC9r       = "dir.c9r"
	fileContentC9r = "contents.c9r"
	symlinkC9r     = "symlink.c9r"
	fullNameC9s    = "name.c9s"

	dirIDBackupC9r = "dirid.c9r"
)

const (
	maxLenNameC9s  = 10 * 1024 // quoth cryptomator "no sane person gives a file a 10kb long name."
	maxLenDirIDC9r = 1000
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cryptomator",
		Description: "Encrypt/Decrypt Cryptomator-format vaults",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remote is read and written.`,
		},
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to use as a Cryptomator vault.\n\nNormally should contain a ':' and a path, e.g. \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
			Required: true,
		}, {
			Name:       "password",
			Help:       "Password for Cryptomator vault.",
			IsPassword: true,
			Required:   true,
		}, {
			Name:     "auto_create",
			Help:     "Whether to automatically create a vault if none exists at the underlying remote.",
			Advanced: true,
			Default:  false,
		}, {
			Name: "auto_create_shortening_threshold",
			Help: `If auto create is enabled, the setting to use for minimum filename length that triggers out-of-band storage of the filename.

This vault setting is normally read out of the vault config file on the remote. It can only be configured when the vault is created, because changing it may make the vault unreadable. Therefore this option has no effect if auto create is disabled, or it is enabled and the the vault already exists.

The out-of-band filename storage works by replacing a file whose name is too long with a directory named using the SHA1 hash of the encrypted filename, and storying the original encrypted filename in a text file. For details, read the Cryptomator documentation on [Name Shortening](https://docs.cryptomator.org/security/vault/#name-shortening).`,
			Advanced: true,
			Default:  newDefaultVaultConfig().ShorteningThreshold,
		}, {
			Name: "auto_create_vault_format_version",
			Help: `If auto create is enabled, the version of the vault format to use.

This vault setting is normally read out of the vault config file on the remote. It can only be configured when the vault is created, because changing it may make the vault unreadable. Therefore this option has no effect if auto create is disabled, or it is enabled and the the vault already exists.

It is not recommended to intentionally choose an old vault format version. This option is mainly useful for testing Rclone itself. Rclone supports vault formats 8 and 7.`,
			Advanced: true,
			Default: newDefaultVaultConfig().Format,
		}},
	})
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	remote := opt.Remote
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point cryptomator remote at itself - check the value of the remote setting")
	}

	wrappedFs, err := cache.Get(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("failed to make remote %q to wrap: %w", remote, err)
	}

	// Remove slashes on start or end, which would otherwise confuse the dirCache (as is documented on dircache.SplitPath).
	root = strings.Trim(root, "/")

	f := &Fs{
		wrapped: wrappedFs,
		name:    name,
		root:    root,
		opt:     *opt,
	}
	cache.PinUntilFinalized(f.wrapped, f)

	f.features = (&fs.Features{
		CanHaveEmptyDirectories:  true,
		SetTier:                  true,
		GetTier:                  true,
		ReadMetadata:             true,
		WriteMetadata:            true,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         true,
		UserDirMetadata:          true,
		DirModTimeUpdatesOnWrite: true,
		PartialUploads:           true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)
	// Cryptomator's obfuscated directory structure can always support empty directories
	f.features.CanHaveEmptyDirectories = true

	password, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}
	err = f.loadVault(ctx, password)
	if errors.Is(err, fs.ErrorObjectNotFound) && opt.AutoCreate {
		// Vault does not exist, so create it
		vaultConfig := newDefaultVaultConfig()
		vaultConfig.ShorteningThreshold = opt.AutoCreateShorteningThreshold
		err = f.createVault(ctx, password, vaultConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create new vault: %w", err)
		}
		err = f.loadVault(ctx, password)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load vault: %w", err)
	}
	f.cryptor, err = newCryptor(f.masterKey, f.vaultConfig.CipherCombo)
	if err != nil {
		return nil, err
	}

	// Make sure the root directory exists
	rootDir, err := f.makeDirIDPath(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create root dir at %q: %s", rootDir, err)
	}

	f.dirCache = dircache.New(root, "", f)
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, "", &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}

		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			if errors.Is(err, fs.ErrorObjectNotFound) {
				// File doesn't exist so return old f
				return f, nil
			}

			return nil, fmt.Errorf("incomprehensible error while checking for whether the root at %q is a file: %w", root, err)
		}

		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Options defines the configuration for this backend
type Options struct {
	Remote   string `config:"remote"`
	Password string `config:"password"`

	AutoCreate                    bool `config:"auto_create"`
	AutoCreateShorteningThreshold int  `config:"auto_create_shortening_threshold"`
}

// Fs wraps another fs and encrypts the directory
// structure, filenames, and file contents as outlined
// in https://docs.cryptomator.org/security/architecture/
type Fs struct {
	wrapped  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features
	wrapper  fs.Fs

	masterKey   masterKey
	vaultConfig vaultConfig
	cryptor

	dirCache *dircache.DirCache
}

// ShorteningThreshold returns the shortening threshold out of the vault config, used for a test
func (f *Fs) ShorteningThreshold() int { return f.vaultConfig.ShorteningThreshold }

// -------- fs.Info

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Cryptomator vault '%s:%s'", f.Name(), f.Root())
}

// Precision of the remote
func (f *Fs) Precision() time.Duration { return f.wrapped.Precision() }

// Hashes returns nothing as the hashes returned by the backend would be of encrypted data, not plaintext
// TODO: does cryptomator have plaintext hashes readily available?
func (f *Fs) Hashes() hash.Set { return hash.NewHashSet() }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// -------- Directories

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	dirPath := f.dirIDPath(dirID)

	encryptedEntries, err := f.wrapped.List(ctx, dirPath)
	if err != nil {
		return nil, err
	}
	return f.decryptListEntries(ctx, dir, dirID, encryptedEntries)
}

// ListP lists the objects and directories of the Fs starting
// from dir non recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	do := f.wrapped.Features().ListP
	if do == nil {
		return errorNotSupportedByUnderlyingRemote
	}

	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}
	dirPath := f.dirIDPath(dirID)
	return do(ctx, dirPath, func(encryptedEntries fs.DirEntries) error {
		entries, err := f.decryptListEntries(ctx, dir, dirID, encryptedEntries)
		if err != nil {
			return err
		}
		return callback(entries)
	})
}

func (f *Fs) decryptListEntries(ctx context.Context, dir string, dirID string, encryptedEntries fs.DirEntries) (entries fs.DirEntries, err error) {
	for _, entry := range encryptedEntries {
		node := node{path: entry.Remote()}
		node.fullName = path.Base(node.path)
		if node.short() {
			fullname, err := f.readSmallFile(ctx, node.c9sFullName(), maxLenNameC9s)
			// if errors.Is(err, fs.ErrorObjectNotFound) {
			// 	fs.Logf(entry, "deflated filename with no full filename, skipping")
			// 	continue
			// }
			if err != nil {
				return nil, fmt.Errorf("failed to read full filename of %v: %w", node, err)
			}
			node.fullName = string(fullname)
		}

		encFilename, ok := strings.CutSuffix(node.fullName, ".c9r")
		if !ok {
			continue
		}
		if encFilename == "dirid" {
			continue
		}
		filename, err := f.decryptFilename(encFilename, dirID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt filename %q: %w", encFilename, err)
		}
		decRemote := path.Join(dir, filename)

		// What kind of entry is this?

		if entry, ok := entry.(fs.Object); ok {
			// The only easy case - file with short filename.
			entries = append(entries, f.newDecryptingObject(entry, node, decRemote))
			continue
		}

		// Now that we know the underlying entry is a directory, the encrypted entry could still be anything: a directory, a symlink, or a file with a shortened name.

		// Is it a directory?
		dirID, err := f.readSmallFile(ctx, node.c9rDirID(), maxLenDirIDC9r)
		if err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
			return nil, fmt.Errorf("failed to read directory id: %w", err)
		}
		if err == nil {
			// It's a directory!
			dirIDPath := f.dirIDPath(string(dirID))

			// Turning that path into an fs.Directory is really annoying. The only thing in the standard Fs interface that returns fs.Directory objects is List, so we have to list the parent.
			// TODO: scaling problem? this List will take time proportional to the total number of directories in the entire vault...divided by 256
			dirIDParent, dirIDLeaf := path.Split(dirIDPath)
			subEntries, err := f.wrapped.List(ctx, dirIDParent)
			if err != nil {
				return nil, err
			}
			var realDir fs.Directory
			for i := range subEntries {
				dir, ok := subEntries[i].(fs.Directory)
				if ok && path.Base(dir.Remote()) == dirIDLeaf {
					realDir = dir
					break
				}
			}
			if realDir == nil {
				err = fmt.Errorf("couldn't find %q in listing of %q (has directory been removed?)", dirIDLeaf, dirIDParent)
			}
			if err != nil {
				return nil, err
			}
			entries = append(entries, &Directory{DirWrapper: fs.NewDirWrapper(decRemote, realDir), f: f})
			continue
		}

		// Is it a symlink?
		_, err = f.wrapped.NewObject(ctx, node.c9rSymlinkTarget())
		if err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
			return nil, fmt.Errorf("failed to open symlink target: %w", err)
		}
		if err == nil {
			// Ignore symlinks, like the default behavior of the local backend.
			continue
		}

		// Surely then it's a file?
		fileContents, err := f.wrapped.NewObject(ctx, node.c9rFileContents())
		if err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
			return nil, fmt.Errorf("failed to open file contents: %w", err)
		}
		if err == nil {
			entries = append(entries, f.newDecryptingObject(fileContents, node, decRemote))
			continue
		}

		//fs.Logf(entry, "node of unknown type, skipping")
		return nil, fmt.Errorf("couldn't determine type of %q: %w", decRemote, err)
	}
	return
}

// FindLeaf finds a child of name leaf in the directory with id pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	subdirIDFile := f.node(leaf, pathID).c9rDirID()
	subdirID, err := f.readSmallFile(ctx, subdirIDFile, maxLenDirIDC9r)
	if errors.Is(err, fs.ErrorObjectNotFound) {
		// If the directory doesn't exist, return found=false and no error to let the DirCache create the directory if it wants.
		err = nil
		return
	}
	if err != nil {
		err = fmt.Errorf("failed to read ID of subdir from %q: %w", subdirIDFile, err)
		return
	}
	pathIDOut = string(subdirID)
	found = true
	return
}

// CreateDir creates a directory at the request of the DirCache
func (f *Fs) CreateDir(ctx context.Context, parentID string, leafName string) (newID string, err error) {
	node := f.node(leafName, parentID)
	newID = uuid.NewString()

	// Create the directory
	_, err = f.makeDirIDPath(ctx, newID)
	if err != nil {
		return
	}

	// Write pointer to directory
	// XXX if someone else attempts to create the same directory at the same time, one of them will win and the other will get an orphaned directory.
	// Without an atomic "create if not exists" for this next writeSmallFile operation, this can't be fixed.
	err = f.writeNodeLongName(ctx, node)
	if err != nil {
		return
	}
	err = f.writeSmallFile(ctx, node.c9rDirID(), []byte(newID))
	return
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// MkdirMetadata makes the directory passed in as dir.
//
// It shouldn't return an error if it already exists.
//
// If the metadata is not nil it is set.
//
// It returns the directory that was created.
func (f *Fs) MkdirMetadata(ctx context.Context, dirPath string, metadata fs.Metadata) (dir fs.Directory, err error) {
	do := f.wrapped.Features().MkdirMetadata
	if do == nil {
		return nil, errorNotSupportedByUnderlyingRemote
	}

	// First create the directory normally, then call MkdirMetadata to update its metadata.
	// This is for a really silly reason: if you call MkdirMetadata first, creating dirid.c9r will reset the mtime! which is one of the things that can be set in the metadata.
	dirID, err := f.dirCache.FindDir(ctx, dirPath, true)
	if err != nil {
		return nil, err
	}

	dir, err = do(ctx, f.dirIDPath(dirID), metadata)
	if dir != nil {
		dir = &Directory{DirWrapper: fs.NewDirWrapper(dirPath, dir), f: f}
	}
	if err != nil {
		return
	}

	return
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fmt.Errorf("failed to find ID for directory %q: %w", dir, err)
	}
	leaf, parentID, err := f.dirCache.FindPath(ctx, dir, false)
	if err != nil {
		return fmt.Errorf("failed to find ID for parent of directory %q: %w", dir, err)
	}

	// These need to get deleted, in this order
	var (
		// The dirid.c9r backup is likely in every directory and needs to be deleted before the directory.
		dirIDBackup string
		// Now the directory. But, if this fails (e.g. due to the directory not being empty), we need to go recreate the dir ID backup!
		dirPath string
		// Finally the pointer to the directory.
		dirPointer node
	)
	dirPath = f.dirIDPath(dirID)
	dirIDBackup = path.Join(dirPath, dirIDBackupC9r)
	dirPointer = f.node(leaf, parentID)

	// Quick check for if the directory is empty - someone else could create a file between this and the final rmdir, so we still need that code that recreates the dir ID backup!
	entries, err := f.wrapped.List(ctx, dirPath)
	if err != nil {
		return err
	}
	empty := true
	for _, entry := range entries {
		if path.Base(entry.Remote()) != dirIDBackupC9r {
			empty = false
			break
		}
	}
	if !empty {
		return fs.ErrorDirectoryNotEmpty
	}

	// Now delete them
	// dirIDBackup
	obj, err := f.wrapped.NewObject(ctx, dirIDBackup)
	if err == nil {
		err = obj.Remove(ctx)
	}
	if err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
		return fmt.Errorf("couldn't remove dir id backup: %w", err)
	}
	// dirPath
	err = f.wrapped.Rmdir(ctx, dirPath)
	if err != nil {
		err = fmt.Errorf("failed to rmdir: %w", err)
		// put the directory ID backup back!
		data := f.encryptReader(bytes.NewBuffer([]byte(dirID)))
		info := object.NewStaticObjectInfo(path.Join(dirPath, dirIDBackupC9r), time.Now(), -1, true, nil, nil)
		_, err2 := f.wrapped.Put(ctx, data, info)
		if err2 != nil {
			err = fmt.Errorf("%w (also failed to restore dir id backup: %w)", err, err2)
		}
		return err
	}
	// dirPointer
	obj, err = f.wrapped.NewObject(ctx, dirPointer.c9rDirID())
	if err == nil {
		err = obj.Remove(ctx)
	}
	if err == nil {
		err = f.deleteNodeLongName(ctx, dirPointer)
	}
	// If it was shortened, deleteNodeLongName already removed the directory
	if !dirPointer.short() && err == nil {
		err = f.wrapped.Rmdir(ctx, dirPointer.path)
	}
	if err != nil {
		return fmt.Errorf("couldn't rmdir dir pointer %q: %w", dirPointer, err)
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// -------- fs.Directory

// Directory wraps the underlying fs.Directory, the one named with a hash of the encrypted directory ID that contains the subnodes and the dirid.c9r backup, not the little directory in its parent that just has dir.c9r.
type Directory struct {
	*fs.DirWrapper
	f *Fs
}

// Fs returns read only access to the Fs that this object is part of
func (d *Directory) Fs() fs.Info { return d.f }

// -------- Objects

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if errors.Is(err, fs.ErrorDirNotFound) {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, fmt.Errorf("failed to find ID for directory of file %q: %w", remote, err)
	}

	node := f.node(leaf, dirID)
	encContentsPath := node.c9rFileContents()
	wrappedObj, err := f.wrapped.NewObject(ctx, encContentsPath)
	if err != nil {
		return nil, err
	}
	return f.newDecryptingObject(wrappedObj, node, remote), nil
}

// DecryptingObject wraps the underlying fs.Object and handles decrypting it
type DecryptingObject struct {
	encObj    fs.Object
	f         *Fs
	node      node
	decRemote string
}

func (f *Fs) newDecryptingObject(o fs.Object, node node, decRemote string) *DecryptingObject {
	return &DecryptingObject{
		encObj:    o,
		f:         f,
		node:      node,
		decRemote: decRemote,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *DecryptingObject) Fs() fs.Info { return o.f }

// Remote returns the decrypted remote path
func (o *DecryptingObject) Remote() string { return o.decRemote }

// String returns a description of the object
func (o *DecryptingObject) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Size returns the size of the object after being decrypted
func (o *DecryptingObject) Size() int64 {
	return o.f.decryptedFileSize(o.encObj.Size())
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
//
// This calls Open on the object of the underlying remote with fs.SeekOption
// and fs.RangeOption removes. This is strictly necessary as the file header
// contains all the information to decrypt the file.
//
// TODO: Since the files are encrypted in 32kb chunks, it would be possible to
// support real seek and range requests. However, it would be necessary to make
// two requests, one for the file header and one for the requested range.
//
// We wrap the reader of the underlying object to decrypt the data.
// - For fs.SeekOption we just discard all the bytes until we reach the Offset
// - For fs.RangeOption we do the same and then wrap the reader in io.LimitReader
func (o *DecryptingObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var offset, limit int64 = 0, -1
	var newOptions []fs.OpenOption
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			newOptions = append(newOptions, option)
		}
	}
	options = newOptions

	reader, err := o.encObj.Open(ctx, options...)
	defer func() {
		if err != nil && reader != nil {
			_ = reader.Close()
		}
	}()
	if err != nil {
		return nil, err
	}

	var decryptReader io.Reader
	decryptReader, err = o.f.newReader(reader)
	if err != nil {
		return nil, err
	}

	if _, err = io.CopyN(io.Discard, decryptReader, offset); err != nil {
		return nil, err
	}

	if limit != -1 {
		decryptReader = io.LimitReader(decryptReader, limit)
	}

	return struct {
		io.Reader
		io.Closer
	}{
		Reader: decryptReader,
		Closer: reader,
	}, nil
}

// Remove this object
func (o *DecryptingObject) Remove(ctx context.Context) error {
	err := o.encObj.Remove(ctx)
	if err != nil {
		return err
	}
	err = o.f.deleteNodeLongName(ctx, o.node)
	if err != nil {
		fs.LogPrintfCtx(ctx, fs.LogLevelWarning, o, "deleting succeeded, but then failed to remove long file name: %v", err)
	}
	return nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *DecryptingObject) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	encIn := o.f.encryptReader(in)
	encSrc := &EncryptingObjectInfo{
		ObjectInfo: src,
		f:          o.f,
		encRemote:  o.encObj.Remote(),
	}
	return o.encObj.Update(ctx, encIn, encSrc, options...)
}

// Hash returns no checksum as it is not possible to quickly obtain a hash of the plaintext of an encrypted file
func (o *DecryptingObject) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// -------- Put

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	encIn := f.encryptReader(in)
	leaf, dirID, err := f.dirCache.FindPath(ctx, src.Remote(), true)
	if err != nil {
		return nil, err
	}
	node := f.node(leaf, dirID)
	err = f.writeNodeLongName(ctx, node)
	if err != nil {
		return nil, err
	}
	encRemotePath := node.c9rFileContents()
	encSrc := &EncryptingObjectInfo{
		ObjectInfo: src,
		f:          f,
		encRemote:  encRemotePath,
	}

	obj, err := put(ctx, encIn, encSrc, options...)
	if obj != nil {
		obj = f.newDecryptingObject(obj, node, src.Remote())
	}
	return obj, err
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options, f.wrapped.Put)
}

// PutUnchecked uploads to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
//
// May create duplicates or return errors if src already
// exists.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.wrapped.Features().PutUnchecked
	if do == nil {
		return nil, errorNotSupportedByUnderlyingRemote
	}
	return f.put(ctx, in, src, options, do)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.wrapped.Features().PutStream
	if do == nil {
		return nil, errorNotSupportedByUnderlyingRemote
	}
	return f.put(ctx, in, src, options, do)
}

// EncryptingObjectInfo wraps the ObjectInfo provided to Put and transforms its attributes to match the encrypted version of the file.
type EncryptingObjectInfo struct {
	fs.ObjectInfo
	f         *Fs
	encRemote string
}

// Fs returns read only access to the Fs that this object is part of
func (i *EncryptingObjectInfo) Fs() fs.Info { return i.f }

// Remote returns the encrypted remote path
func (i *EncryptingObjectInfo) Remote() string { return i.encRemote }

// String returns a description of the Object
func (i *EncryptingObjectInfo) String() string {
	if i == nil {
		return "<nil>"
	}
	return i.encRemote
}

// Size returns the size of the object after being encrypted
func (i *EncryptingObjectInfo) Size() int64 {
	return i.f.encryptedFileSize(i.ObjectInfo.Size())
}

// Hash returns no checksum as it is not possible to quickly obtain a hash of the plaintext of an encrypted file
func (i *EncryptingObjectInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Copy src to this remote using server-side copy operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
//
// Cryptomator: Can just pass through the copy operation, since the encryption of file contents is independent of the directory.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.wrapped.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	srcObj, ok := src.(*DecryptingObject)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	node := f.node(leaf, dirID)
	err = f.writeNodeLongName(ctx, node)
	if err != nil {
		return nil, err
	}
	encryptedPath := node.c9rFileContents()
	dst, err := do(ctx, srcObj.encObj, encryptedPath)
	if dst != nil {
		dst = f.newDecryptingObject(dst, node, remote)
	}
	return dst, err
}

// Move src to this remote using server-side move operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
//
// Cryptomator: Can just pass through the move operation, since the encryption of file contents is independent of the directory.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.wrapped.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	srcObj, ok := src.(*DecryptingObject)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}
	node := f.node(leaf, dirID)
	err = f.writeNodeLongName(ctx, node)
	if err != nil {
		return nil, err
	}
	encryptedPath := node.c9rFileContents()
	dst, err := do(ctx, srcObj.encObj, encryptedPath)
	if err != nil {
		return nil, err
	}
	dst = f.newDecryptingObject(dst, node, remote)
	err = f.deleteNodeLongName(ctx, srcObj.node)
	if err != nil {
		fs.LogPrintfCtx(ctx, fs.LogLevelWarning, srcObj, "moving to %s succeeded, but then failed to remove long file name: %v", dst, err)
	}
	return dst, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	do := f.wrapped.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Errorf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	if srcFs.vaultConfig.ID != f.vaultConfig.ID {
		fs.Errorf(srcFs, "Can't move directory - not same cryptomator vault")
		return fs.ErrorCantDirMove
	}
	// TODO: It would be almost as easy to implement this operation without server-side support, by deleting and recreating the dir.c9r file (though it wouldn't be atomic.)
	_, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	srcNode := f.node(srcLeaf, srcDirectoryID)
	dstNode := f.node(dstLeaf, dstDirectoryID)
	err = do(ctx, srcFs.wrapped, srcNode.path, dstNode.path)
	if err != nil {
		return err
	}

	if dstNode.short() {
		err = f.writeNodeLongName(ctx, dstNode)
	} else if srcNode.short() {
		// get rid of name.c9s residue, manually...
		var obj fs.Object
		obj, err = f.wrapped.NewObject(ctx, path.Join(dstNode.path, fullNameC9s))
		if err == nil {
			err = obj.Remove(ctx)
		}
	}
	if err != nil {
		fs.LogPrintfCtx(ctx, fs.LogLevelWarning, srcNode, "moving to %s succeeded, but then failed to update long file name: %v", dstNode, err)
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// -------- private

// dirIDPath returns the encrypted path to the directory with a given ID.
func (f *Fs) dirIDPath(dirID string) string {
	encryptedDirID := f.encryptDirID(dirID)
	dirPath := path.Join("d", encryptedDirID[:2], encryptedDirID[2:])
	// TODO: verify that dirid.c9r inside the directory contains dirID
	return dirPath
}

// makeDirIDPath creates the directory at the encrypted path for the given ID and returns the path.
func (f *Fs) makeDirIDPath(ctx context.Context, dirID string) (dirPath string, err error) {
	dirPath = f.dirIDPath(dirID)
	data := f.encryptReader(bytes.NewBuffer([]byte(dirID)))
	info := object.NewStaticObjectInfo(path.Join(dirPath, dirIDBackupC9r), time.Now(), -1, true, nil, nil)
	_, err = f.wrapped.Put(ctx, data, info)
	return
}

// node represents a filesystem node as it is represented in the underlying storage, as either a .c9r node with encrypted name, or a .c9s node with sha1 of encrypted name.
type node struct {
	// Path from the root of the vault to the node.
	path string
	// Original filename before c9s shortening, "" if unknown
	fullName string
}

func (n node) String() string {
	return n.path
}

func (n node) c9sFullName() string {
	if n.short() {
		return path.Join(n.path, fullNameC9s)
	}
	return ""
}

func (n node) c9rDirID() string {
	return path.Join(n.path, dirIDC9r)
}

func (n node) c9rSymlinkTarget() string {
	return path.Join(n.path, symlinkC9r)
}

func (n node) c9rFileContents() string {
	if n.short() {
		return path.Join(n.path, fileContentC9r)
	}
	return n.path
}

func (n node) short() bool {
	return strings.HasSuffix(n.path, ".c9s")
}

// node returns the representation of a node with the given name in the directory with the given ID.
func (f *Fs) node(name, dirID string) node {
	dirPath := f.dirIDPath(dirID)
	encryptedFilename := f.encryptFilename(name, dirID) + ".c9r"
	basename := encryptedFilename
	if len(encryptedFilename) > f.ShorteningThreshold() {
		basename = deflateFilename(encryptedFilename) + ".c9s"
	}
	return node{path: path.Join(dirPath, basename), fullName: encryptedFilename}
}

func (f *Fs) writeNodeLongName(ctx context.Context, node node) (err error) {
	if node.short() {
		err = f.writeSmallFile(ctx, node.c9sFullName(), []byte(node.fullName))
	}
	return
}

func (f *Fs) deleteNodeLongName(ctx context.Context, node node) error {
	if node.short() {
		obj, err := f.wrapped.NewObject(ctx, node.c9sFullName())
		if err != nil {
			return err
		}
		err = obj.Remove(ctx)
		if err != nil {
			return err
		}
		err = f.wrapped.Rmdir(ctx, node.path)
		if err != nil {
			return err
		}
	}
	return nil
}

// encryptReader returns a reader that produces an encrypted version of the data in r, suitable for storing directly in the wrapped filesystem.
func (f *Fs) encryptReader(r io.Reader) io.Reader {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		encWriter, err := f.newWriter(pipeWriter)
		if err != nil {
			pipeWriter.CloseWithError(err)
			return
		}

		if _, err = io.Copy(encWriter, r); err != nil {
			pipeWriter.CloseWithError(err)
			return
		}

		pipeWriter.CloseWithError(encWriter.Close())
	}()

	return pipeReader
}

// readSmallFile reads a file in full from the wrapped filesystem and returns it as bytes.
func (f *Fs) readSmallFile(ctx context.Context, path string, maxLen int64) ([]byte, error) {
	obj, err := f.wrapped.NewObject(ctx, path)
	if err != nil {
		return nil, err
	}
	if obj.Size() > maxLen {
		return nil, fmt.Errorf("file %s exceeds length limit %d", path, maxLen)
	}
	reader, err := obj.Open(ctx)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	return data, err
}

// writeSmallFile writes a byte slice to a file in the wrapped filesystem.
func (f *Fs) writeSmallFile(ctx context.Context, path string, data []byte) error {
	info := object.NewStaticObjectInfo(path, time.Now(), int64(len(data)), true, nil, nil)
	_, err := f.wrapped.Put(ctx, bytes.NewReader(data), info)
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.MkdirMetadataer = (*Fs)(nil)
	_ fs.ListPer         = (*Fs)(nil)
	// TODO: implement OpenChunkWriter. It's entirely possible to encrypt chunks of a file in parallel.
)
