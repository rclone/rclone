package youtubemusic

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/rest"
)

type Fs struct {
	name      string                 // name of this remote
	root      string                 // the path we are working on if any
	opt       Options                // parsed options
	features  *fs.Features           // optional features
	unAuth    *rest.Client           // unauthenticated http client
	srv       *rest.Client           // the connection to the server
	ts        *oauthutil.TokenSource // token source for oauth2
	pacer     *fs.Pacer              // To pace the API calls
	startTime time.Time              // time Fs was started - used for datestamps
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
	return fmt.Sprintf("YouTube Music path '%q'", f.root)
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// TODO:
	return nil, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// TODO:
	return nil, nil
}

// Put the object into the bucket
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// TODO:
	return nil, nil
}

// Mkdir creates the album if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// TODO:
	return nil
}

// Rmdir deletes the bucket if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// TODO:
	return nil
}

// NewFs constructs an Fs from the path.
//
// The returned Fs implements fs.Fs interface.
func NewFs(ctx context.Context, name, path string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	fsys := &Fs{}

	return fsys, nil
}

// Check the interfaces are satisfied.
var _ fs.Fs = &Fs{}
