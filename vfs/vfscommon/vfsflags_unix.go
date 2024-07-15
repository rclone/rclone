//go:build linux || darwin || freebsd

package vfscommon

import (
	"golang.org/x/sys/unix"
)

// get the current umask
func getUmask() int {
	umask := unix.Umask(0) // read the umask
	unix.Umask(umask)      // set it back to what it was
	return umask
}

// get the current uid
func getUID() uint32 {
	return uint32(unix.Geteuid())
}

// get the current gid
func getGID() uint32 {
	return uint32(unix.Getegid())
}
