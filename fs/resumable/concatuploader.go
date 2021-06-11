package resumable

import (
	"context"
	"io"

	"github.com/rclone/rclone/fs"
)

type concatUploader struct {
	fragmentUploader
	fs ConcatenatorFs
}

// ConcatenatorFs represents a fs.Fs that implements the fs.Concatenator interface
type ConcatenatorFs interface {
	fs.Fs
	fs.Concatenator
}

// NewConcatUploader returns an fs.Uploader that is able to receive upload chunks which are then concatenated upon completion
func NewConcatUploader(remote, uploadDir string, f ConcatenatorFs, ctx context.Context) fs.Uploader {
	self := &concatUploader{fragmentUploader{nil, remote, uploadDir, f, ctx, []io.Reader{}, 0, -1, 0, nil}, f}
	self.self = self
	return self
}

func (u *concatUploader) finish(fragments fs.Objects) error {
	_, err := u.fs.Concat(u.ctx, fragments, u.remote)
	return err
}

// Check interfaces
var (
	_ FragmentUploader = (*concatUploader)(nil)
	_ fs.Uploader      = (*concatUploader)(nil)
)
