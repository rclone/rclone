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

// Errors
var (
	errorMetaTooBig = errors.New("metadata file is too big")
)

const (
	dirIDC9r       = "dir.c9r"
	dirIDBackupC9r = "dirid.c9r"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cryptomator",
		Description: "Encrypt/Decrypt Cryptomator-format vaults",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to use as a Cryptomator vault.\n\nNormally should contain a ':' and a path, e.g. \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
			Required: true,
		}, {
			Name:       "password",
			Help:       "Password for Cryptomator vault.",
			IsPassword: true,
			Required:   true,
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

	f.features = (&fs.Features{}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)
	// Cryptomator's obfuscated directory structure can always support empty directories
	f.features.CanHaveEmptyDirectories = true

	password, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}
	err = f.loadOrCreateVault(ctx, password)
	if err != nil {
		return nil, err
	}
	f.Cryptor, err = NewCryptor(f.masterKey, f.vaultConfig.CipherCombo)
	if err != nil {
		return nil, err
	}

	// Make sure the root directory exists
	rootDirID, err := f.dirIDPath("")
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt root dir id: %w", err)
	}
	// TODO: make directory ID backup
	err = f.wrapped.Mkdir(ctx, rootDirID)
	if err != nil {
		return nil, fmt.Errorf("failed to create root dir at %q: %s", rootDirID, err)
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
			if err == fs.ErrorObjectNotFound {
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
}

// Fs wraps another fs and encrypts the directory
// structure, filenames, and file contents as outlined
// in https://docs.cryptomator.org/en/latest/security/architecture/
type Fs struct {
	wrapped  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features

	masterKey   MasterKey
	vaultConfig VaultConfig
	Cryptor

	dirCache *dircache.DirCache
}

// -------- fs.Info

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Cryptomator vault '%s:%s'", f.Name(), f.Root())
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return f.wrapped.Precision()
}

// Hashes returns nothing as the hashes returned by the backend would be of encrypted data, not plaintext
// TODO: does cryptomator have plaintext hashes readily available?
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	dirPath, err := f.dirIDPath(dirID)
	if err != nil {
		return nil, err
	}

	encryptedEntries, err := f.wrapped.List(ctx, dirPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range encryptedEntries {
		encryptedFilename := path.Base(entry.Remote())
		encryptedFilename, ok := strings.CutSuffix(encryptedFilename, ".c9r")
		if !ok {
			continue
		}
		if encryptedFilename == "dirid" {
			continue
		}
		filename, err := f.DecryptFilename(encryptedFilename, dirID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt filename %q: %w", encryptedFilename, err)
		}
		remote := path.Join(dir, filename)

		switch entry := entry.(type) {
		case fs.Directory:
			entries = append(entries, &Directory{
				Directory: entry,
				fs:        f,
				remote:    remote,
			})
		case fs.Object:
			entries = append(entries, &DecryptingObject{
				Object:    entry,
				f:         f,
				decRemote: remote,
			})
		default:
			return nil, fmt.Errorf("unknown entry type %T", entry)
		}
	}
	return
}

