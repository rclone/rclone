package filelu_test

import (
	"testing"

	"github.com/filelucom/rclone/backend/filelu"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests for the FileLu backend
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFileLu:",
		NilObject:  (*filelu.Fs)(nil),
	})
}
