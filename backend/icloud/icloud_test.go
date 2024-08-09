package icloud_test

import (
	"testing"

	"github.com/rclone/rclone/backend/icloud"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestICloudDrive:",
		NilObject:  (*icloud.Object)(nil),
	})
}
