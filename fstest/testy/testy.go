// Package testy contains test utilities for rclone
package testy

import (
	"os"
	"testing"
)

// CI returns true if we are running on the CI server
func CI() bool {
	return os.Getenv("CI") != ""
}

// SkipUnreliable skips this test if running on CI
func SkipUnreliable(t *testing.T) {
	if !CI() {
		return
	}
	t.Skip("Skipping Unreliable Test on CI")
}
