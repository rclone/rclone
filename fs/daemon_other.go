// Daemonization stub for non-Unix platforms (common definitions)

//go:build windows || plan9 || js

package fs

// IsDaemon returns true if this process runs in background
func IsDaemon() bool {
	return false
}
