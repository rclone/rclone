// +build solaris

package daemon

import (
	"io"
	"syscall"
)

func lockFile(fd uintptr) error {
	lockInfo := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
		Start:  0,
		Len:    0,
	}
	if err := syscall.FcntlFlock(fd, syscall.F_SETLK, &lockInfo); err != nil {
		if err == syscall.EAGAIN {
			err = ErrWouldBlock
		}
		return err
	}
	return nil
}

func unlockFile(fd uintptr) error {
	lockInfo := syscall.Flock_t{
		Type:   syscall.F_UNLCK,
		Whence: io.SeekStart,
		Start:  0,
		Len:    0,
	}
	if err := syscall.FcntlFlock(fd, syscall.F_GETLK, &lockInfo); err != nil {
		if err == syscall.EAGAIN {
			err = ErrWouldBlock
		}
		return err
	}
	return nil
}
