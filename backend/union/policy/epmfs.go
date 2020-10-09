package policy

import (
	"context"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("epmfs", &EpMfs{})
}

// EpMfs stands for existing path, most free space
// Of all the candidates on which the path exists choose the one with the most free space.
type EpMfs struct {
	EpAll
}

func (p *EpMfs) mfs(upstreams []*upstream.Fs) (*upstream.Fs, error) {
	var maxFreeSpace int64
	var mfsupstream *upstream.Fs
	for _, u := range upstreams {
		space, err := u.GetFreeSpace()
		if err != nil {
			fs.LogPrintf(fs.LogLevelNotice, nil,
				"Free Space is not supported for upstream %s, treating as infinite", u.Name())
		}
		if maxFreeSpace < space {
			maxFreeSpace = space
			mfsupstream = u
		}
	}
	if mfsupstream == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return mfsupstream, nil
}

func (p *EpMfs) mfsEntries(entries []upstream.Entry) (upstream.Entry, error) {
	var maxFreeSpace int64
	var mfsEntry upstream.Entry
	for _, e := range entries {
		space, err := e.UpstreamFs().GetFreeSpace()
		if err != nil {
			fs.LogPrintf(fs.LogLevelNotice, nil,
				"Free Space is not supported for upstream %s, treating as infinite", e.UpstreamFs().Name())
		}
		if maxFreeSpace < space {
			maxFreeSpace = space
			mfsEntry = e
		}
	}
	return mfsEntry, nil
}

// Action category policy, governing the modification of files and directories
func (p *EpMfs) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Action(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.mfs(upstreams)
	return []*upstream.Fs{u}, err
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpMfs) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.ActionEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.mfsEntries(entries)
	return []upstream.Entry{e}, err
}

// Create category policy, governing the creation of files and directories
func (p *EpMfs) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Create(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.mfs(upstreams)
	return []*upstream.Fs{u}, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpMfs) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.CreateEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.mfsEntries(entries)
	return []upstream.Entry{e}, err
}

// Search category policy, governing the access to files and directories
func (p *EpMfs) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams, err := p.epall(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return p.mfs(upstreams)
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *EpMfs) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.mfsEntries(entries)
}
