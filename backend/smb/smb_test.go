// Test smb filesystem interface
package smb_test

import (
	"testing"

	_ "github.com/rclone/rclone/backend/alias"
	"github.com/rclone/rclone/backend/smb"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSMB:",
		NilObject:  (*smb.Object)(nil),
	})
}
