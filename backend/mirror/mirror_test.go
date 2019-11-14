// Test Mirror filesystem interface
package mirror_test

import (
	"testing"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/backend/mirror"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  "TestMirror:",
		NilObject:   (*mirror.Object)(nil),
		SkipFsMatch: true,
	})
}
