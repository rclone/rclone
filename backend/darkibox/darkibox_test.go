// Test Darkibox filesystem interface
package darkibox_test

import (
	"testing"

	"github.com/rclone/rclone/backend/darkibox"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDarkibox:",
		NilObject:  (*darkibox.Object)(nil),
	})
}
