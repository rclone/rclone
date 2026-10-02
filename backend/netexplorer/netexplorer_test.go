// Test NetExplorer filesystem interface
package netexplorer_test

import (
	"testing"

	"github.com/rclone/rclone/backend/netexplorer"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestNetExplorer:",
		NilObject:  (*netexplorer.Object)(nil),
	})
}
