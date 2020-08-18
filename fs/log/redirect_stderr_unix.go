// Log the panic under unix to the log file

// +build !windows,!solaris,!plan9,!js

package log

import (
	"log"
	"os"

	"github.com/rclone/rclone/fs/config"
	"golang.org/x/sys/unix"
)

// redirectStderr to the file passed in
func redirectStderr(f *os.File) {
	passPromptFd, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		log.Fatalf("Failed to duplicate stderr: %v", err)
	}
	config.PasswordPromptOutput = os.NewFile(uintptr(passPromptFd), "passPrompt")
	err = unix.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		log.Fatalf("Failed to redirect stderr to file: %v", err)
	}
}
