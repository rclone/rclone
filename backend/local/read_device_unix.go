// Device reading functions

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package local

import (
	"os"
	"syscall"

	"github.com/rclone/rclone/fs"
)

// readDevice turns a valid os.FileInfo into a device number,
// returning devUnset if it fails.
func readDevice(fi os.FileInfo, oneFileSystem bool) uint64 {
	if !oneFileSystem {
		return devUnset
	}
	statT, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		fs.Debugf(fi.Name(), "Type assertion fi.Sys().(*syscall.Stat_t) failed from: %#v", fi.Sys())
		return devUnset
	}
	return uint64(statT.Dev) // nolint: unconvert
}
