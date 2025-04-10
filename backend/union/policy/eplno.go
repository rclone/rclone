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

func (p *EpLno) getUpstreamObjectCount(ctx context.Context, upstream *upstream.Fs) int64 {
	numObj, err := upstream.GetNumObjects()
	if err != nil {
		uName := upstream.Name()
		fs.LogPrintf(fs.LogLevelNotice, nil,
			"Number of Objects is not supported for upstream %s, falling back to listing (this may be slower)...", uName)
		listing, err := upstream.List(ctx, "")
		if err != nil {
			fs.LogPrintf(fs.LogLevelError, nil, "Listing failed, treating as 0: %v", err)
			return 0
		}
		numObj = int64(len(listing))
		fs.LogPrintf(fs.LogLevelDebug, nil, "Counted %d objects for upstream %s by listing", numObj, uName)
	}
	return numObj
}

func (p *EpLno) lno(ctx context.Context, upstreams []*upstream.Fs) (*upstream.Fs, error) {
	var minNumObj int64 = math.MaxInt64
	var lnoUpstream *upstream.Fs
	for _, u := range upstreams {
		numObj := p.getUpstreamObjectCount(ctx, u)
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

func (p *EpLno) lnoEntries(ctx context.Context, entries []upstream.Entry) (upstream.Entry, error) {
	var minNumObj int64 = math.MaxInt64
	var lnoEntry upstream.Entry
	for _, e := range entries {
		numObj := p.getUpstreamObjectCount(ctx, e.UpstreamFs())
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
	u, err := p.lno(ctx, upstreams)
	return []*upstream.Fs{u}, err
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpLno) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.ActionEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.lnoEntries(context.Background(), entries)
	return []upstream.Entry{e}, err
}

// Create category policy, governing the creation of files and directories
func (p *EpLno) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	upstreams, err := p.EpAll.Create(ctx, upstreams, path)
	if err != nil {
		return nil, err
	}
	u, err := p.lno(ctx, upstreams)
	return []*upstream.Fs{u}, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpLno) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	entries, err := p.EpAll.CreateEntries(entries...)
	if err != nil {
		return nil, err
	}
	e, err := p.lnoEntries(context.Background(), entries)
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
	return p.lno(ctx, upstreams)
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *EpLno) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.lnoEntries(context.Background(), entries)
}
