// Cross platform errors

package vfs

import (
	"fmt"
	"os"
)

// Error describes low level errors in a cross platform way.
type Error byte

// NB if changing errors translateError in cmd/mount/fs.go, cmd/cmount/fs.go

// Low level errors
const (
	OK Error = iota
	ENODATA
	ENOTEMPTY
	ESPIPE
	EBADF
	EROFS
	ENOSYS
	ENOATTR
	ERANGE
)

// Errors which have exact counterparts in os
var (
	ENOENT  = os.ErrNotExist
	EEXIST  = os.ErrExist
	EPERM   = os.ErrPermission
	EINVAL  = os.ErrInvalid
	ECLOSED = os.ErrClosed
)

var errorNames = []string{
	OK:        "Success",
	ENODATA:   "No data available",
	ENOTEMPTY: "Directory not empty",
	ESPIPE:    "Illegal seek",
	EBADF:     "Bad file descriptor",
	EROFS:     "Read only file system",
	ENOSYS:    "Function not implemented",
	ENOATTR:   "No such attribute",
	ERANGE:    "Result too large",
}

// Error renders the error as a string
func (e Error) Error() string {
	if int(e) >= len(errorNames) {
		return fmt.Sprintf("Low level error %d", e)
	}
	return errorNames[e]
}
