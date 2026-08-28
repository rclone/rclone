// Test memory filesystem interface
package memory

import (
	"testing"

	"github.com/PhateValleyman/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName:  ":memory:",
		NilObject:   (*Object)(nil),
		QuickTestOK: true,
	})
}
