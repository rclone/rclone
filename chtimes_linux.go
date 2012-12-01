// An implementation of Chtimes which preserves nanosecond precision under linux
//
// Should go in standard library, but here for now

// +build linux

package main

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

// COPIED from syscall
// byteSliceFromString returns a NUL-terminated slice of bytes
// containing the text of s. If s contains a NUL byte at any
// location, it returns (nil, EINVAL).
func byteSliceFromString(s string) ([]byte, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return nil, syscall.EINVAL
		}
	}
	a := make([]byte, len(s)+1)
	copy(a, s)
	return a, nil
}

// COPIED from syscall
// bytePtrFromString returns a pointer to a NUL-terminated array of
// bytes containing the text of s. If s contains a NUL byte at any
// location, it returns (nil, EINVAL).
func bytePtrFromString(s string) (*byte, error) {
	a, err := byteSliceFromString(s)
	if err != nil {
		return nil, err
	}
	return &a[0], nil
}

// COPIED from syscall auto generated code modified from utimes
func utimensat(dirfd int, path string, times *[2]syscall.Timespec) (err error) {
	var _p0 *byte
	_p0, err = bytePtrFromString(path)
	if err != nil {
		return
	}
	_, _, e1 := syscall.Syscall(syscall.SYS_UTIMENSAT, uintptr(dirfd), uintptr(unsafe.Pointer(_p0)), uintptr(unsafe.Pointer(times)))
	if e1 != 0 {
		err = e1
	}
	return
}

// FIXME needs defining properly!
const AT_FDCWD = -100

// COPIED from syscall and modified
//sys	utimes(path string, times *[2]Timeval) (err error)
func Utimensat(dirfd int, path string, ts []syscall.Timespec) (err error) {
	if len(ts) != 2 {
		return syscall.EINVAL
	}
	return utimensat(dirfd, path, (*[2]syscall.Timespec)(unsafe.Pointer(&ts[0])))
}

// COPIED from syscall and modified
func Chtimes(name string, atime time.Time, mtime time.Time) error {
	var utimes [2]syscall.Timespec
	utimes[0] = syscall.NsecToTimespec(atime.UnixNano())
	utimes[1] = syscall.NsecToTimespec(mtime.UnixNano())
	if e := Utimensat(AT_FDCWD, name, utimes[0:]); e != nil {
		return &os.PathError{"chtimes", name, e}
	}
	return nil
}
