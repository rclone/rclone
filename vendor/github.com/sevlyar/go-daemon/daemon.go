package daemon

import (
	"errors"
	"os"
	"syscall"
)

var errNotSupported = errors.New("daemon: Non-POSIX OS is not supported")

// Mark of daemon process - system environment variable _GO_DAEMON=1
const (
	MARK_NAME  = "_GO_DAEMON"
	MARK_VALUE = "1"
)

// Default file permissions for log and pid files.
const FILE_PERM = os.FileMode(0640)

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

	// Credential holds user and group identities to be assumed by a daemon-process.
	Credential *syscall.Credential
	// If Umask is non-zero, the daemon-process call Umask() func with given value.
	Umask int

	// Struct contains only serializable public fields (!!!)
	abspath  string
	pidFile  *LockFile
	logFile  *os.File
	nullFile *os.File

	rpipe, wpipe *os.File
}

// WasReborn returns true in child process (daemon) and false in parent process.
func WasReborn() bool {
	return os.Getenv(MARK_NAME) == MARK_VALUE
}

// Reborn runs second copy of current process in the given context.
// function executes separate parts of code in child process and parent process
// and provides demonization of child process. It look similar as the
// fork-daemonization, but goroutine-safe.
// In success returns *os.Process in parent process and nil in child process.
// Otherwise returns error.
func (d *Context) Reborn() (child *os.Process, err error) {
	return d.reborn()
}

// Search search daemons process by given in context pid file name.
// If success returns pointer on daemons os.Process structure,
// else returns error. Returns nil if filename is empty.
func (d *Context) Search() (daemon *os.Process, err error) {
	return d.search()
}

// Release provides correct pid-file release in daemon.
func (d *Context) Release() (err error) {
	return d.release()
}
