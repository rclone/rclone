// FUSE main Fs

// +build linux darwin freebsd

package mount

import (
	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
)

// Default permissions
const (
	dirPerms  = 0755
	filePerms = 0644
)

// FS represents the top level filing system
type FS struct {
	f fs.Fs
}

// Check interface satistfied
var _ fusefs.FS = (*FS)(nil)

// Root returns the root node
func (f *FS) Root() (fusefs.Node, error) {
	fs.Debug(f.f, "Root()")
	return newDir(f.f, ""), nil
}

// mount the file system
//
// The mount point will be ready when this returns.
//
// returns an error, and an error channel for the serve process to
// report an error when fusermount is called.
func mount(f fs.Fs, mountpoint string) (<-chan error, error) {
	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return nil, err
	}

	filesys := &FS{
		f: f,
	}

	// Serve the mount point in the background returning error to errChan
	errChan := make(chan error, 1)
	go func() {
		err := fusefs.Serve(c, filesys)
		closeErr := c.Close()
		if err == nil {
			err = closeErr
		}
		errChan <- err
	}()

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return nil, err
	}

	return errChan, nil
}
