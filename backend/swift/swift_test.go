// Test Swift filesystem interface
package swift_test

import (
	"testing"

	"github.com/ncw/rclone/backend/swift"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestSwift:",
		NilObject:  (*swift.Object)(nil),
	})
}
