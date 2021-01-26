// Test OneDrive filesystem interface
package onedrive

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestOneDrive:",
		NilObject:  (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			CeilChunkSize: fstests.NextMultipleOf(chunkSizeMultiple),
		},
	})
}

// TestIntegrationCn runs integration tests against the remote
func TestIntegrationCn(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("skipping as -remote is set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestOneDriveCn:",
		NilObject:  (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			CeilChunkSize: fstests.NextMultipleOf(chunkSizeMultiple),
		},
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

var _ fstests.SetUploadChunkSizer = (*Fs)(nil)
