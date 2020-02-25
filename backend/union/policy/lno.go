package policy

import (
	"context"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("lno", &Lno{})
}

// Lno stands for least number of objects
// Search category: same as eplno.
// Action category: same as eplno.
// Create category: Pick the drive with the least number of objects.
type Lno struct {
	EpLno
}

// Create category policy, governing the creation of files and directories
func (p *Lno) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.lno(upstreams)
	return []*upstream.Fs{u}, err
}
