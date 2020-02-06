package resumable

import (
	"context"
	"github.com/rclone/rclone/fs"
	"io"
)

type concatUploader struct {
	fragmentUploader
	fs ConcatenatorFs
}

type ConcatenatorFs interface {
	fs.Fs
	fs.Concatenator
}

func NewConcatUploader(remote, uploadDir string, f ConcatenatorFs, ctx context.Context) fs.Uploader {
	self := &concatUploader{fragmentUploader{nil, remote, uploadDir, f, ctx, []io.Reader{}, 0, -1}, f}
	self.self = self
	return self
}

func (u *concatUploader) finish(fragments fs.Objects) (err error) {
	_, err = u.fs.Concat(u.ctx, fragments, u.remote)
	return
}

// Check interfaces
var (
	_ FragmentUploader = (*concatUploader)(nil)
	_ fs.Uploader      = (*concatUploader)(nil)
)
