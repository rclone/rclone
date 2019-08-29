// Test Mailru filesystem interface
package mailru_test

import (
	"testing"

	"github.com/rclone/rclone/backend/mailru"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	remoteName := *fstest.RemoteName // from the -remote cli option
	if remoteName == "" {
		remoteName = "TestMailru:"
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:               remoteName,
		NilObject:                (*mailru.Object)(nil),
		SkipBadWindowsCharacters: true,
	})
}
