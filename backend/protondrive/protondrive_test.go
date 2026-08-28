package protondrive_test

import (
	"testing"

	"github.com/PhateValleyman/rclone/backend/protondrive"
	"github.com/PhateValleyman/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestProtonDrive:",
		NilObject:  (*protondrive.Object)(nil),
	})
}
