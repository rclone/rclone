// Package cryptomator provides wrappers for Fs and Object which implement Cryptomator encryption
package cryptomator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
)

// Errors
var (
	errorMetaTooBig = errors.New("metadata file is too big")
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

	f := &Fs{
		wrapped: wrappedFs,
		name:    name,
		root:    root,
		opt:     *opt,
	}
	cache.PinUntilFinalized(f.wrapped, f)

	f.features = (&fs.Features{}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	password, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}
	err = loadVault(ctx, f, password)
	if err != nil {
		return nil, fmt.Errorf("failed to load vault config: %w", err)
	}
	f.Cryptor, err = NewCryptor(f.masterKey, f.vaultConfig.CipherCombo)
	if err != nil {
		return nil, err
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
			fs.Logf(f, "error!!! %q", err)
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}

			return nil, err
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

	wrapper fs.Fs
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

func (f *Fs) String() string {
	return fmt.Sprintf("Cryptomator vault of %s", f.wrapped.String())
}

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

// -------- fs.Fs

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to find ID for directory %q: %w", dir, err)
	}
	dirPath, err := f.encryptedPathForDirID(dirID)
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
		path.Ext(filename)
		remote := path.Join(dir, filename)

		switch entry.(type) {
		case fs.Directory:
			entries = append(entries, &Directory{
				Directory: entry.(fs.Directory),
				fs:        f,
				remote:    remote,
			})
		case fs.Object:
			entries = append(entries, &Object{
				Object: entry.(fs.Object),
				fs:     f,
				remote: remote,
			})
		default:
			return nil, fmt.Errorf("unknown entry type %T", entry)
		}
	}
	return
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		return nil, fmt.Errorf("failed to find ID for directory of file %q: %w", remote, err)
	}
	dirPath, err := f.encryptedPathForDirID(dirID)
	if err != nil {
		return nil, err
	}
	encryptedFilename, err := f.EncryptFilename(leaf, dirID)
	if err != nil {
		return nil, err
	}
	encryptedPath := path.Join(dirPath, encryptedFilename+".c9r")
	wrappedObj, err := f.wrapped.NewObject(ctx, encryptedPath)
	if err != nil {
		return nil, err
	}
	return &Object{
		Object: wrappedObj,
		fs:     f,
		remote: remote,
	}, nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return fmt.Errorf("TODO implement Fs.Rmdir")
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return fmt.Errorf("TODO implement Fs.Mkdir")
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fmt.Errorf("TODO implement Fs.Put")
}

// -------- fs.DirCacher

// FindLeaf finds a child of name leaf in the directory with id pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	dirPath, err := f.encryptedPathForDirID(pathID)
	if err != nil {
		return
	}
	encryptedFilename, err := f.EncryptFilename(leaf, pathID)
	if err != nil {
		return
	}
	subdirIDFile := path.Join(dirPath, encryptedFilename+".c9r", "dir.c9r")
	subdirID, err := f.readSmallFile(ctx, subdirIDFile, 100)
	if errors.Is(err, fs.ErrorDirNotFound) {
		err = nil
		return
	}
	// ErrorObjectNotFound should stay an error, that would mean that the directory exists but the dir.c9r file inside is somehow missing.
	// TODO: add an explicit message for that case
	if err != nil {
		err = fmt.Errorf("failed to read ID of subdir from %q: %w", subdirIDFile, err)
		return
	}
	pathIDOut = string(subdirID)
	found = true
	return
}

func (f *Fs) CreateDir(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("TODO implement DirCacher.CreateDir")
}

// -------- fs.Directory

type Directory struct {
	fs.Directory
	fs     *Fs
	remote string
}

func (d *Directory) Fs() fs.Info    { return d.fs }
func (d *Directory) Remote() string { return d.remote }

// -------- fs.Object

type Object struct {
	fs.Object
	fs     *Fs
	remote string
}

func (o *Object) Fs() fs.Info    { return o.fs }
func (o *Object) Remote() string { return o.remote }

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
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
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
	if err != nil {
		return nil, err
	}

	header, err := o.fs.UnmarshalHeader(reader)
	if err != nil {
		reader.Close()
		return nil, err
	}

	var decryptReader io.Reader
	decryptReader, err = o.fs.NewReader(reader, header)

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

// -------- private

func (f *Fs) encryptedPathForDirID(dirID string) (string, error) {
	encryptedDirID, err := f.EncryptDirID(dirID)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt directory ID: %w", err)
	}
	dirPath := path.Join("d", encryptedDirID[:2], encryptedDirID[2:])
	// TODO: verify that dirid.c9r inside the directory contains dirID
	return dirPath, nil
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
	defer reader.Close()
	data, err := io.ReadAll(reader)
	return data, err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs = (*Fs)(nil)
)
