package estuary_test

import (
	"github.com/rclone/rclone/backend/estuary"
	"testing"

	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:      "TestEstuary:",
		NilObject:       (*estuary.Object)(nil),
		SkipInvalidUTF8: true,
	})
}
