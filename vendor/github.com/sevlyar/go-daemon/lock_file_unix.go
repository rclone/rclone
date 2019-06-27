// +build darwin dragonfly freebsd linux netbsd openbsd plan9

package daemon

import (
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
