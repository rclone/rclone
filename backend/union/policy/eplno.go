package policy

import (
	"context"
	"math"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("eplno", &EpLno{})
}

// EpLno stands for existing path, least number of objects
// Of all the candidates on which the path exists choose the one with the least number of objects
type EpLno struct {
	EpAll
}

func (p *EpLno) lno(upstreams []*upstream.Fs) (*upstream.Fs, error) {
	numObjs := usages(len(upstreams), func(i int) (int64, error) {
		return upstreams[i].GetNumObjects()
	})
	var minNumObj int64 = math.MaxInt64
	var lnoUpstream *upstream.Fs
	for i, u := range upstreams {
		numObj := numObjs[i]
		if minNumObj > numObj {
			minNumObj = numObj
			lnoUpstream = u
		}
	}
	if lnoUpstream == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return lnoUpstream, nil
}

func (p *EpLno) lnoEntries(entries []upstream.Entry) (upstream.Entry, error) {
	numObjs := usages(len(entries), func(i int) (int64, error) {
		return entries[i].UpstreamFs().GetNumObjects()
	})
	var minNumObj int64 = math.MaxInt64
	var lnoEntry upstream.Entry
	for i, e := range entries {
		numObj := numObjs[i]
		if minNumObj > numObj {
			minNumObj = numObj
			lnoEntry = e
		}
	}
	return lnoEntry, nil
}

// Action category policy, governing the modification of files and directories
func (p *EpLno) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Action(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.lno(upstreams)
	return []*upstream.Fs{u}, err
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpLno) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.ActionEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.lnoEntries(entries)
	return []upstream.Entry{e}, err
}

// Create category policy, governing the creation of files and directories
func (p *EpLno) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Create(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.lno(upstreams)
	return []*upstream.Fs{u}, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpLno) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.CreateEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.lnoEntries(entries)
	return []upstream.Entry{e}, err
}

// Search category policy, governing the access to files and directories
func (p *EpLno) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams, err := p.epall(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	return p.lno(upstreams)
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *EpLno) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.lnoEntries(entries)
}