// FindLeaf finds a child of name leaf in the directory with id pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	encryptedPath, err := f.leafPath(leaf, pathID)
	if err != nil {
		return
	}
	subdirIDFile := path.Join(encryptedPath, dirIDC9r)
	subdirID, err := f.readSmallFile(ctx, subdirIDFile, 100)
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
func (f *Fs) CreateDir(ctx context.Context, pathID string, leaf string) (newID string, err error) {
	leafPath, err := f.leafPath(leaf, pathID)
	if err != nil {
		return
	}
	newID = uuid.NewString()
	dirPath, err := f.dirIDPath(newID)
	if err != nil {
		return
	}

	// Put directory ID backup file, thus creating the directory
	data := f.encryptReader(bytes.NewBuffer([]byte(newID)))
	info := object.NewStaticObjectInfo(path.Join(dirPath, dirIDBackupC9r), time.Now(), -1, true, nil, nil)
	_, err = f.wrapped.Put(ctx, data, info)
	if err != nil {
		return
	}

	// Write pointer to directory
	// XXX if someone else attempts to create the same directory at the same time, one of them will win and the other will get an orphaned directory.
	// Without an atomic "create if not exists" for this next writeSmallFile operation, this can't be fixed.
	err = f.writeSmallFile(ctx, path.Join(leafPath, dirIDC9r), []byte(newID))
	if err != nil {
		return
	}

	return
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fmt.Errorf("failed to find ID for directory %q: %w", dir, err)
	}
	var parentID string
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
		// Finally the pointer to the directory. First the file
		dirPointerFile string
		// Then the directory containing the pointer
		dirPointerPath string
	)
	dirPath, err = f.dirIDPath(dirID)
	if err != nil {
		return err
	}
	dirIDBackup = path.Join(dirPath, dirIDBackupC9r)
	dirPointerPath, err = f.leafPath(leaf, parentID)
	if err != nil {
		return err
	}
	dirPointerFile = path.Join(dirPointerPath, dirIDC9r)

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
	// dirPointerFile
	obj, err = f.wrapped.NewObject(ctx, dirPointerFile)
	if err == nil {
		err = obj.Remove(ctx)
	}
	// dirPointerPath
	if err == nil {
		err = f.wrapped.Rmdir(ctx, dirPointerPath)
	}
	if err != nil {
		return fmt.Errorf("couldn't rmdir dir pointer %q: %w", dirPointerFile, err)
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// -------- fs.Directory

// Directory wraps the underlying fs.Directory and handles filename encryption and so on
type Directory struct {
	fs.Directory
	fs     *Fs
	remote string
}

// Fs returns read only access to the Fs that this object is part of
func (d *Directory) Fs() fs.Info { return d.fs }

// Remote returns the decrypted remote path
func (d *Directory) Remote() string { return d.remote }

// String returns a description of the Object
func (d *Directory) String() string {
	if d == nil {
		return "<nil>"
	}
	return d.remote
}

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
	encryptedPath, err := f.leafPath(leaf, dirID)
	if err != nil {
		return nil, err
	}
	wrappedObj, err := f.wrapped.NewObject(ctx, encryptedPath)
	if err != nil {
		return nil, err
	}
	return &DecryptingObject{
		Object:    wrappedObj,
		f:         f,
		decRemote: remote,
	}, nil
}

// DecryptingObject wraps the underlying fs.Object and handles decrypting it
type DecryptingObject struct {
	fs.Object
	f         *Fs
	decRemote string
}

// TODO: override all relevant methods

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
	return o.f.DecryptedFileSize(o.Object.Size())
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

	reader, err := o.Object.Open(ctx, options...)
	defer func() {
		if err != nil && reader != nil {
			_ = reader.Close()
		}
	}()
	if err != nil {
		return nil, err
	}

	var decryptReader io.Reader
	decryptReader, err = o.f.NewReader(reader)
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
		encRemote:  o.Object.Remote(),
	}
	return o.Object.Update(ctx, encIn, encSrc, options...)
}

// Hash returns no checksum as it is not possible to quickly obtain a hash of the plaintext of an encrypted file
func (o *DecryptingObject) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// -------- Put

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
	encIn := f.encryptReader(in)
	leaf, dirID, err := f.dirCache.FindPath(ctx, src.Remote(), true)
	if err != nil {
		return nil, err
	}
	encRemotePath, err := f.leafPath(leaf, dirID)
	if err != nil {
		return nil, err
	}
	encSrc := &EncryptingObjectInfo{
		ObjectInfo: src,
		f:          f,
		encRemote:  encRemotePath,
	}

	obj, err := f.wrapped.Put(ctx, encIn, encSrc, options...)
	if obj != nil {
		obj = &DecryptingObject{
			Object:    obj,
			f:         f,
			decRemote: src.Remote(),
		}
	}
	return obj, err
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
	return i.f.EncryptedFileSize(i.ObjectInfo.Size())
}

// Hash returns no checksum as it is not possible to quickly obtain a hash of the plaintext of an encrypted file
func (i *EncryptingObjectInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// -------- private

// dirIDPath returns the encrypted path to the directory with a given ID.
func (f *Fs) dirIDPath(dirID string) (string, error) {
	encryptedDirID, err := f.EncryptDirID(dirID)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt directory ID: %w", err)
	}
	dirPath := path.Join("d", encryptedDirID[:2], encryptedDirID[2:])
	// TODO: verify that dirid.c9r inside the directory contains dirID
	return dirPath, nil
}

// leafPath returns the encrypted path to a leaf node with the given name in the directory with the given ID.
func (f *Fs) leafPath(leaf, dirID string) (p string, err error) {
	dirPath, err := f.dirIDPath(dirID)
	if err != nil {
		return
	}
	encryptedFilename, err := f.EncryptFilename(leaf, dirID)
	if err != nil {
		return
	}
	return path.Join(dirPath, encryptedFilename+".c9r"), nil
}

// encryptReader returns a reader that produces an encrypted version of the data in r, suitable for storing directly in the wrapped filesystem.
func (f *Fs) encryptReader(r io.Reader) io.Reader {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		encWriter, err := f.NewWriter(pipeWriter)
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
		return nil, errorMetaTooBig
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
	_ fs.Fs = (*Fs)(nil)
)
