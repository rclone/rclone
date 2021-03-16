// FUSE main Fs

// +build linux freebsd

package mount

import (
	"context"
	"syscall"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
)

// FS represents the top level filing system
type FS struct {
	*vfs.VFS
	f      fs.Fs
	opt    *mountlib.Options
	server *fusefs.Server
}

// Check interface satisfied
var _ fusefs.FS = (*FS)(nil)

// NewFS makes a new FS
func NewFS(VFS *vfs.VFS, opt *mountlib.Options) *FS {
	fsys := &FS{
		VFS: VFS,
		f:   VFS.Fs(),
		opt: opt,
	}
	return fsys
}

// Root returns the root node
func (f *FS) Root() (node fusefs.Node, err error) {
	defer log.Trace("", "")("node=%+v, err=%v", &node, &err)
	root, err := f.VFS.Root()
	if err != nil {
		return nil, translateError(err)
	}
	return &Dir{root, f}, nil
}

// Check interface satisfied
var _ fusefs.FSStatfser = (*FS)(nil)

// Statfs is called to obtain file system metadata.
// It should write that data to resp.
func (f *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) (err error) {
	defer log.Trace("", "")("stat=%+v, err=%v", resp, &err)
	const blockSize = 4096
	total, _, free := f.VFS.Statfs()
	resp.Blocks = uint64(total) / blockSize // Total data blocks in file system.
	resp.Bfree = uint64(free) / blockSize   // Free blocks in file system.
	resp.Bavail = resp.Bfree                // Free blocks in file system if you're not root.
	resp.Files = 1e9                        // Total files in file system.
	resp.Ffree = 1e9                        // Free files in file system.
	resp.Bsize = blockSize                  // Block size
	resp.Namelen = 255                      // Maximum file name length?
	resp.Frsize = blockSize                 // Fragment size, smallest addressable data size in the file system.
	mountlib.ClipBlocks(&resp.Blocks)
	mountlib.ClipBlocks(&resp.Bfree)
	mountlib.ClipBlocks(&resp.Bavail)
	return nil
}

// Translate errors from mountlib
func translateError(err error) error {
	if err == nil {
		return nil
	}
	switch errors.Cause(err) {
	case vfs.OK:
		return nil
	case vfs.ENOENT, fs.ErrorDirNotFound, fs.ErrorObjectNotFound:
		return fuse.ENOENT
	case vfs.EEXIST, fs.ErrorDirExists:
		return fuse.EEXIST
	case vfs.EPERM, fs.ErrorPermissionDenied:
		return fuse.EPERM
	case vfs.ECLOSED:
		return fuse.Errno(syscall.EBADF)
	case vfs.ENOTEMPTY:
		return fuse.Errno(syscall.ENOTEMPTY)
	case vfs.ESPIPE:
		return fuse.Errno(syscall.ESPIPE)
	case vfs.EBADF:
		return fuse.Errno(syscall.EBADF)
	case vfs.EROFS:
		return fuse.Errno(syscall.EROFS)
	case vfs.ENOSYS, fs.ErrorNotImplemented:
		return fuse.ENOSYS
	case vfs.EINVAL:
		return fuse.Errno(syscall.EINVAL)
	}
	return err
}
