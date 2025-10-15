// Test piqlConnect filesystem interface
package piqlconnect_test

import (
	"testing"

	"github.com/rclone/rclone/backend/piqlconnect"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestPiqlConnect:Workspace/FirstPackage",
		NilObject:  (*piqlconnect.Object)(nil),
	})
}
