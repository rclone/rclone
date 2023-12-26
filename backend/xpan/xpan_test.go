package xpan_test

import (
	"testing"

	"github.com/rclone/rclone/backend/xpan"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "baidu:/apps/rclone",
		NilObject:  (*xpan.Object)(nil),
	})
}
