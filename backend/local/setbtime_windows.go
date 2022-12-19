//go:build windows
// +build windows

package local

import (
	"os"
	"syscall"
	"time"
)

const haveSetBTime = true

// setBTime sets the birth time of the file passed in
func setBTime(name string, btime time.Time) (err error) {
	h, err := syscall.Open(name, os.O_RDWR, 0755)
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
