//go:build windows

package local

import (
	"time"
)

const haveLChtimes = true

// lChtimes changes the access and modification times of the named
// link, similar to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a
// less precise time unit.
// If there is an error, it will be of type *PathError.
func lChtimes(name string, atime time.Time, mtime time.Time) error {
	return setTimes(name, atime, mtime, time.Time{}, true)
}
