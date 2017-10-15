// Test Mega filesystem interface
package mega_test

import (
	"testing"

	"github.com/ncw/rclone/backend/mega"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestMega:",
		NilObject:  (*mega.Object)(nil),
	})
}
