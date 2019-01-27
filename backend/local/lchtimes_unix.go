// +build !windows,!plan9

package local

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const haveLChtimes = true

// lChtimes changes the access and modification times of the named
// link, similar to the Unix utime() or utimes() functions.
//
// The underlying filesystem may truncate or round the values to a
// less precise time unit.
// If there is an error, it will be of type *PathError.
func lChtimes(name string, atime time.Time, mtime time.Time) error {
	var utimes [2]unix.Timespec
	utimes[0] = unix.NsecToTimespec(atime.UnixNano())
	utimes[1] = unix.NsecToTimespec(mtime.UnixNano())
	if e := unix.UtimesNanoAt(unix.AT_FDCWD, name, utimes[0:], unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return &os.PathError{Op: "lchtimes", Path: name, Err: e}
	}
	return nil
}
