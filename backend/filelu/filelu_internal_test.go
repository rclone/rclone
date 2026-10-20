package filelu

import (
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
)

// SetUploadChunkSize changes the configured chunk size, returning the old value.
//
// It is only called by the integration tests while no transfer is in progress.
func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	return old, nil
}

// SetUploadCutoff changes the configured upload cutoff, returning the old value.
//
// It is only called by the integration tests while no transfer is in progress.
func (f *Fs) SetUploadCutoff(cutoff fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.UploadCutoff = f.opt.UploadCutoff, cutoff
	return old, nil
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)
