// Test Cloudinary filesystem interface

package cloudinary_test

import (
	"testing"

	"github.com/rclone/rclone/backend/cloudinary"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestCloudinary:",
		NilObject:  (*cloudinary.Object)(nil),
	})
}

func TestIntegration2(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	name := "TestCloudinary"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*cloudinary.Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "directory_markers", Value: "true"},
		},
	})
}
