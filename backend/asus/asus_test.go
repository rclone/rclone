// Test AsusWebStorage filesystem interface
package asus_test

import (
	"testing"

	"github.com/rclone/rclone/backend/asus"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestAsus:",
		NilObject:  (*asus.Object)(nil),
	})
}
