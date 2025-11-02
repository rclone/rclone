// Package base is a base archive Fs
package base

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/vfs"
)

// Fs represents a wrapped fs.Fs
type Fs struct {
	f           fs.Fs
	wrapper     fs.Fs
	name        string
	features    *fs.Features // optional features
	vfs         *vfs.VFS
	node        vfs.Node // archive object
	remote      string   // remote of the archive object
	prefix      string   // position for objects
	prefixSlash string   // position for objects with a slash on
	root        string   // position to read from within the archive
}

var errNotImplemented = errors.New("internal error: method not implemented in archiver")

// New constructs an Fs from the (wrappedFs, remote) with the objects
// prefix with prefix and rooted at root
func New(ctx context.Context, wrappedFs fs.Fs, remote, prefix, root string) (*Fs, error) {
	// FIXME vfs cache?
	// FIXME could factor out ReadFileHandle and just use that rather than the full VFS
	fs.Debugf(nil, "New: remote=%q, prefix=%q, root=%q", remote, prefix, root)
	VFS := vfs.New(wrappedFs, nil)
	node, err := VFS.Stat(remote)
	if err != nil {
		return nil, fmt.Errorf("failed to find %q archive: %w", remote, err)
	}

	f := &Fs{
		f:           wrappedFs,
		name:        path.Join(fs.ConfigString(wrappedFs), remote),
		vfs:         VFS,
		node:        node,
		remote:      remote,
		root:        root,
		prefix:      prefix,
		prefixSlash: prefix + "/",
	}

	// FIXME
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	//
	// FIXME some of these need to be forced on - CanHaveEmptyDirectories
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            false, // MimeTypes not supported with gzip
		WriteMimeType:           false,
		BucketBased:             false,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	return f, nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// String returns a description of the FS
func (f *Fs) String() string {
	return f.name
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
	return nil, errNotImplemented
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	return nil, errNotImplemented
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return vfs.EROFS
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return vfs.EROFS
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (o fs.Object, err error) {
	return nil, vfs.EROFS
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
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

// Object describes an object to be read from the raw zip file
type Object struct {
	f      *Fs
	remote string
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.f
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return -1
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return time.Now()
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return vfs.EROFS
}

// Storable raturns a boolean indicating if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	return nil, errNotImplemented
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return vfs.EROFS
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return vfs.EROFS
}

// Check the interfaces are satisfied
var (
	_ fs.Fs        = (*Fs)(nil)
	_ fs.UnWrapper = (*Fs)(nil)
	_ fs.Wrapper   = (*Fs)(nil)
	_ fs.Object    = (*Object)(nil)
)
