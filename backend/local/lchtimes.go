// +build windows plan9 js

package local

import (
	"time"
)

const haveLChtimes = false

// lChtimes changes the access and modification times of the named
// link, similar to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a
// less precise time unit.
// If there is an error, it will be of type *PathError.
func lChtimes(name string, atime time.Time, mtime time.Time) error {
	// Does nothing
	return nil
}
