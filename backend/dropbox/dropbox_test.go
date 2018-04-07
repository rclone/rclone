// Test Dropbox filesystem interface
package dropbox_test

import (
	"testing"

	"github.com/ncw/rclone/backend/dropbox"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDropbox:",
		NilObject:  (*dropbox.Object)(nil),
	})
}
