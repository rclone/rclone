// Test Put.io filesystem interface
package putio_test

import (
	"testing"

	"github.com/rclone/rclone/backend/putio"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestPutio:",
		NilObject:  (*putio.Object)(nil),
	})
}
