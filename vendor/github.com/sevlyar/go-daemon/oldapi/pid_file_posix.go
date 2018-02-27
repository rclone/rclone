package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// PidFile contains information of pid-file.
type PidFile struct {
	file *os.File
	path string
}

// ErrWoldBlock indicates on locking pid-file by another process.
var ErrWouldBlock = syscall.EWOULDBLOCK

// func LockPidFile trys create and lock pid-file.
func LockPidFile(path string, perm os.FileMode) (pidf *PidFile, err error) {
	var fileLen int

	var file *os.File
	file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return
	}

	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK {
			// allready locked by other process instance
			file.Close()
			return
		}
		goto SKIP
	}

	if fileLen, err = fmt.Fprint(file, os.Getpid()); err != nil {
		goto SKIP
	}

	if err = file.Truncate(int64(fileLen)); err != nil {
		goto SKIP
	}

SKIP:
	if err != nil {
		syscall.Unlink(path)
		file.Close()
	} else {
		pidf = &PidFile{file, path}
	}

	return
}

// func Unlock unlocks and removes pid-file.
func (pidf *PidFile) Unlock() (err error) {

	err = syscall.Unlink(pidf.path)
	err2 := pidf.file.Close()

	// return one of two errors
	if err == nil {
		err = err2
	}
	return
}
