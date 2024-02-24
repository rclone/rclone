package hasher_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/backend/hasher"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/kv"

	_ "github.com/rclone/rclone/backend/all" // for integration tests
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if !kv.Supported() {
		t.Skip("hasher is not supported on this OS")
	}
	opt := fstests.Opt{
		RemoteName: *fstest.RemoteName,
		NilObject:  (*hasher.Object)(nil),
		UnimplementableFsMethods: []string{
			"OpenWriterAt",
			"OpenChunkWriter",
		},
		UnimplementableObjectMethods: []string{},
	}
	if *fstest.RemoteName == "" {
		tempDir := filepath.Join(os.TempDir(), "rclone-hasher-test")
		opt.ExtraConfig = []fstests.ExtraConfigItem{
			{Name: "TestHasher", Key: "type", Value: "hasher"},
			{Name: "TestHasher", Key: "remote", Value: tempDir},
		}
		opt.RemoteName = "TestHasher:"
		opt.QuickTestOK = true
	}
	fstests.Run(t, &opt)
	// test again with MaxAge = 0
	if *fstest.RemoteName == "" {
		opt.ExtraConfig = append(opt.ExtraConfig, fstests.ExtraConfigItem{Name: "TestHasher", Key: "max_age", Value: "0"})
		fstests.Run(t, &opt)
	}
}
