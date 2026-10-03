// Test Funambol / OneMediaHub filesystem interface
package funambol_test

import (
	"testing"

	"github.com/rclone/rclone/backend/funambol"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote configured as
// TestFunambol: in your rclone config.
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFunambol:",
		NilObject:  (*funambol.Object)(nil),
	})
}
