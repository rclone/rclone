// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!plan9,!solaris

package daemon

import (
	"os"
)

// A Context describes daemon context.
type Context struct {
	// If PidFileName is non-empty, parent process will try to create and lock
	// pid file with given name. Child process writes process id to file.
	PidFileName string
	// Permissions for new pid file.
	PidFilePerm os.FileMode

	// If LogFileName is non-empty, parent process will create file with given name
	// and will link to fd 2 (stderr) for child process.
	LogFileName string
	// Permissions for new log file.
	LogFilePerm os.FileMode

	// If WorkDir is non-empty, the child changes into the directory before
	// creating the process.
	WorkDir string
	// If Chroot is non-empty, the child changes root directory
	Chroot string

	// If Env is non-nil, it gives the environment variables for the
	// daemon-process in the form returned by os.Environ.
	// If it is nil, the result of os.Environ will be used.
	Env []string
	// If Args is non-nil, it gives the command-line args for the
	// daemon-process. If it is nil, the result of os.Args will be used
	// (without program name).
	Args []string

	// If Umask is non-zero, the daemon-process call Umask() func with given value.
	Umask int
}

func (d *Context) reborn() (child *os.Process, err error) {
	return nil, errNotSupported
}

func (d *Context) search() (daemon *os.Process, err error) {
	return nil, errNotSupported
}

func (d *Context) release() (err error) {
	return errNotSupported
}
