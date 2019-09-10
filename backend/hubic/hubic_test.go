// Test Hubic filesystem interface
package hubic_test

import (
	"testing"

	"github.com/rclone/rclone/backend/hubic"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:          "TestHubic:",
		NilObject:           (*hubic.Object)(nil),
		SkipFsCheckWrap:     true,
		SkipObjectCheckWrap: true,
	})
}
