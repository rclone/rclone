// Package mockfs provides mock Fs for testing.
package mockfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
)

// Register with Fs
func Register() {
	fs.Register(&fs.RegInfo{
		Name:        "mockfs",
		Description: "Mock FS",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "potato",
			Help:     "Does it have a potato?.",
			Required: true,
		}},
	})
}

// Fs is a minimal mock Fs
type Fs struct {
	name     string        // the name of the remote
	root     string        // The root directory (OS path)
	features *fs.Features  // optional features
	rootDir  fs.DirEntries // directory listing of root
	hashes   hash.Set      // which hashes we support
}

// ErrNotImplemented is returned by unimplemented methods
var ErrNotImplemented = errors.New("not implemented")

// NewFs returns a new mock Fs
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	f := &Fs{
		name: name,
		root: root,
	}
	f.features = (&fs.Features{}).Fill(ctx, f)
	return f, nil
}

// AddObject adds an Object for List to return
// Only works for the root for the moment
func (f *Fs) AddObject(o fs.Object) {
	f.rootDir = append(f.rootDir, o)
	// Make this object part of mockfs if possible
	do, ok := o.(interface{ SetFs(f fs.Fs) })
	if ok {
		do.SetFs(f)
	}
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
	return fmt.Sprintf("Mock file system at %s", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return f.hashes
}

// SetHashes sets the hashes that this supports
func (f *Fs) SetHashes(hashes hash.Set) {
	f.hashes = hashes
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	if dir == "" {
		return f.rootDir, nil
	}
	return entries, fs.ErrorDirNotFound
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	dirPath := path.Dir(remote)
	if dirPath == "" || dirPath == "." {
		for _, entry := range f.rootDir {
			if entry.Remote() == remote {
				return entry.(fs.Object), nil
			}
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, ErrNotImplemented
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return ErrNotImplemented
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return ErrNotImplemented
}

// Assert it is the correct type
var _ fs.Fs = (*Fs)(nil)
