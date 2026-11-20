// Test Tarkle filesystem interface
package tarkle_test

import (
	"testing"

	"github.com/rclone/rclone/backend/tarkle"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestTarkle:",
		NilObject:  (*tarkle.Object)(nil),
	})
}
