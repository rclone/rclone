//go:build !windows && !plan9 && !js && !linux

package local

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const haveLChmod = true

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
//
// Borrowed from the syscall source since it isn't public.
func syscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	return o
}

// lChmod changes the mode of the named file to mode. If the file is a symbolic
// link, it changes the link, not the target. If there is an error,
// it will be of type *PathError.
func lChmod(name string, mode os.FileMode) error {
	// NB linux does not support AT_SYMLINK_NOFOLLOW as a parameter to fchmodat
	// and returns ENOTSUP if you try, so we don't support this on linux
	if e := unix.Fchmodat(unix.AT_FDCWD, name, syscallMode(mode), unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return &os.PathError{Op: "lChmod", Path: name, Err: e}
	}
	return nil
}
