package resumable

import (
	"context"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"time"
)

type uploadSrcInfo struct {
}

func (u *uploadSrcInfo) Name() string {
	return "Upload"
}

func (u *uploadSrcInfo) Root() string {
	return ""
}

func (u *uploadSrcInfo) String() string {
	return "Upload from non addressable source"
}

func (u *uploadSrcInfo) Precision() time.Duration {
	return time.Second
}

func (u *uploadSrcInfo) Hashes() hash.Set {
	return 0
}

func (u *uploadSrcInfo) Features() *fs.Features {
	return nil
}

type uploadSrcObjectInfo struct {
	size   int64
	remote string
}

func (u *uploadSrcObjectInfo) String() string {
	if u == nil {
		return "<nil>"
	}
	return u.remote
}

func (u *uploadSrcObjectInfo) Remote() string {
	return u.remote
}

func (u *uploadSrcObjectInfo) ModTime(context.Context) time.Time {
	return time.Now()
}

func (u *uploadSrcObjectInfo) Size() int64 {
	return u.size
}

func (u *uploadSrcObjectInfo) Fs() fs.Info {
	return &uploadSrcInfo{}
}

func (u *uploadSrcObjectInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

func (u *uploadSrcObjectInfo) Storable() bool {
	return false
}
