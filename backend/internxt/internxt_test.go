package internxt

import (
	"testing"

	"github.com/rclone/rclone/backend/internxt"
	"github.com/rclone/rclone/fstest/fstests"
)

func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestInternxt:",
		NilObject:  (*internxt.Object)(nil),
	})
}
