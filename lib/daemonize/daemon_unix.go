// Daemonization interface for Unix platforms (implementation)

//go:build !windows && !plan9 && !js
// +build !windows,!plan9,!js

package daemonize

import (
	"os"
	"syscall"

	"github.com/rclone/rclone/fs"
)

// StartDaemon runs background twin of current process.
// It executes separate parts of code in child and parent processes.
// Returns child process pid in the parent or nil in the child.
// The method looks like a fork but safe for goroutines.
func StartDaemon(args []string) (*os.Process, error) {
	if fs.IsDaemon() {
		// This process is already daemonized
		return nil, nil
	}

	env := append(os.Environ(), fs.DaemonMarkVar+"="+fs.DaemonMarkChild)

	me, err := os.Executable()
	if err != nil {
		me = os.Args[0]
	}

	null, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	files := []*os.File{
		null, // (0) stdin
		null, // (1) stdout
		null, // (2) stderr
	}
	sysAttr := &syscall.SysProcAttr{
		// setsid (https://linux.die.net/man/2/setsid) in the child process will reset
		// its process group id (PGID) to its PID thus detaching it from parent.
		// This would make autofs fail because it detects mounting process by its PGID.
		Setsid: false,
	}
	attr := &os.ProcAttr{
		Env:   env,
		Files: files,
		Sys:   sysAttr,
	}

	daemon, err := os.StartProcess(me, args, attr)
	if err != nil {
		return nil, err
	}

	return daemon, nil
}
