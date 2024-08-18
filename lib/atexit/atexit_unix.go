//go:build !windows && !plan9

package atexit

import (
	"os"
	"syscall"

	"github.com/rclone/rclone/lib/exitcode"
)

var exitSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM} // Not syscall.SIGQUIT as we want the default behaviour

// exitCode calculates the exit code for the given signal. Many Unix programs
// exit with 128+signum if they handle signals. Most shell also implement the
// same convention if a program is terminated by an uncaught and/or fatal
// signal.
func exitCode(sig os.Signal) int {
	if real, ok := sig.(syscall.Signal); ok && int(real) > 0 {
		return 128 + int(real)
	}

	return exitcode.UncategorizedError
}
