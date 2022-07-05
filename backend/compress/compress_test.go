// Test Crypt filesystem interface
package compress

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/swift"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	opt := fstests.Opt{
		RemoteName: *fstest.RemoteName,
		NilObject:  (*Object)(nil),
		UnimplementableFsMethods: []string{
			"OpenWriterAt",
			"MergeDirs",
			"DirCacheFlush",
			"PutUnchecked",
			"PutStream",
			"UserInfo",
			"Disconnect",
		},
		TiersToTest:                  []string{"STANDARD", "STANDARD_IA"},
		UnimplementableObjectMethods: []string{}}
	fstests.Run(t, &opt)
}

// TestRemoteGzip tests GZIP compression
func TestRemoteGzip(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-compress-test-gzip")
	name := "TestCompressGzip"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*Object)(nil),
		UnimplementableFsMethods: []string{
			"OpenWriterAt",
			"MergeDirs",
			"DirCacheFlush",
			"PutUnchecked",
			"PutStream",
			"UserInfo",
			"Disconnect",
		},
		UnimplementableObjectMethods: []string{
			"GetTier",
			"SetTier",
		},
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "compress"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "compression_mode", Value: "gzip"},
		},
		QuickTestOK: true,
	})
}
