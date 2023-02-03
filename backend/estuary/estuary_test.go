package estuary_test

import (
	"github.com/rclone/rclone/backend/estuary"
	"github.com/rclone/rclone/fstest/fstests"
	"testing"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:      "TestEstuary:",
		NilObject:       (*estuary.Object)(nil),
		SkipInvalidUTF8: true,
	})
}
