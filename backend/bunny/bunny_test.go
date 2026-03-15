package bunny_test

import (
	"testing"

	"github.com/rclone/rclone/backend/bunny"
	"github.com/rclone/rclone/fstest/fstests"
)

// The following fs tests will fail and is excluded from integration tests
// - FsRmdirNotFound
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestBunny:",
		NilObject:  (*bunny.Object)(nil),
	})
}
