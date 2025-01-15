// Log the panic under windows to the log file
//
// Code from minix, via
//
// https://play.golang.org/p/kLtct7lSUg

//go:build windows

package log

import (
	"os"
	"syscall"

	"github.com/rclone/rclone/fs"
)

var (
	kernel32         = syscall.MustLoadDLL("kernel32.dll")
	procSetStdHandle = kernel32.MustFindProc("SetStdHandle")
)

func setStdHandle(stdhandle int32, handle syscall.Handle) error {
	r0, _, e1 := syscall.SyscallN(procSetStdHandle.Addr(), uintptr(stdhandle), uintptr(handle))
	if r0 == 0 {
		if e1 != 0 {
			return error(e1)
		}
		return syscall.EINVAL
	}
	return nil
}

// redirectStderr to the file passed in
func redirectStderr(f *os.File) {
	err := setStdHandle(syscall.STD_ERROR_HANDLE, syscall.Handle(f.Fd()))
	if err != nil {
		fs.Fatalf(nil, "Failed to redirect stderr to file: %v", err)
	}
}
