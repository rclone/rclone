//go:build !js
// +build !js

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
	"github.com/rclone/rclone/fs/object"
	"golang.org/x/text/unicode/norm"

	"github.com/fhilgers/gocryptomator/pkg/vault"
)

// Globals
// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "cryptomator",
		Description: "Treat a remote as Cryptomator Vault",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remote is read and written`,
		},
		Options: []fs.Option{
			{
				Name:     "remote",
				Help:     "Remote which contains the Cryptomator Vault",
				Required: true,
			},
			{
				Name:       "password",
				Help:       "Password for the Cryptomator Vault",
				IsPassword: true,
				Required:   true,
			},
		},
	})
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, rpath string, m configmap.Mapper) (fs.Fs, error) {
	opts, err := newOpts(m)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(opts.Remote, name+":") {
		return nil, errors.New("can't point cryptomator remote at itself")
	}

	rootFs, err := cache.Get(ctx, opts.Remote)
	if err != nil {
		return nil, err
	}

	cryptomatorAdapterFs := NewCryptomatorAdapterFs(ctx, rootFs)

	password, err := obscure.Reveal(opts.Password)
	if err != nil {
		return nil, err
	}

	v, err := vault.Open(cryptomatorAdapterFs, password)
	if err != nil {
		fs.Logf(rootFs, "vault not found, creating new")
		v, err = vault.Create(cryptomatorAdapterFs, password)
		if err != nil {
			return nil, fmt.Errorf("failed to create vault: %w", err)
		}
	}

	var fsErr error
	if filePath, _, err := v.GetFilePath(rpath); err == nil {
		if exists, _ := fs.FileExists(ctx, rootFs, filePath); exists {
			rpath = path.Dir(rpath)
			fsErr = fs.ErrorIsFile
		}
	}
	f := &Fs{
		name:  name,
		root:  rpath,
		vault: v,
		Fs:    rootFs,
	}

	cache.PinUntilFinalized(rootFs, f)

	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          true,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
		SetTier:                 true,
		GetTier:                 true,
		ReadMetadata:            true,
		WriteMetadata:           true,
		UserMetadata:            true,
		WriteMimeType:           false,
		ReadMimeType:            false,
	}).Fill(ctx, f).Mask(ctx, rootFs).WrapsFs(f, rootFs)

	f.features.CanHaveEmptyDirectories = true

	return f, fsErr
}

// Options defines the configuration for this backend
type Options struct {
	Remote   string `config:"remote"`
	Password string `config:"password"`
}

// Set the options from the configmap
func newOpts(m configmap.Mapper) (*Options, error) {
	opts := new(Options)
	err := configstruct.Set(m, opts)
	return opts, err
}

// Fs ---------------------------------------------

// Fs wraps another fs and encrypts the directory
// structure, filenames, and file contents as outlined
// in https://docs.cryptomator.org/en/latest/security/architecture/
type Fs struct {
	fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	vault    *vault.Vault
	features *fs.Features
}

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

// Returns the supported hash types of the filesystem
//
// We cannot support hashes
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

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
	path, dirID, err := f.vault.GetDirPath(f.fullPath(dir))
	if err != nil {
		return nil, fs.ErrorDirNotFound
	}

	entries, err := f.Fs.List(ctx, path)
	if err != nil {
		return nil, err
	}

	return f.wrapEntries(entries, dir, dirID)
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fullPath := f.fullPath(remote)
	objPath, dirID, err := f.vault.GetFilePath(fullPath)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	obj, err := f.Fs.NewObject(ctx, objPath)
	if err != nil {
		return nil, err
	}

	return f.newObject(obj, path.Dir(remote), dirID)
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
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (obj fs.Object, err error) {
	return f.put(ctx, in, src, options, f.Fs.Put)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fullPath := f.fullPath(dir)
	fullPath = path.Clean(fullPath)
	if fullPath == "." {
		fullPath = ""
	}

	segments := strings.Split(fullPath, "/")
	for i := range segments {
		if err := f.vault.Mkdir(strings.Join(segments[:i+1], "/")); err != nil {
			return err
		}
	}

	return nil
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.vault.Rmdir(f.fullPath(dir))
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}

	if err := f.Mkdir(ctx, path.Dir(remote)); err != nil {
		return nil, err
	}

	encRemote, dirID, err := f.vault.GetFilePath(f.fullPath(remote))
	if err != nil {
		return nil, err
	}

	oResult, err := do(ctx, o.Object, encRemote)
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult, path.Dir(remote), dirID)
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
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	if err := f.Mkdir(ctx, path.Dir(remote)); err != nil {
		return nil, err
	}

	encRemote, dirID, err := f.vault.GetFilePath(f.fullPath(remote))
	if err != nil {
		return nil, err
	}

	oResult, err := do(ctx, o.Object, encRemote)
	if err != nil {
		return nil, err
	}

	return f.newObject(oResult, path.Dir(remote), dirID)
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
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	if _, _, err := f.vault.GetDirPath(f.fullPath(dstRemote)); err == nil {
		return fs.ErrorDirExists
	}

	// collect all subdirectories
	srcsToMove := []string{srcRemote}
	i := 0
	for {
		if i == len(srcsToMove) {
			break
		}

		entries, err := srcFs.List(ctx, srcsToMove[i])
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if d, ok := entry.(fs.Directory); ok {
				srcsToMove = append(srcsToMove, d.Remote())
			}
		}

		i += 1
	}

	// find all their encrypted paths
	encSrcNames := []string{}
	for _, srcName := range srcsToMove {
		encName, _, err := srcFs.vault.GetDirPath(path.Join(srcFs.root, srcName))
		if err != nil {
			return err
		}

		encSrcNames = append(encSrcNames, encName)
	}

	// move all the encrypted paths
	for _, encName := range encSrcNames {
		if err := do(ctx, srcFs.Fs, encName, encName); err == fs.ErrorDirExists {
			// Ignore
		} else if err != nil {
			return err
		}
	}

	srcDirID, err := srcFs.vault.GetDirID(path.Join(srcFs.root, srcRemote))
	if err != nil {
		return err
	}

	srcDirIDPath, _, err := srcFs.vault.GetFilePath(path.Join(srcFs.root, srcRemote))
	if err != nil {
		return nil
	}

	// remove dirID from src
	obj, err := srcFs.Fs.NewObject(ctx, path.Join(srcDirIDPath, "dir.c9r"))
	if err != nil {
		return err
	}

	if err = obj.Remove(ctx); err != nil {
		return err
	}

	if err = srcFs.Fs.Rmdir(ctx, srcDirIDPath); err != nil {
		return err
	}

	// create dir in dest
	if err = f.Mkdir(ctx, path.Dir(dstRemote)); err != nil {
		return err
	}

	encName, _, err := f.vault.GetFilePath(f.fullPath(dstRemote))
	if err != nil {
		return err
	}

	if err = f.Fs.Mkdir(ctx, encName); err != nil {
		return err
	}

	info := object.NewStaticObjectInfo(path.Join(encName, "dir.c9r"), time.Now(), -1, true, nil, f.Fs)

	// write dirID to dest
	if _, err = f.Fs.Put(ctx, strings.NewReader(srcDirID), info); err != nil {
		return err
	}

	f.vault.FullyInvalidate()
	srcFs.vault.FullyInvalidate()

	return nil
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	do := f.Fs.Features().DirCacheFlush
	if do != nil {
		do()
	}
	f.vault.FullyInvalidate()
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	do := f.Fs.Features().PublicLink
	if do == nil {
		return "", errors.New("PublicLink not supported")
	}
	path, _, err := f.vault.GetDirPath(f.fullPath(remote))
	if err != nil {
		path, _, err = f.vault.GetFilePath(f.fullPath(remote))
		if err != nil {
			return "", err
		}
	}
	return do(ctx, path, expire, unlink)
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
//
// May create duplicates or return errors if src already
// exists.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("PutUnchecked not supported by underlying fs")
	}
	return f.put(ctx, in, src, options, do)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options, f.Fs.Features().PutStream)
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	do := f.Fs.Features().CleanUp
	if do == nil {
		return errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	do := f.Fs.Features().UserInfo
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	do := f.Fs.Features().Disconnect
	if do == nil {
		return fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	do := f.Fs.Features().Shutdown
	if do == nil {
		return nil
	}
	return do(ctx)
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (obj fs.Object, err error) {
	if err = f.Mkdir(ctx, path.Dir(src.Remote())); err != nil {
		return
	}

	dirID, err := f.vault.GetDirID(path.Dir(f.fullPath(src.Remote())))
	if err != nil {
		return nil, err
	}

	encReader, err := f.vault.NewEncryptReader(in)
	if err != nil {
		return nil, err
	}

	info, err := f.newEncryptedObjectInfo(src, src.Remote())
	if err != nil {
		return nil, err
	}

	obj, err = put(ctx, encReader, info, options...)
	if err != nil {
		return obj, err
	}

	return f.newObject(obj, path.Dir(src.Remote()), dirID)
}

// Wrap ObjectInfo to pass it on to the underlying fs, eg. determine encrypted size
// if known and determine encrypted remote path
func (f *Fs) newEncryptedObjectInfo(info fs.ObjectInfo, remote string) (*EncryptedObjectInfo, error) {
	encryptedRemote, _, err := f.vault.GetFilePath(f.fullPath(remote))
	if err != nil {
		return nil, err
	}

	size := info.Size()
	if size != -1 {
		size = vault.CalculateEncryptedFileSize(info.Size())
	}

	return &EncryptedObjectInfo{
		ObjectInfo: info,
		remote:     encryptedRemote,
		size:       size,
	}, nil
}

// Wrap the Object of the underlying fs, eg. determine decrypted remote path and filesize
// and attach this fs to Decrypt / Encrypt the Data on Open / Update
func (f *Fs) newObject(obj fs.Object, dir, dirID string) (*Object, error) {
	encryptedName := path.Base(obj.Remote())
	decryptedName, err := f.vault.DecryptFileName(encryptedName, dirID)
	if err != nil {
		return nil, err
	}

	remote := path.Join(dir, decryptedName)
	size := vault.CalculateRawFileSize(obj.Size())

	return &Object{
		Object: obj,
		remote: remote,
		size:   size,
		f:      f,
	}, nil
}

// Wrap the Directory of the underlying fs, eg. determine the decrypted remote path
func (f *Fs) newDirectory(d fs.Directory, dir, dirID string) (*Directory, error) {
	encName := path.Base(d.Remote())
	decName, err := f.vault.DecryptFileName(encName, dirID)
	if err != nil {
		return nil, err
	}

	return &Directory{
		remote:    path.Join(dir, decName),
		Directory: d,
	}, nil
}

// Wrap the DirEntries of the underlying fs in either cryptomator.Object or cryptomator.Directory
func (f *Fs) wrapEntries(entries fs.DirEntries, dir, dirID string) (wrappedEntries fs.DirEntries, err error) {
	var wrappedEntry fs.DirEntry
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			if path.Base(x.Remote()) == "dirid.c9r" {
				continue
			}
			wrappedEntry, err = f.newObject(x, dir, dirID)
		case fs.Directory:
			wrappedEntry, err = f.newDirectory(x, dir, dirID)
		}

		if err != nil {
			return
		}

		wrappedEntries = append(wrappedEntries, wrappedEntry)
	}

	return
}

func (f *Fs) fullPath(name string) string {
	return norm.NFC.String(path.Join(f.root, name))
}

// EncryptedObjectInfo -----------------------------------

// EncryptedObjectInfo provides read only information about an object
//
// This is wraps the plain Object info and adds the encrypted remote
// path and encrypted size to pass it on to the underlying fs.
type EncryptedObjectInfo struct {
	fs.ObjectInfo

	f *Fs

	remote string
	size   int64
}

// String returns a description of the Object
func (i *EncryptedObjectInfo) String() string {
	if i == nil {
		return "<nil>"
	}
	return i.remote
}

// Remote returns the encrypted remote path
func (i *EncryptedObjectInfo) Remote() string {
	return i.remote
}

// Size returns the encrypted size of the file
func (i *EncryptedObjectInfo) Size() int64 {
	return i.size
}

// Fs returns read only access to the Fs that this object is part of
func (i *EncryptedObjectInfo) Fs() fs.Info {
	return i.f
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
//
// TODO: We cannot compute the same hash without storing information
// about the file header which is not implemented yet.
func (i *EncryptedObjectInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// MimeType returns the content type of the Object if
// known, or "" if not
//
// This is deliberately unsupported so we don't leak mime type info by
// default.
func (i *EncryptedObjectInfo) MimeType(ctx context.Context) string {
	return ""
}

// ID returns the ID of the Object if known, or "" if not
func (i *EncryptedObjectInfo) ID() string {
	do, ok := i.ObjectInfo.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *EncryptedObjectInfo) UnWrap() fs.Object {
	return fs.UnWrapObjectInfo(o.ObjectInfo)
}

// GetTier returns storage tier or class of the Object
func (i *EncryptedObjectInfo) GetTier() string {
	do, ok := i.ObjectInfo.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (i *EncryptedObjectInfo) Metadata(ctx context.Context) (fs.Metadata, error) {
	do, ok := i.ObjectInfo.(fs.Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// Directory ----------------------------------------

// Directory wraps a directory of the underlying fs but
// stores the decrypted remote.
type Directory struct {
	fs.Directory
	remote string
}

// String returns a description of the Object
func (d *Directory) String() string {
	if d == nil {
		return "<nil>"
	}
	return d.remote
}

// Remote returns the decrypted remote path
func (d *Directory) Remote() string {
	return d.remote
}

// Size returns the size of the file
//
// As we do not know the real size we return 0
func (d *Directory) Size() int64 {
	return 0
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
//
// As the directory structure is flattened and cluttered
// with dirid.c9r and dir.c9r files we do not know the real
// amount and return -1
func (d *Directory) Items() int64 {
	return -1
}

// Object -------------------------------------------

// Object wraps an object of the underlying fs but stores
// the decrypted remote and the decrypted file size
type Object struct {
	fs.Object

	f *Fs

	remote string
	size   int64
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the decrypted remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the decrypted size of the file
func (o *Object) Size() int64 {
	return o.size
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.f
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
//
// We cannot compute a hash of the object without downloading
// and decrypting it so this is not Supported
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// MimeType returns the content type of the Object if
// known, or "" if not
//
// This is deliberately unsupported so we don't leak mime type info by
// default.
func (o *Object) MimeType(ctx context.Context) string {
	return ""
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	do, ok := o.Object.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ParentID() string {
	do, ok := o.Object.(fs.ParentIDer)
	if !ok {
		return ""
	}
	return do.ParentID()
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *Object) SetTier(tier string) error {
	do, ok := o.Object.(fs.SetTierer)
	if !ok {
		return errors.New("crypt: underlying remote does not support SetTier")
	}
	return do.SetTier(tier)
}

// GetTier returns storage tier or class of the Object
func (o *Object) GetTier() string {
	do, ok := o.Object.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	do, ok := o.Object.(fs.Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
//
// This calls Open on the object of the underlying remote with fs.SeekOption
// and fs.RangeOption removes. This is strictly necessary as the file header
// contains all the information to decrypt the file.
//
// We wrap the reader of the underlying object to decrypt the data.
// - For fs.SeekOption we just discard all the bytes until we reach the Offset
// - For fs.RangeOption we do the same and then wrap the reader in io.LimitReader
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var offset, limit int64 = 0, -1
	var openOptions []fs.OpenOption
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			openOptions = append(openOptions, option)
		}
	}

	readCloser, err := o.Object.Open(ctx, openOptions...)
	if err != nil {
		return nil, err
	}

	decryptReader, err := o.f.vault.NewDecryptReader(readCloser)
	if err != nil {
		return nil, err
	}

	if offset > 0 {
		if _, err = io.CopyN(io.Discard, decryptReader, offset); err != nil {
			return nil, err
		}
	}

	if limit != -1 {
		return newReadCloser(io.LimitReader(decryptReader, limit), readCloser), nil
	}

	return newReadCloser(decryptReader, readCloser), nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	o.size = src.Size()

	update := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.Object, o.Object.Update(ctx, in, src, options...)
	}

	_, err := o.f.put(ctx, in, src, options, update)

	return err
}

// Create a readerCLoserWrapper from both parts
func newReadCloser(r io.Reader, c io.Closer) io.ReadCloser {
	return readerCloserWrapper{
		Reader: r,
		Closer: c,
	}
}

// Wraps a reader and closer, necessary when wrapping
// a ReadCloser with a method which takes and returns
// just a Reader.
type readerCloserWrapper struct {
	io.Reader
	io.Closer
}

var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Wrapper         = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.FullObjectInfo  = (*EncryptedObjectInfo)(nil)
	_ fs.FullObject      = (*Object)(nil)
	_ fs.Directory       = (*Directory)(nil)
)
