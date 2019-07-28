// Test AmazonCloudDrive filesystem interface

// +build acd

package amazonclouddrive_test

import (
	"testing"

	"github.com/rclone/rclone/backend/amazonclouddrive"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.NilObject = fs.Object((*amazonclouddrive.Object)(nil))
	fstests.RemoteName = "TestAmazonCloudDrive:"
	fstests.Run(t)
}
