// Test Crypt filesystem interface
package press

import (
	"os"
	"path/filepath"
	"testing"

//	"github.com/ncw/rclone/backend/press"
//	_ "github.com/ncw/rclone/backend/drive" // for integration tests
	_ "github.com/ncw/rclone/backend/local"
//	_ "github.com/ncw/rclone/backend/swift" // for integration tests
	"github.com/ncw/rclone/fstest"
	"github.com/ncw/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: *fstest.RemoteName,
		NilObject:  (*Object)(nil),
	})
}

// TestRemoteLz4 tests LZ4 compression
func TestRemoteLz4(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-press-test-lz4")
	name := "TestPressLz4"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "press"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "compression_mode", Value: "lz4"},
		},
	})
}

// TestRemoteGzip tests GZIP compression
func TestRemoteGzip(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-press-test-gzip")
	name := "TestPressGzip"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "press"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "compression_mode", Value: "gzip-min"},
		},
	})
}

// TestRemoteXZ tests XZ compression
func TestRemoteXZ(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-press-test-xz")
	name := "TestPressXZ"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*Object)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "press"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "compression_mode", Value: "xz-min"},
		},
	})
}