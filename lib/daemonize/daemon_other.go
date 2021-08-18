// Daemonization stub for non-Unix platforms (implementation)

//go:build windows || plan9 || js
// +build windows plan9 js

package daemonize

import (
	"os"
	"runtime"

	"github.com/pkg/errors"
)

// StartDaemon runs background twin of current process.
func StartDaemon(args []string) (*os.Process, error) {
	return nil, errors.Errorf("background mode is not supported on %s platform", runtime.GOOS)
}
