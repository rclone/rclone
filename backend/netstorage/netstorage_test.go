package netstorage_test

import (
	"testing"

	"github.com/PhateValleyman/rclone/backend/netstorage"
	"github.com/PhateValleyman/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestnStorage:",
		NilObject:  (*netstorage.Object)(nil),
	})
}
