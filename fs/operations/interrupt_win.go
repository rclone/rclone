//go:build windows
// +build windows

package operations

import (
	"os/exec"
	"syscall"
)

// Credit: https://github.com/golang/go/blob/6125d0c4265067cdb67af1340bf689975dd128f4/src/os/signal/signal_windows_test.go#L18
func sendInterrupt() error {
	d, e := syscall.LoadDLL("kernel32.dll")
	if e != nil {
		return e
	}
	p, e := d.FindProc("GenerateConsoleCtrlEvent")
	if e != nil {
		return e
	}
	r, _, e := p.Call(syscall.CTRL_BREAK_EVENT, uintptr(syscall.Getpid()))
	if r == 0 {
		return e
	}
	return nil
}

func setupCmd(cmd *exec.Cmd) {
	(*cmd).SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
