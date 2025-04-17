package adrive

import (
	"context"
	"io"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/tickstep/aliyunpan-api/aliyunpan"
)

// upload does a single non-multipart upload
//
// This is recommended for less than 50 MiB of content
func (o *Object) upload(ctx context.Context, in io.Reader, leaf, directoryID string, modTime time.Time, options ...fs.OpenOption) (err error) {
	// TODO
	return o.setMetaData(&aliyunpan.FileEntity{})
}

// uploadMultipart uploads a file using multipart upload
func (o *Object) uploadMultipart(ctx context.Context, in io.Reader, leaf, directoryID string, size int64, modTime time.Time, options ...fs.OpenOption) (err error) {
	// TODO
	return o.setMetaData(&aliyunpan.FileEntity{})
}
