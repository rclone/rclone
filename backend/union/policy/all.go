package policy

import (
	"context"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("all", &All{})
}

// All policy behaves the same as EpAll except for the CREATE category
// Action category: same as epall.
// Create category: apply to all branches.
// Search category: same as epall.
type All struct {
	EpAll
}

// Create category policy, governing the creation of files and directories
func (p *All) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return upstreams, nil
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *All) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterNCEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return entries, nil
}
