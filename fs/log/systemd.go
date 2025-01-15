// Systemd interface for non-Unix variants only

//go:build !unix

package log

import (
	"runtime"

	"github.com/rclone/rclone/fs"
)

// Enables systemd logs if configured or if auto-detected
func startSystemdLog() bool {
	fs.Fatalf(nil, "--log-systemd not supported on %s platform", runtime.GOOS)
	return false
}

func isJournalStream() bool {
	return false
}
