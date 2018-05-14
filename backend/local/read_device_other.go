// Device reading functions

// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris

package local

import "os"

// readDevice turns a valid os.FileInfo into a device number,
// returning devUnset if it fails.
func readDevice(fi os.FileInfo, oneFileSystem bool) uint64 {
	return devUnset
}
