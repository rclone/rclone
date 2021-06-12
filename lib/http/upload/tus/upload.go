package tus

import (
	"context"
	"github.com/rclone/rclone/fs"
	tus "github.com/tus/tusd/pkg/handler"
	"io"
)

type upload struct {
	object   fs.Object
	fileInfo *tus.FileInfo
}

func (u *upload) NewLock() (tus.Lock, error) {
	// TODO does tus need to manage locks? PATCH makes some decisions between a get and write that may need to be sync
	panic("implement me")
}

func (u *upload) WriteChunk(ctx context.Context, offset int64, src io.Reader) (int64, error) {
	panic("implement me")
}

func (u *upload) GetInfo(ctx context.Context) (tus.FileInfo, error) {
	if u.fileInfo != nil {
		return *u.fileInfo, nil
	}
	panic("implement me")
}

func (u *upload) GetReader(ctx context.Context) (io.Reader, error) {
	panic("implement me")
}

func (u *upload) DeclareLength(ctx context.Context, length int64) error {
	panic("implement me")
}

func (u *upload) ConcatUploads(ctx context.Context, partialUploads []tus.Upload) error {
	panic("implement me")
}

func (u *upload) FinishUpload(ctx context.Context) error {
	panic("implement me")
}

func (u *upload) Terminate(ctx context.Context) error {
	panic("implement me")
}
