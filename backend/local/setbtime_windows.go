//go:build windows

package local

import (
	"syscall"
	"time"
)

const haveSetBTime = true

// setTimes sets any of atime, mtime or btime
// if link is set it sets a link rather than the target
func setTimes(name string, atime, mtime, btime time.Time, link bool) (err error) {
	pathp, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	fileFlag := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	if link {
		fileFlag |= syscall.FILE_FLAG_OPEN_REPARSE_POINT
	}
	h, err := syscall.CreateFile(pathp,
		syscall.FILE_WRITE_ATTRIBUTES, syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, fileFlag, 0)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := syscall.Close(h)
		if err == nil {
			err = closeErr
		}
	}()
	var patime, pmtime, pbtime *syscall.Filetime
	if !atime.IsZero() {
		t := syscall.NsecToFiletime(atime.UnixNano())
		patime = &t
	}
	if !mtime.IsZero() {
		t := syscall.NsecToFiletime(mtime.UnixNano())
		pmtime = &t
	}
	if !btime.IsZero() {
		t := syscall.NsecToFiletime(btime.UnixNano())
		pbtime = &t
	}
	return syscall.SetFileTime(h, pbtime, patime, pmtime)
}

// setBTime sets the birth time of the file passed in
func setBTime(name string, btime time.Time) (err error) {
	return setTimes(name, time.Time{}, time.Time{}, btime, false)
}

// lsetBTime changes the birth time of the link passed in
func lsetBTime(name string, btime time.Time) error {
	return setTimes(name, time.Time{}, time.Time{}, btime, true)
}
