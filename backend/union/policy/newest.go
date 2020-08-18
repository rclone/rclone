package policy

import (
	"context"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("newest", &Newest{})
}

// Newest policy picks the file / directory with the largest mtime
// It implies the existence of a path
type Newest struct {
	EpAll
}

func (p *Newest) newest(ctx context.Context, upstreams []*upstream.Fs, filePath string) (*upstream.Fs, error) {
	var wg sync.WaitGroup
	ufs := make([]*upstream.Fs, len(upstreams))
	mtimes := make([]time.Time, len(upstreams))
	for i, u := range upstreams {
		wg.Add(1)
		i, u := i, u // Closure
		go func() {
			defer wg.Done()
			rfs := u.RootFs
			remote := path.Join(u.RootPath, filePath)
			if e := findEntry(ctx, rfs, remote); e != nil {
				ufs[i] = u
				mtimes[i] = e.ModTime(ctx)
			}
		}()
	}
	wg.Wait()
	maxMtime := time.Time{}
	var newestFs *upstream.Fs
	for i, u := range ufs {
		if u != nil && mtimes[i].After(maxMtime) {
			maxMtime = mtimes[i]
			newestFs = u
		}
	}
	if newestFs == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return newestFs, nil
}

func (p *Newest) newestEntries(entries []upstream.Entry) (upstream.Entry, error) {
	var wg sync.WaitGroup
	mtimes := make([]time.Time, len(entries))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i, e := range entries {
		wg.Add(1)
		i, e := i, e // Closure
		go func() {
			defer wg.Done()
			mtimes[i] = e.ModTime(ctx)
		}()
	}
	wg.Wait()
	maxMtime := time.Time{}
	var newestEntry upstream.Entry
	for i, t := range mtimes {
		if t.After(maxMtime) {
			maxMtime = t
			newestEntry = entries[i]
		}
	}
	if newestEntry == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return newestEntry, nil
}

// Action category policy, governing the modification of files and directories
func (p *Newest) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterRO(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.newest(ctx, upstreams, path)
	return []*upstream.Fs{u}, err
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *Newest) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterROEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	e, err := p.newestEntries(entries)
	return []upstream.Entry{e}, err
}

// Create category policy, governing the creation of files and directories
func (p *Newest) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.newest(ctx, upstreams, path+"/..")
	return []*upstream.Fs{u}, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *Newest) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterNCEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	e, err := p.newestEntries(entries)
	return []upstream.Entry{e}, err
}

// Search category policy, governing the access to files and directories
func (p *Newest) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.newest(ctx, upstreams, path)
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *Newest) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.newestEntries(entries)
}
