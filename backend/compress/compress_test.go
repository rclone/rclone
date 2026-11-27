// Test Crypt filesystem interface
package compress

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buengese/sgzip"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/swift"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
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
		"ServerSideFetchURL",
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
	}
	opt.QuickTestOK = true
	fstests.Run(t, &opt)
}

// newTestCompressFs makes a compress Fs wrapping an in-memory remote.
func newTestCompressFs(t *testing.T, name, mode, level string) (*Fs, error) {
	info, err := fs.Find("compress")
	require.NoError(t, err)
	config := configmap.Simple{
		"remote": ":memory:" + name,
		"mode":   mode,
	}
	if level != "" {
		config["level"] = level
	}
	m := fs.ConfigMap("", info.Options, "", config)
	f, err := NewFs(context.Background(), "compress", "", m)
	if err != nil {
		return nil, err
	}
	return f.(*Fs), nil
}

// TestCompressionLevel checks that the default level is applied for
// both modes and that levels outside the supported range are rejected.
func TestCompressionLevel(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		level   string
		want    int
		wantErr bool
	}{
		{name: "gzip default", mode: "gzip", want: sgzip.DefaultCompression},
		{name: "zstd default", mode: "zstd", want: defaultZstdLevel},
		{name: "zstd off", mode: "zstd", level: "0", want: 0},
		{name: "zstd fastest", mode: "zstd", level: "1", want: int(zstd.SpeedFastest)},
		{name: "zstd best", mode: "zstd", level: "4", want: int(zstd.SpeedBestCompression)},
		{name: "zstd too high", mode: "zstd", level: "5", wantErr: true},
		{name: "zstd negative", mode: "zstd", level: "-2", wantErr: true},
		{name: "gzip too high", mode: "gzip", level: "10", wantErr: true},
		{name: "gzip too low", mode: "gzip", level: "-3", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f, err := newTestCompressFs(t, strings.ReplaceAll(test.name, " ", "-"), test.mode, test.level)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, f.opt.CompressionLevel)
		})
	}
}

// putAndRead puts data through a compress Fs and reads it back,
// returning the stored data object.
func putAndRead(t *testing.T, f *Fs, content []byte) *Object {
	ctx := context.Background()
	src := object.NewStaticObjectInfo("file.txt", time.Now(), int64(len(content)), true, nil, f)
	o, err := f.Put(ctx, bytes.NewReader(content), src)
	require.NoError(t, err)
	require.Equal(t, int64(len(content)), o.Size())

	newO, err := f.NewObject(ctx, "file.txt")
	require.NoError(t, err)
	rc, err := newO.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, content, got)
	return o.(*Object)
}

// TestDefaultLevelCompresses checks that leaving the level unset still
// compresses data with both modes.
func TestDefaultLevelCompresses(t *testing.T) {
	content := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog "), 200)
	for _, test := range []struct {
		mode   string
		suffix string
	}{
		{mode: "gzip", suffix: gzFileExt},
		{mode: "zstd", suffix: zstdFileExt},
	} {
		t.Run(test.mode, func(t *testing.T) {
			f, err := newTestCompressFs(t, "default-level-"+test.mode, test.mode, "")
			require.NoError(t, err)
			o := putAndRead(t, f, content)
			assert.Less(t, o.Object.Size(), int64(len(content)))
			assert.True(t, strings.HasSuffix(o.Object.Remote(), test.suffix), o.Object.Remote())
		})
	}
}

// TestZstdLevelOff checks that level 0 stores the file uncompressed.
func TestZstdLevelOff(t *testing.T) {
	content := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog "), 200)
	f, err := newTestCompressFs(t, "zstd-level-off", "zstd", "0")
	require.NoError(t, err)
	o := putAndRead(t, f, content)
	assert.Equal(t, Uncompressed, o.meta.Mode)
	assert.Equal(t, int64(len(content)), o.Object.Size())
	assert.True(t, strings.HasSuffix(o.Object.Remote(), uncompressedFileExt), o.Object.Remote())
}
