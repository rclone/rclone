// Test IMAP filesystem interface
package imap

import (
	"testing"

	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestIMAP:",
		NilObject:  (*Object)(nil),
	})
}
