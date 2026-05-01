//go:build !windows

package sync

import (
	"testing"
)

// Helper function that only runs in a separate child process to lock a file for testing purposes
func lockFileExclusive(t *testing.T, _ string) {
	t.Helper()
	t.Skip("exclusive file locking is only supported on Windows")
}
