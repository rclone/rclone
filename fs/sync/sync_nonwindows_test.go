//go:build !windows

package sync

import (
	"fmt"
	"os"
	"testing"
)

// Spawn a new process that holds an exclusive lock on the specified file.
// Blocks until the lock is acquired, then returns a cleanup function to release the lock (also blocking) when called.
func createExclusiveFileLock(t *testing.T, _ string) func() {
	t.Helper()
	return nil
}

// Helper function that only runs in a separate child process to hold an exclusive lock on a file until signaled to release it
func holdExclusiveFileLock(t *testing.T, _ string) {
	t.Helper()
	fmt.Fprint(os.Stderr, "Exclusive file locking is only supported on Windows")
	os.Exit(1)
}
