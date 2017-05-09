// FUSE main Fs

// +build linux darwin freebsd

package mount

import (
	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// FS represents the top level filing system
type FS struct {
	*mountlib.FS
	f fs.Fs
}

// Check interface satistfied
var _ fusefs.FS = (*FS)(nil)

// NewFS makes a new FS
func NewFS(f fs.Fs) *FS {
	fsys := &FS{
		FS: mountlib.NewFS(f),
		f:  f,
	}
	if noSeek {
		fsys.FS.NoSeek()
	}
	if noChecksum {
		fsys.FS.NoChecksum()
	}
	return fsys
}

// Root returns the root node
func (f *FS) Root() (node fusefs.Node, err error) {
	defer fs.Trace("", "")("node=%+v, err=%v", &node, &err)
	root, err := f.FS.Root()
	if err != nil {
		return nil, translateError(err)
	}
	return &Dir{root}, nil
}

// Check interface satsified
var _ fusefs.FSStatfser = (*FS)(nil)

// Statfs is called to obtain file system metadata.
// It should write that data to resp.
func (f *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) (err error) {
	defer fs.Trace("", "")("stat=%+v, err=%v", resp, &err)
	const blockSize = 4096
	const fsBlocks = (1 << 50) / blockSize
	resp.Blocks = fsBlocks  // Total data blocks in file system.
	resp.Bfree = fsBlocks   // Free blocks in file system.
	resp.Bavail = fsBlocks  // Free blocks in file system if you're not root.
	resp.Files = 1E9        // Total files in file system.
	resp.Ffree = 1E9        // Free files in file system.
	resp.Bsize = blockSize  // Block size
	resp.Namelen = 255      // Maximum file name length?
	resp.Frsize = blockSize // Fragment size, smallest addressable data size in the file system.
	return nil
}

// Translate errors from mountlib
func translateError(err error) error {
	if err == nil {
		return nil
	}
	cause := errors.Cause(err)
	if mErr, ok := cause.(mountlib.Error); ok {
		switch mErr {
		case mountlib.ENOENT:
			return fuse.ENOENT
		case mountlib.ENOTEMPTY:
			return fuse.EEXIST // return fuse.ENOTEMPTY - doesn't exist though so use EEXIST
		case mountlib.EEXIST:
			return fuse.EEXIST
		}
	}
	return err
}
