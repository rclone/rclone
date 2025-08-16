/*
Unit-Tests:
go test -v ./backend/filejump/ -remote filejump:

Integration Tests:
go test -v ./fs/operations/ -remote filejump: -timeout=50m
go test -v ./fs/sync/ -remote filejump: -timeout=50m
*/
package filejump_test

import (
	"testing"

	"github.com/rclone/rclone/backend/filejump"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestFileJump:",
		NilObject:  (*filejump.Object)(nil),
	})
}
