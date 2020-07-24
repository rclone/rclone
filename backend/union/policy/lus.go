package policy

import (
	"context"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("lus", &Lus{})
}

// Lus stands for least used space
// Search category: same as eplus.
// Action category: same as eplus.
// Create category: Pick the drive with the least used space.
type Lus struct {
	EpLus
}

// Create category policy, governing the creation of files and directories
func (p *Lus) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.lus(upstreams)
	return []*upstream.Fs{u}, err
}
