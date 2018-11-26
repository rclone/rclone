// Test GoogleCloudStorage filesystem interface

// +build go1.9

package googlecloudstorage_test

import (
	"testing"

	"github.com/ncw/rclone/backend/googlecloudstorage"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestGoogleCloudStorage:",
		NilObject:  (*googlecloudstorage.Object)(nil),
	})
}
