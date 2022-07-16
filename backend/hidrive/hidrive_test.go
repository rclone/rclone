// Test HiDrive filesystem interface
package hidrive

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
)

// TestIntegration runs integration tests against the remote.
func TestIntegration(t *testing.T) {
	name := "TestHiDrive"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize:       1,
			MaxChunkSize:       MaximumUploadBytes,
			CeilChunkSize:      nil,
			NeedMultipleChunks: false,
		},
	})
}

// Change the configured UploadChunkSize.
// Will only be called while no transfer is in progress.
func (f *Fs) SetUploadChunkSize(chunksize fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.UploadChunkSize = f.opt.UploadChunkSize, chunksize
	return old, nil
}

// Change the configured UploadCutoff.
// Will only be called while no transfer is in progress.
func (f *Fs) SetUploadCutoff(cutoff fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.UploadCutoff = f.opt.UploadCutoff, cutoff
	return old, nil
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)
