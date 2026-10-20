// Test Crypt filesystem interface
package compress

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/buengese/sgzip"

	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/swift"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

func TestObjectMetadataValidate(t *testing.T) {
	tests := []struct {
		name string
		meta ObjectMetadata
		want bool
	}{
		{
			name: "nil gzip metadata",
			meta: ObjectMetadata{Mode: Gzip},
			want: true,
		},
		{
			name: "zero block size",
			meta: ObjectMetadata{
				Mode: Gzip,
				Size: 1024,
				CompressionMetadataGzip: &sgzip.GzipMetadata{
					BlockSize: 0,
					Size:      1024,
					BlockData: []uint32{0},
				},
			},
			want: true,
		},
		{
			name: "negative block size",
			meta: ObjectMetadata{
				Mode: Gzip,
				Size: 1024,
				CompressionMetadataGzip: &sgzip.GzipMetadata{
					BlockSize: -1,
					Size:      1024,
					BlockData: []uint32{0},
				},
			},
			want: true,
		},
		{
			name: "mismatched sizes",
			meta: ObjectMetadata{
				Mode: Gzip,
				Size: 1024,
				CompressionMetadataGzip: &sgzip.GzipMetadata{
					BlockSize: 512,
					Size:      2048,
					BlockData: []uint32{0, 1},
				},
			},
			want: true,
		},
		{
			name: "short block data",
			meta: ObjectMetadata{
				Mode: Gzip,
				Size: 1025,
				CompressionMetadataGzip: &sgzip.GzipMetadata{
					BlockSize: 512,
					Size:      1025,
					BlockData: []uint32{0, 1},
				},
			},
			want: true,
		},
		{
			name: "valid",
			meta: ObjectMetadata{
				Mode: Gzip,
				Size: 1025,
				CompressionMetadataGzip: &sgzip.GzipMetadata{
					BlockSize: 512,
					Size:      1025,
					BlockData: []uint32{0, 1, 2},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.meta.validate()
			if (err != nil) != test.want {
				t.Fatalf("validate() error = %v, want error: %v", err, test.want)
			}
		})
	}
}

var defaultOpt = fstests.Opt{
	RemoteName: "TestCompress:",
	NilObject:  (*Object)(nil),
	UnimplementableFsMethods: []string{
		"OpenWriterAt",
		"OpenChunkWriter",
		"MergeDirs",
		"DirCacheFlush",
		"PutUnchecked",
		"PutStream",
		"UserInfo",
		"Disconnect",
	},
	TiersToTest:                  []string{"STANDARD", "STANDARD_IA"},
	UnimplementableObjectMethods: []string{},
}

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &defaultOpt)
}

// TestRemoteGzip tests GZIP compression
func TestRemoteGzip(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-compress-test-gzip")
	name := "TestCompressGzip"
	opt := defaultOpt
	opt.RemoteName = name + ":"
	opt.ExtraConfig = []fstests.ExtraConfigItem{
		{Name: name, Key: "type", Value: "compress"},
		{Name: name, Key: "remote", Value: tempdir},
		{Name: name, Key: "mode", Value: "gzip"},
		{Name: name, Key: "level", Value: "-1"},
	}
	opt.QuickTestOK = true
	fstests.Run(t, &opt)
}

// TestRemoteZstd tests ZSTD compression
func TestRemoteZstd(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-compress-test-zstd")
	name := "TestCompressZstd"
	opt := defaultOpt
	opt.RemoteName = name + ":"
	opt.ExtraConfig = []fstests.ExtraConfigItem{
		{Name: name, Key: "type", Value: "compress"},
		{Name: name, Key: "remote", Value: tempdir},
		{Name: name, Key: "mode", Value: "zstd"},
		{Name: name, Key: "level", Value: "2"},
	}
	opt.QuickTestOK = true
	fstests.Run(t, &opt)
}
