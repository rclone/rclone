// Test Databricks Unity Catalog filesystem interface

//go:build !plan9

package databricks

import (
	"testing"

	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDatabricks:",
		NilObject:  (*Object)(nil),
	})
}
