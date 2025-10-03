//go:build !plan9

// Test Telegram filesystem interface
package kvfs_test

import (
	"testing"

	"github.com/rclone/rclone/backend/kvfs"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:      "TestKvfs:",
		NilObject:       (*kvfs.Object)(nil),
		SkipInvalidUTF8: true,
	})
}
