// Syslog interface for non-Unix variants only

//go:build windows || nacl || plan9

package log

import (
	"runtime"

	"github.com/rclone/rclone/fs"
)

// Starts syslog if configured, returns true if it was started
func startSysLog() bool {
	fs.Fatalf(nil, "--syslog not supported on %s platform", runtime.GOOS)
	return false
}
