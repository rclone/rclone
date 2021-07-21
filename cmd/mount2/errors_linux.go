package mount2

import (
	"syscall"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// Translate errors from mountlib into Syscall error numbers
func translateError(err error) syscall.Errno {
	if err == nil {
		return 0
	}
	switch errors.Cause(err) {
	case vfs.OK:
		return 0
	case vfs.ENOENT, fs.ErrorDirNotFound, fs.ErrorObjectNotFound:
		return syscall.ENOENT
	case vfs.EEXIST, fs.ErrorDirExists:
		return syscall.EEXIST
	case vfs.EPERM, fs.ErrorPermissionDenied:
		return syscall.EPERM
	case vfs.ECLOSED:
		return syscall.EBADF
	case vfs.ENOTEMPTY:
		return syscall.ENOTEMPTY
	case vfs.ESPIPE:
		return syscall.ESPIPE
	case vfs.EBADF:
		return syscall.EBADF
	case vfs.EROFS:
		return syscall.EROFS
	case vfs.ENOSYS, fs.ErrorNotImplemented:
		return syscall.ENOSYS
	case vfs.EINVAL:
		return syscall.EINVAL
	case vfs.ENOATTR:
		// On non-BSD platforms, there is no ENOATTR. xattr operations instead
		// return ENODATA.
		return syscall.ENODATA
	case vfs.ENODATA:
		return syscall.ENODATA
	case vfs.ERANGE:
		return syscall.ERANGE
	}
	fs.Errorf(nil, "IO error: %v", err)
	return syscall.EIO
}
