// Windows event logging stubs for non windows machines

//go:build !windows

package log

import (
	"fmt"
	"runtime"
)

// Starts windows event log if configured.
func startWindowsEventLog(*OutputHandler) error {
	return fmt.Errorf("windows event log not supported on %s platform", runtime.GOOS)
}
