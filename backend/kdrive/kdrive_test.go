// Test kdrive filesystem interface
package kdrive_test

import (
	"testing"

	"github.com/rclone/rclone/backend/kdrive"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestKdrive:",
		NilObject:  (*kdrive.Object)(nil),
	})
}
