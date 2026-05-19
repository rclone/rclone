// Build for kdrive for unsupported platforms to stop go complaining on Plan9

//go:build plan9 || js

package kdrive

import (
	"context"
	"errors"
	"io"

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/fs"
)

// uploadSession implements fs.ChunkWriter for kdrive multipart uploads
type uploadSession struct {
	f          *Fs
	parentID   string
	fileName   string
	token      string
	uploadURL  string
	fileInfo   *api.Item
	chunkCount int
	hash       string // Hash from the last chunk upload
}

// UploadMultipart does a generic multipart upload from src using f as newChunkWriter.
// It returns the chunkWriter used in case the caller needs to extract any private info from it.
func (f *Fs) UploadMultipart(ctx context.Context, src fs.ObjectInfo, in io.Reader, opt []fs.OpenOption) (chunkWriterOut fs.ChunkWriter, err error) {
	return nil, errors.New("kdrive multipart upload not supported on this platform!")
}

// WriteChunk uploads a single chunk
func (u *uploadSession) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (bytesWritten int64, err error) {
	return 0, errors.New("kdrive multipart upload not supported on this platform!")
}

// Close finalizes the upload session and returns the created file info
func (u *uploadSession) Close(ctx context.Context) error {
	return errors.New("kdrive multipart upload not supported on this platform!")
}

// Abort the upload session
func (u *uploadSession) Abort(ctx context.Context) error {
	return errors.New("kdrive multipart upload not supported on this platform!")
}

