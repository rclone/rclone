package policy

import (
	"context"
	"math"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("eplus", &EpLus{})
}

// EpLus stands for existing path, least used space
// Of all the candidates on which the path exists choose the one with the least used space.
type EpLus struct {
	EpAll
}

func (p *EpLus) lus(upstreams []*upstream.Fs) (*upstream.Fs, error) {
	var minUsedSpace int64 = math.MaxInt64
	var lusupstream *upstream.Fs
	for _, u := range upstreams {
		space, err := u.GetUsedSpace()
		if err != nil {
			fs.LogPrintf(fs.LogLevelNotice, nil,
				"Used Space is not supported for upstream %s, treating as 0", u.Name())
		}
		if space < minUsedSpace {
			minUsedSpace = space
			lusupstream = u
		}
	}
	if lusupstream == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return lusupstream, nil
}

func (p *EpLus) lusEntries(entries []upstream.Entry) (upstream.Entry, error) {
	var minUsedSpace int64
	var lusEntry upstream.Entry
	for _, e := range entries {
		space, err := e.UpstreamFs().GetFreeSpace()
		if err != nil {
			fs.LogPrintf(fs.LogLevelNotice, nil,
				"Used Space is not supported for upstream %s, treating as 0", e.UpstreamFs().Name())
		}
		if space < minUsedSpace {
			minUsedSpace = space
			lusEntry = e
		}
	}
	return lusEntry, nil
}

// Action category policy, governing the modification of files and directories
func (p *EpLus) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Action(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.lus(upstreams)
	return []*upstream.Fs{u}, err
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpLus) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.ActionEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.lusEntries(entries)
	return []upstream.Entry{e}, err
}

// Create category policy, governing the creation of files and directories
func (p *EpLus) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Create(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.lus(upstreams)
	return []*upstream.Fs{u}, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpLus) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.CreateEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.lusEntries(entries)
	return []upstream.Entry{e}, err
}

// Search category policy, governing the access to files and directories
func (p *EpLus) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams, err := p.epall(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return p.lus(upstreams)
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *EpLus) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.lusEntries(entries)
}
