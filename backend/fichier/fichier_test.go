// Test 1Fichier filesystem interface
package fichier_test

import (
	"testing"

	"github.com/ncw/rclone/backend/box"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFichier:",
		NilObject:  (*box.Object)(nil),
	})
}
