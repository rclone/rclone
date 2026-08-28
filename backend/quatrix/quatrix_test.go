// Test Quatrix filesystem interface
package quatrix_test

import (
	"testing"

	"github.com/PhateValleyman/rclone/backend/quatrix"
	"github.com/PhateValleyman/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestQuatrix:",
		NilObject:  (*quatrix.Object)(nil),
	})
}
