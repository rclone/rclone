//go:build !unix

// Package daemonize provides daemonization stub for non-Unix platforms.
package daemonize

import (
	"fmt"
	"os"
	"runtime"
)

var errNotSupported = fmt.Errorf("daemon mode is not supported on the %s platform", runtime.GOOS)

// StartDaemon runs background twin of current process.
func StartDaemon(args []string) (*os.Process, error) {
	return nil, errNotSupported
}

// Check returns non nil if the daemon process has died
func Check(daemon *os.Process) error {
	return errNotSupported
}
