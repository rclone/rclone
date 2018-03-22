package daemon

import (
	"errors"
	"os"
)

var errNotSupported = errors.New("daemon: Non-POSIX OS is not supported")

// Mark of daemon process - system environment variable _GO_DAEMON=1
const (
	MARK_NAME  = "_GO_DAEMON"
	MARK_VALUE = "1"
)

// Default file permissions for log and pid files.
const FILE_PERM = os.FileMode(0640)

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

// Search searches daemons process by given in context pid file name.
// If success returns pointer on daemons os.Process structure,
// else returns error. Returns nil if filename is empty.
func (d *Context) Search() (daemon *os.Process, err error) {
	return d.search()
}

// Release provides correct pid-file release in daemon.
func (d *Context) Release() (err error) {
	return d.release()
}
