// Log the panic under windows to the log file
//
// Code from minix, via
//
// https://play.golang.org/p/kLtct7lSUg

//go:build windows
// +build windows

package log

import (
	"log"
	"os"

	"github.com/rclone/rclone/lib/terminal"
	"golang.org/x/sys/windows"
)

// dup oldfd creating a functional copy as newfd
// conceptually the same as the unix `dup()` function
func dup(oldfd uintptr) (newfd uintptr, err error) {
	var (
		newfdHandle   windows.Handle
		processHandle = windows.CurrentProcess()
	)
	err = windows.DuplicateHandle(
		processHandle,                 // hSourceProcessHandle
		windows.Handle(oldfd),         // hSourceHandle
		processHandle,                 // hTargetProcessHandle
		&newfdHandle,                  // lpTargetHandle
		0,                             // dwDesiredAccess
		true,                          // bInheritHandle
		windows.DUPLICATE_SAME_ACCESS, // dwOptions
	)
	if err != nil {
		return 0, err
	}
	return uintptr(newfdHandle), nil
}

// redirectStderr to the file passed in
func redirectStderr(f *os.File) {
	termFd, err := dup(os.Stderr.Fd())
	if err != nil {
		log.Fatalf("Failed to duplicate stderr: %v", err)
	}
	terminal.RawOut = os.NewFile(termFd, "termOut")
	err = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
	os.Stderr = f
}
