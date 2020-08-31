//go:build !windows
// +build !windows

package operations

import (
	"os"
	"os/exec"
	"syscall"
)

func sendInterrupt() error {
	p, err := os.FindProcess(syscall.Getpid())
	if err != nil {
		return err
	}
	err = p.Signal(os.Interrupt)
	return err
}

func setupCmd(cmd *exec.Cmd) {
	// Only needed for windows
}
