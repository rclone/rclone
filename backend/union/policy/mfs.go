package policy

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/backend/union/upstream"
)

func init(){
	registerPolicy("mfs", &Mfs{})
}

// Mfs stands for most free space
// Of all the candidates on which the path exists choose the one with the most free space.
type Mfs struct {
	EpMfs
}

// Create category policy, governing the creation of files and directories
func (p *Mfs) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	r, err := p.mfs(upstreams)
	return []*upstream.Fs{r}, err
}
