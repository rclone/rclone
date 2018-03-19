// +build darwin

package daemon

import (
	"syscall"
	"unsafe"
)

import "C"

// darwin's MAXPATHLEN
const maxpathlen = 1024

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

func getFdName(fd uintptr) (name string, err error) {
	buf := make([]C.char, maxpathlen+1)
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETPATH, uintptr(unsafe.Pointer(&buf[0])))
	if errno == 0 {
		return C.GoString(&buf[0]), nil
	}
	return "", errno
}
