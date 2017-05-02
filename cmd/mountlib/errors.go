// Cross platform errors

package mountlib

import "fmt"

// Error describes low level errors in a cross platform way
type Error byte

// Low level errors
const (
	OK Error = iota
	ENOENT
	ENOTEMPTY
	EEXIST
	ESPIPE
	EBADF
)

var errorNames = []string{
	OK:        "Success",
	ENOENT:    "No such file or directory",
	ENOTEMPTY: "Directory not empty",
	EEXIST:    "File exists",
	ESPIPE:    "Illegal seek",
	EBADF:     "Bad file descriptor",
}

// Error renders the error as a string
func (e Error) Error() string {
	if int(e) >= len(errorNames) {
		return fmt.Sprintf("Low level error %d", e)
	}
	return errorNames[e]
}
