// Device reading functions

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package local

import (
	"os"
	"syscall"

	"github.com/ncw/rclone/fs"
)

var (
	oneFileSystem = fs.BoolP("one-file-system", "x", false, "Don't cross filesystem boundaries.")
)

// readDevice turns a valid os.FileInfo into a device number,
// returning devUnset if it fails.
func readDevice(fi os.FileInfo) uint64 {
	if !*oneFileSystem {
		return devUnset
	}
	statT, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		fs.Debug(fi.Name(), "Type assertion fi.Sys().(*syscall.Stat_t) failed from: %#v", fi.Sys())
		return devUnset
	}
	return uint64(statT.Dev)
}
