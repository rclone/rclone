package sqlar_test

import (
	"testing"

	"github.com/rclone/rclone/backend/sqlar"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  ":sqlar,path=" + t.TempDir() + "/test.sqlar:",
		NilObject:   (*sqlar.Object)(nil),
		QuickTestOK: true,
	})
}
