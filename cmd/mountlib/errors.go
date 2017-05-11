// Cross platform errors

package mountlib

import "fmt"

// Error describes low level errors in a cross platform way
type Error byte

// NB if changing errors translateError in cmd/mount/fs.go, cmd/cmount/fs.go

// Low level errors
const (
	OK Error = iota
	ENOENT
	ENOTEMPTY
	EEXIST
	ESPIPE
	EBADF
	EROFS
)

var errorNames = []string{
	OK:        "Success",
	ENOENT:    "No such file or directory",
	ENOTEMPTY: "Directory not empty",
	EEXIST:    "File exists",
	ESPIPE:    "Illegal seek",
	EBADF:     "Bad file descriptor",
	EROFS:     "Read only file system",
}

// Error renders the error as a string
func (e Error) Error() string {
	if int(e) >= len(errorNames) {
		return fmt.Sprintf("Low level error %d", e)
	}
	return errorNames[e]
}
