package policy

import (
	"context"
	"math/rand"
	"time"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("eprand", &EpRand{})
}

// EpRand stands for existing path, random
// Calls epall and then randomizes. Returns one candidate.
type EpRand struct {
	EpAll
}

func (p *EpRand) rand(upstreams []*upstream.Fs) *upstream.Fs {
	rand.Seed(time.Now().Unix())
	return upstreams[rand.Intn(len(upstreams))]
}

func (p *EpRand) randEntries(entries []upstream.Entry) upstream.Entry {
	rand.Seed(time.Now().Unix())
	return entries[rand.Intn(len(entries))]
}

// Action category policy, governing the modification of files and directories
func (p *EpRand) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Action(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return []*upstream.Fs{p.rand(upstreams)}, nil
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpRand) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.ActionEntries(entries...)
	if err != nil {
		return nil, err
	}
	return []upstream.Entry{p.randEntries(entries)}, nil
}

// Create category policy, governing the creation of files and directories
func (p *EpRand) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Create(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return []*upstream.Fs{p.rand(upstreams)}, nil
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpRand) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.CreateEntries(entries...)
	if err != nil {
		return nil, err
	}
	return []upstream.Entry{p.randEntries(entries)}, nil
}

// Search category policy, governing the access to files and directories
func (p *EpRand) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
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
func (p *EpRand) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.randEntries(entries), nil
}
