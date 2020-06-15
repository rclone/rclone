// Package testy contains test utilities for rclone
package testy

import (
	"os"
	"testing"
)

// SkipUnreliable skips this test if running on CI
func SkipUnreliable(t *testing.T) {
	if os.Getenv("CI") == "" {
		return
	}
	t.Skip("Skipping Unreliable Test on CI")
}
