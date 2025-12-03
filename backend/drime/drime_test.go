// Test Drime filesystem interface
package drime_test

import (
	"testing"

	"github.com/rclone/rclone/backend/drime"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDrime:",
		NilObject:  (*drime.Object)(nil),
	})
}
