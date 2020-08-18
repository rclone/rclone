package policy

import (
	"context"
	"math/rand"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("rand", &Rand{})
}

// Rand stands for random
// Calls all and then randomizes. Returns one candidate.
type Rand struct {
	All
}

func (p *Rand) rand(upstreams []*upstream.Fs) *upstream.Fs {
	return upstreams[rand.Intn(len(upstreams))]
}

func (p *Rand) randEntries(entries []upstream.Entry) upstream.Entry {
	return entries[rand.Intn(len(entries))]
}

// Action category policy, governing the modification of files and directories
func (p *Rand) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.All.Action(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return []*upstream.Fs{p.rand(upstreams)}, nil
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *Rand) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.All.ActionEntries(entries...)
	if err != nil {
		return nil, err
	}
	return []upstream.Entry{p.randEntries(entries)}, nil
}

// Create category policy, governing the creation of files and directories
func (p *Rand) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.All.Create(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return []*upstream.Fs{p.rand(upstreams)}, nil
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *Rand) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.All.CreateEntries(entries...)
	if err != nil {
		return nil, err
	}
	return []upstream.Entry{p.randEntries(entries)}, nil
}

// Search category policy, governing the access to files and directories
func (p *Rand) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams, err := p.epall(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return p.rand(upstreams), nil
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *Rand) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.randEntries(entries), nil
}
