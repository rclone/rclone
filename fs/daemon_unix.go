// Daemonization interface for Unix platforms (common definitions)

//go:build !windows && !plan9 && !js

package fs

import (
	"os"
)

// We use a special environment variable to let the child process know its role.
const (
	DaemonMarkVar   = "_RCLONE_DAEMON_"
	DaemonMarkChild = "_rclone_daemon_"
)

// IsDaemon returns true if this process runs in background
func IsDaemon() bool {
	return os.Getenv(DaemonMarkVar) == DaemonMarkChild
}
