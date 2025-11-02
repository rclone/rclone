//go:build !plan9

// Package archive implements a backend to access archive files in a remote
package archive

// FIXME factor common code between backends out - eg VFS initialization

// FIXME can we generalize the VFS handle caching and use it in zip backend

// Factor more stuff out if possible

// Odd stats which are probably coming from the VFS
// *  tensorflow.sqfs:  0% /3.074Gi, 204.426Ki/s, 4h22m46s

// FIXME this will perform poorly for unpacking as the VFS Reader is bad
// at multiple streams - need cache mode setting?

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	// Import all the required archivers here
	_ "github.com/rclone/rclone/backend/archive/squashfs"
	_ "github.com/rclone/rclone/backend/archive/zip"

	"github.com/rclone/rclone/backend/archive/archiver"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "archive",
		Description: "Read archives",
		NewFs:       NewFs,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remote is read and written.`,
		},
		Options: []fs.Option{{
			Name: "remote",
			Help: `Remote to wrap to read archives from.

Normally should contain a ':' and a path, e.g. "myremote:path/to/dir",
"myremote:bucket" or "myremote:".

If this is left empty, then the archive backend will use the root as
the remote.

This means that you can use :archive:remote:path and it will be
equivalent to setting remote="remote:path".
`,
			Required: false,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Remote string `config:"remote"`
}

// Fs represents a archive of upstreams
type Fs struct {
	name     string       // name of this remote
	features *fs.Features // optional features
	opt      Options      // options for this Fs
	root     string       // the path we are working on
	f        fs.Fs        // remote we are wrapping
	wrapper  fs.Fs        // fs that wraps us

	mu       sync.Mutex          // protects the below
	archives map[string]*archive // the archives we have, by path
}

// A single open archive
type archive struct {
	archiver archiver.Archiver // archiver responsible
	remote   string            // path to the archive
	prefix   string            // prefix to add on to listings
	root     string            // root of the archive to remove from listings
	mu       sync.Mutex        // protects the following variables
	f        fs.Fs             // the archive Fs, may be nil
}

// If remote is an archive then return it otherwise return nil
func findArchive(remote string) *archive {
	// FIXME use something faster than linear search?
	for _, archiver := range archiver.Archivers {
		if strings.HasSuffix(remote, archiver.Extension) {
			return &archive{
				archiver: archiver,
				remote:   remote,
				prefix:   remote,
				root:     "",
			}
		}
	}
	return nil
}

// Find an archive buried in remote
func subArchive(remote string) *archive {
	archive := findArchive(remote)
	if archive != nil {
		return archive
	}
	parent := path.Dir(remote)
	if parent == "/" || parent == "." {
		return nil
	}
	return subArchive(parent)
}

// If remote is an archive then return it otherwise return nil
func (f *Fs) findArchive(remote string) (archive *archive) {
	archive = findArchive(remote)
	if archive != nil {
		f.mu.Lock()
		f.archives[remote] = archive
		f.mu.Unlock()
	}
	return archive
}

// Instantiate archive if it hasn't been instantiated yet
//
// This is done lazily so that we can list a directory full of
// archives without opening them all.
func (a *archive) init(ctx context.Context, f fs.Fs) (fs.Fs, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.f != nil {
		return a.f, nil
	}
	newFs, err := a.archiver.New(ctx, f, a.remote, a.prefix, a.root)
	if err != nil && err != fs.ErrorIsFile {
		return nil, fmt.Errorf("failed to create archive %q: %w", a.remote, err)
	}
	a.f = newFs
	return a.f, nil
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (outFs fs.Fs, err error) {
	// defer log.Trace(nil, "name=%q, root=%q, m=%v", name, root, m)("f=%+v, err=%v", &outFs, &err)
	// Parse config into Options struct
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	remote := opt.Remote
	origRoot := root

	// If remote is empty, use the root instead
	if remote == "" {
		remote = root
		root = ""
	}
	isDirectory := strings.HasSuffix(remote, "/")
	remote = strings.TrimRight(remote, "/")
	if remote == "" {
		remote = "/"
	}
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point archive remote at itself - check the value of the upstreams setting")
	}

	_ = isDirectory

	foundArchive := subArchive(remote)
	if foundArchive != nil {
		fs.Debugf(nil, "Found archiver for %q remote %q", foundArchive.archiver.Extension, foundArchive.remote)
		// Archive path
		foundArchive.root = strings.Trim(remote[len(foundArchive.remote):], "/")
		// Path to the archive
		archiveRemote := remote[:len(foundArchive.remote)]
		// Remote is archive leaf name
		foundArchive.remote = path.Base(archiveRemote)
		foundArchive.prefix = ""
		// Point remote to archive file
		remote = archiveRemote
	}

	// Make sure to remove trailing . referring to the current dir
	if path.Base(root) == "." {
		root = strings.TrimSuffix(root, ".")
	}
	remotePath := fspath.JoinRootPath(remote, root)
	wrappedFs, err := cache.Get(ctx, remotePath)
	if err != fs.ErrorIsFile && err != nil {
		return nil, fmt.Errorf("failed to make remote %q to wrap: %w", remote, err)
	}

	f := &Fs{
		name: name,
		//root:     path.Join(remotePath, root),
		root:     origRoot,
		opt:      *opt,
		f:        wrappedFs,
		archives: make(map[string]*archive),
	}
	cache.PinUntilFinalized(f.f, f)
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             true,
		SetTier:                 true,
		GetTier:                 true,
		ReadMetadata:            true,
		WriteMetadata:           true,
		UserMetadata:            true,
		PartialUploads:          true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	if foundArchive != nil {
		fs.Debugf(f, "Root is an archive")
		if err != fs.ErrorIsFile {
			return nil, fmt.Errorf("expecting to find a file at %q", remote)
		}
		return foundArchive.init(ctx, f.f)
	}
	// Correct root if definitely pointing to a file
	if err == fs.ErrorIsFile {
		f.root = path.Dir(f.root)
		if f.root == "." || f.root == "/" {
			f.root = ""
		}
	}
	return f, err
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("archive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.f.Rmdir(ctx, dir)
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return f.f.Hashes()
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.f.Mkdir(ctx, dir)
}

// Purge all files in the directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	do := f.f.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do(ctx, dir)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.f.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	// FIXME
	// o, ok := src.(*Object)
	// if !ok {
	// 	return nil, fs.ErrorCantCopy
	// }
	return do(ctx, src, remote)
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.f.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	// FIXME
	// o, ok := src.(*Object)
	// if !ok {
	// 	return nil, fs.ErrorCantMove
	// }
	return do(ctx, src, remote)
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	do := f.f.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(ctx, srcFs.f, srcRemote, dstRemote)
}

// ChangeNotify calls the passed function with a path
// that has had changes. If the implementation
// uses polling, it should adhere to the given interval.
// At least one value will be written to the channel,
// specifying the initial value and updated values might
// follow. A 0 Duration should pause the polling.
// The ChangeNotify implementation must empty the channel
// regularly. When the channel gets closed, the implementation
// should stop polling and release resources.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), ch <-chan time.Duration) {
	do := f.f.Features().ChangeNotify
	if do == nil {
		return
	}
	wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
		// fs.Debugf(f, "ChangeNotify: path %q entryType %d", path, entryType)
		notifyFunc(path, entryType)
	}
	do(ctx, wrappedNotifyFunc, ch)
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	do := f.f.Features().DirCacheFlush
	if do != nil {
		do()
	}
}

func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, stream bool, options ...fs.OpenOption) (fs.Object, error) {
	var o fs.Object
	var err error
	if stream {
		o, err = f.f.Features().PutStream(ctx, in, src, options...)
	} else {
		o, err = f.f.Put(ctx, in, src, options...)
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return o, o.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.put(ctx, in, src, false, options...)
	default:
		return nil, err
	}
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return o, o.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.put(ctx, in, src, true, options...)
	default:
		return nil, err
	}
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.f.Features().About
	if do == nil {
		return nil, errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// Find the Fs for the directory
func (f *Fs) findFs(ctx context.Context, dir string) (subFs fs.Fs, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	subFs = f.f

	// FIXME should do this with a better datastructure like a prefix tree
	// FIXME want to find the longest first otherwise nesting won't work
	dirSlash := dir + "/"
	for archiverRemote, archive := range f.archives {
		subRemote := archiverRemote + "/"
		if strings.HasPrefix(dirSlash, subRemote) {
			subFs, err = archive.init(ctx, f.f)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return subFs, nil
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// defer log.Trace(f, "dir=%q", dir)("entries = %v, err=%v", &entries, &err)

	subFs, err := f.findFs(ctx, dir)
	if err != nil {
		return nil, err
	}

	entries, err = subFs.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	for i, entry := range entries {
		// Can only unarchive files
		if o, ok := entry.(fs.Object); ok {
			remote := o.Remote()
			archive := f.findArchive(remote)
			if archive != nil {
				// Overwrite entry with directory
				entries[i] = fs.NewDir(remote, o.ModTime(ctx))
			}
		}
	}
	return entries, nil
}

// NewObject creates a new remote archive file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {

	dir := path.Dir(remote)
	if dir == "/" || dir == "." {
		dir = ""
	}

	subFs, err := f.findFs(ctx, dir)
	if err != nil {
		return nil, err
	}

	o, err := subFs.NewObject(ctx, remote)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Precision is the greatest precision of all the archivers
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	if do := f.f.Features().Shutdown; do != nil {
		return do(ctx)
	}
	return nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	do := f.f.Features().PublicLink
	if do == nil {
		return "", errors.New("PublicLink not supported")
	}
	return do(ctx, remote, expire, unlink)
}

// PutUnchecked in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
//
// May create duplicates or return errors if src already
// exists.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.f.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	o, err := do(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	if len(dirs) == 0 {
		return nil
	}
	do := f.f.Features().MergeDirs
	if do == nil {
		return errors.New("MergeDirs not supported")
	}
	return do(ctx, dirs)
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	do := f.f.Features().CleanUp
	if do == nil {
		return errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// OpenWriterAt opens with a handle for random access writes
//
// Pass in the remote desired and the size if known.
//
// It truncates any existing object
func (f *Fs) OpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	do := f.f.Features().OpenWriterAt
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	return do(ctx, remote, size)
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.f
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	do := f.f.Features().OpenChunkWriter
	if do == nil {
		return info, nil, fs.ErrorNotImplemented
	}
	return do(ctx, remote, src, options...)
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	do := f.f.Features().UserInfo
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	do := f.f.Features().Disconnect
	if do == nil {
		return fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.OpenWriterAter  = (*Fs)(nil)
	_ fs.OpenChunkWriter = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Disconnecter    = (*Fs)(nil)
	// FIXME _ fs.FullObject      = (*Object)(nil)
)
