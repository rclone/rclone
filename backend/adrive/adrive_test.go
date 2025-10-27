package adrive_test

import (
	"testing"

	"github.com/rclone/rclone/backend/adrive"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestAliyunDrive:",
		NilObject:  (*adrive.Object)(nil),
	})
}
