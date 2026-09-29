// Test WebHDFS filesystem interface
package webhdfs_test

import (
	"testing"

	"github.com/rclone/rclone/backend/webhdfs"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestWebHDFS:",
		NilObject:  (*webhdfs.Object)(nil),
	})
}
