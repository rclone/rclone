// Test Voltn filesystem interface
package voltn_test

import (
	"testing"

	"github.com/rclone/rclone/backend/voltn"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestVoltn:",
		NilObject:  (*voltn.Object)(nil),
	})
}
