//go:build cmount && cgo
// +build cmount,cgo

package cmount

import (
	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
)

// Translate errors from mountlib
func translateError(err error) (errc int) {
	if err == nil {
		return 0
	}
	switch errors.Cause(err) {
	case vfs.OK:
		return 0
	case vfs.ENOENT, fs.ErrorDirNotFound, fs.ErrorObjectNotFound:
		return -fuse.ENOENT
	case vfs.EEXIST, fs.ErrorDirExists:
		return -fuse.EEXIST
	case vfs.EPERM, fs.ErrorPermissionDenied:
		return -fuse.EPERM
	case vfs.ECLOSED:
		return -fuse.EBADF
	case vfs.ENOTEMPTY:
		return -fuse.ENOTEMPTY
	case vfs.ESPIPE:
		return -fuse.ESPIPE
	case vfs.EBADF:
		return -fuse.EBADF
	case vfs.EROFS:
		return -fuse.EROFS
	case vfs.ENOSYS, fs.ErrorNotImplemented:
		return -fuse.ENOSYS
	case vfs.EINVAL:
		return -fuse.EINVAL
	case vfs.ENOATTR:
		return -fuse.ENOATTR
	case vfs.ERANGE:
		return -fuse.ERANGE
	}
	fs.Errorf(nil, "IO error: %v", err)
	return -fuse.EIO
}
