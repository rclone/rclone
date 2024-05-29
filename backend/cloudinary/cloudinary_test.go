// Test Cloudinary filesystem interface

package cloudinary_test

import (
	"testing"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/backend/cloudinary"
)

func TestInit(t *testing.T) {
	fstests.Init(t)
}

func TestConfig(t *testing.T) {
	opt := fs.ConfigFileSections["cloudinary"]
	if opt == nil {
		t.Fatalf("Configuration for cloudinary backend not found")
	}
	if opt.Get("type") != "cloudinary" {
		t.Fatalf("Incorrect backend type %v", opt.Get("type"))
	}
}

func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestCloudinary:",
		NilObject:  (*fs.Object)(nil),
	})
}

func TestCustomCloudinaryFeatures(t *testing.T) {
	r, err := fs.NewFs("TestCloudinary:")
	if err != nil {
		t.Fatalf("Failed to create new fs: %v", err)
	}

	obj, err := r.NewObject("path/to/object")
	if err != nil {
		t.Fatalf("Failed to create new object: %v", err)
	}

	md5sum, err := obj.Hash(fs.HashMD5)
	if err != nil {
		t.Fatalf("Failed to get MD5 hash: %v", err)
	}

	if md5sum != "expected_md5_hash" {
		t.Errorf("Unexpected MD5 hash: %v", md5sum)
	}
}

func TestCleanup(t *testing.T) {
	r, err := fs.NewFs("TestCloudinary:")
	if err != nil {
		t.Fatalf("Failed to create new fs: %v", err)
	}

	err = r.Purge()
	if err != nil {
		t.Fatalf("Failed to purge: %v", err)
	}
}
