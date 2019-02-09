// Test ADB filesystem interface
package adb_test

import (
	"testing"

	"github.com/ncw/rclone/backend/adb"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestAdb:/data/local/tmp",
		NilObject:  (*adb.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: "TestAdb", Key: "copy_links", Value: "true"},
		},
	})
}
