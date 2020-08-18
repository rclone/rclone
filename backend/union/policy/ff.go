package policy

import (
	"context"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("ff", &FF{})
}

// FF stands for first found
// Search category: same as epff.
// Action category: same as epff.
// Create category: Given the order of the candiates, act on the first one found.
type FF struct {
	EpFF
}

// Create category policy, governing the creation of files and directories
func (p *FF) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return upstreams, fs.ErrorPermissionDenied
	}
	return upstreams[:1], nil
}
