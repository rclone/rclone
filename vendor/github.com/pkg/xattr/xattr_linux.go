//go:build linux
// +build linux

package xattr

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// XATTR_SUPPORTED will be true if the current platform is supported
	XATTR_SUPPORTED = true

	XATTR_CREATE  = unix.XATTR_CREATE
	XATTR_REPLACE = unix.XATTR_REPLACE

	// ENOATTR is not exported by the syscall package on Linux, because it is
	// an alias for ENODATA. We export it here so it is available on all
	// our supported platforms.
	ENOATTR = syscall.ENODATA
)

// On Linux, FUSE and CIFS filesystems can return EINTR for interrupted system
// calls. This function works around this by retrying system calls until they
// stop returning EINTR.
//
// See https://github.com/golang/go/commit/6b420169d798c7ebe733487b56ea5c3fa4aab5ce.
func ignoringEINTR(fn func() error) (err error) {
	for {
		err = fn()
		if err != unix.EINTR {
			break
		}
	}
	return err
}

func getxattr(path string, name string, data []byte) (int, error) {
	var r int
	err := ignoringEINTR(func() (err error) {
		r, err = unix.Getxattr(path, name, data)
		return err
	})
	return r, err
}

func lgetxattr(path string, name string, data []byte) (int, error) {
	var r int
	err := ignoringEINTR(func() (err error) {
		r, err = unix.Lgetxattr(path, name, data)
		return err
	})
	return r, err
}

func fgetxattr(f *os.File, name string, data []byte) (int, error) {
	var r int
	err := ignoringEINTR(func() (err error) {
		r, err = unix.Fgetxattr(int(f.Fd()), name, data)
		return err
	})
	return r, err
}

func setxattr(path string, name string, data []byte, flags int) error {
	return ignoringEINTR(func() (err error) {
		return unix.Setxattr(path, name, data, flags)
	})
}

func lsetxattr(path string, name string, data []byte, flags int) error {
	return ignoringEINTR(func() (err error) {
		return unix.Lsetxattr(path, name, data, flags)
	})
}

func fsetxattr(f *os.File, name string, data []byte, flags int) error {
	return ignoringEINTR(func() (err error) {
		return unix.Fsetxattr(int(f.Fd()), name, data, flags)
	})
}

func removexattr(path string, name string) error {
	return ignoringEINTR(func() (err error) {
		return unix.Removexattr(path, name)
	})
}

func lremovexattr(path string, name string) error {
	return ignoringEINTR(func() (err error) {
		return unix.Lremovexattr(path, name)
	})
}

func fremovexattr(f *os.File, name string) error {
	return ignoringEINTR(func() (err error) {
		return unix.Fremovexattr(int(f.Fd()), name)
	})
}

func listxattr(path string, data []byte) (int, error) {
	var r int
	err := ignoringEINTR(func() (err error) {
		r, err = unix.Listxattr(path, data)
		return err
	})
	return r, err
}

func llistxattr(path string, data []byte) (int, error) {
	var r int
	err := ignoringEINTR(func() (err error) {
		r, err = unix.Llistxattr(path, data)
		return err
	})
	return r, err
}

func flistxattr(f *os.File, data []byte) (int, error) {
	var r int
	err := ignoringEINTR(func() (err error) {
		r, err = unix.Flistxattr(int(f.Fd()), data)
		return err
	})
	return r, err
}

// stringsFromByteSlice converts a sequence of attributes to a []string.
// On Darwin and Linux, each entry is a NULL-terminated string.
func stringsFromByteSlice(buf []byte) (result []string) {
	offset := 0
	for index, b := range buf {
		if b == 0 {
			result = append(result, string(buf[offset:index]))
			offset = index + 1
		}
	}
	return
}
