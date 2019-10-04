// Test Webdav filesystem interface
package webdav_test

import (
	"testing"

	"github.com/rclone/rclone/backend/webdav"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestWebdavNexcloud:",
		NilObject:  (*webdav.Object)(nil),
	})
}

// TestIntegration runs integration tests against the remote
func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestWebdavOwncloud:",
		NilObject:  (*webdav.Object)(nil),
	})
}

// TestIntegration runs integration tests against the remote
func TestIntegration3(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestWebdavRclone:",
		NilObject:  (*webdav.Object)(nil),
	})
}
