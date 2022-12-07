package baidupcs_test

import (
	"github.com/rclone/rclone/backend/baidupcs"
	"github.com/rclone/rclone/fstest/fstests"
	"testing"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestBaiduPCS:",
		NilObject:  (*baidupcs.Object)(nil),
	})
}
