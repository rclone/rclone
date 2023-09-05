// Package diskusage provides a cross platform version of the statfs
// system call to read disk space usage.
package diskusage

import "errors"

// Info is returned from New showing details about the disk.
type Info struct {
	Free      uint64 // total free bytes
	Available uint64 // free bytes available to the current user
	Total     uint64 // total bytes on disk
}

// ErrUnsupported is returned if this platform doesn't support disk usage.
var ErrUnsupported = errors.New("disk usage unsupported on this platform")
