// Package daemon provides function to daemonization processes.
// And such as the handling of system signals and the pid-file creation.
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	envVarName  = "_GO_DAEMON"
	envVarValue = "1"
)

// func Reborn daemonize process. Function Reborn calls ForkExec
// in the parent process and terminates him. In the child process,
// function sets umask, work dir and calls Setsid. Function sets
// for child process environment variable _GO_DAEMON=1 - the mark,
// might used for debug.
func Reborn(umask uint32, workDir string) (err error) {

	if !WasReborn() {
		// parent process - fork and exec
		var path string
		if path, err = filepath.Abs(os.Args[0]); err != nil {
			return
		}

		cmd := prepareCommand(path)

		if err = cmd.Start(); err != nil {
			return
		}

		os.Exit(0)
	}

	// child process - daemon
	syscall.Umask(int(umask))

	if len(workDir) != 0 {
		if err = os.Chdir(workDir); err != nil {
			return
		}
	}

	_, err = syscall.Setsid()

	// Do not required redirect std
	// to /dev/null, this work was
	// done function ForkExec

	return
}

// func WasReborn, return true if the process has environment
// variable _GO_DAEMON=1 (child process).
func WasReborn() bool {
	return os.Getenv(envVarName) == envVarValue
}

func prepareCommand(path string) (cmd *exec.Cmd) {

	// prepare command-line arguments
	cmd = exec.Command(path, os.Args[1:]...)

	// prepare environment variables
	envVar := fmt.Sprintf("%s=%s", envVarName, envVarValue)
	cmd.Env = append(os.Environ(), envVar)

	return
}

// func RedirectStream redirects file s to file target.
func RedirectStream(s, target *os.File) (err error) {

	stdoutFd := int(s.Fd())
	if err = syscall.Close(stdoutFd); err != nil {
		return
	}

	err = syscall.Dup2(int(target.Fd()), stdoutFd)

	return
}
