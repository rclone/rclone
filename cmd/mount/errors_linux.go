package mount

import (
	"syscall"

	"bazil.org/fuse"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

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
	case vfs.ENOATTR:
		return fuse.ErrNoXattr
	case vfs.ERANGE:
		return fuse.Errno(syscall.ERANGE)
	}
	return err
}
