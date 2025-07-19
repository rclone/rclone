// Test Internxt filesystem interface

package internxt_test

import (
	"testing"

	"github.com/rclone/rclone/backend/internxt"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestInternxt:",
		NilObject:  (*internxt.Object)(nil),
	})
}
