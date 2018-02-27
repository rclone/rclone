// +build dragonfly freebsd linux netbsd openbsd plan9 solaris

package daemon

import (
	"fmt"
	"syscall"
)

func lockFile(fd uintptr) error {
	err := syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK {
		err = ErrWouldBlock
	}
	return err
}

func unlockFile(fd uintptr) error {
	err := syscall.Flock(int(fd), syscall.LOCK_UN)
	if err == syscall.EWOULDBLOCK {
		err = ErrWouldBlock
	}
	return err
}

const pathMax = 0x1000

func getFdName(fd uintptr) (name string, err error) {
	path := fmt.Sprintf("/proc/self/fd/%d", int(fd))
	// We use predefined pathMax const because /proc directory contains special files
	// so that unable to get correct size of pseudo-symlink through lstat.
	// please see notes and example for readlink syscall:
	// http://man7.org/linux/man-pages/man2/readlink.2.html#NOTES
	buf := make([]byte, pathMax)
	var n int
	if n, err = syscall.Readlink(path, buf); err == nil {
		name = string(buf[:n])
	}
	return
}
