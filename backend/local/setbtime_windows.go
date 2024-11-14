//go:build windows

package local

import (
	"syscall"
	"time"
)

const haveSetBTime = true

// setBTime sets the birth time of the file passed in
func setBTime(name string, btime time.Time) (err error) {
	pathp, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	h, err := syscall.CreateFile(pathp,
		syscall.FILE_WRITE_ATTRIBUTES, syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := syscall.Close(h)
		if err == nil {
			err = closeErr
		}
	}()
	bFileTime := syscall.NsecToFiletime(btime.UnixNano())
	return syscall.SetFileTime(h, &bFileTime, nil, nil)
}
