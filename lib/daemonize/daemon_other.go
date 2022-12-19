//go:build windows || plan9 || js
// +build windows plan9 js

// Package daemonize provides daemonization stub for non-Unix platforms.
package daemonize

import (
	"fmt"
	"os"
	"runtime"
)

// StartDaemon runs background twin of current process.
func StartDaemon(args []string) (*os.Process, error) {
	return nil, fmt.Errorf("background mode is not supported on %s platform", runtime.GOOS)
}
