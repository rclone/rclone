// Log the panic under unix to the log file

//go:build !windows && !solaris && !plan9 && !js
// +build !windows,!solaris,!plan9,!js

package log

import (
	"log"
	"os"

	"github.com/rclone/rclone/lib/terminal"
	"golang.org/x/sys/unix"
)

// redirectStderr to the file passed in
func redirectStderr(f *os.File) {
	termFd, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		log.Fatalf("Failed to duplicate stderr: %v", err)
	}
	terminal.RawOut = os.NewFile(uintptr(termFd), "termOut")
	err = unix.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
}
