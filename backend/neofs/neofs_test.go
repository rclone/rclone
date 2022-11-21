package neofs

import (
	"testing"

	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	debug := true
	fstest.Verbose = &debug
	fstests.Run(t, &fstests.Opt{
		RemoteName:      "TestNeoFS:",
		NilObject:       (*Object)(nil),
		SkipInvalidUTF8: true,
	})
}
