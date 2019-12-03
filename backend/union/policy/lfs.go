package policy

import (
	"context"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("lfs", &Lfs{})
}

// Lfs stands for least free space
// Search category: same as eplfs.
// Action category: same as eplfs.
// Create category: Pick the drive with the least free space.
type Lfs struct {
	EpLfs
}

// Create category policy, governing the creation of files and directories
func (p *Lfs) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.lfs(upstreams)
	return []*upstream.Fs{u}, err
}
