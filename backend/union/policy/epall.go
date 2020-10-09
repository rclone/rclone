package policy

import (
	"context"
	"path"
	"sync"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("epall", &EpAll{})
}

// EpAll stands for existing path, all
// Action category: apply to all found.
// Create category: apply to all found.
// Search category: same as epff.
type EpAll struct {
	EpFF
}

func (p *EpAll) epall(ctx context.Context, upstreams []*upstream.Fs, filePath string) ([]*upstream.Fs, error) {
	var wg sync.WaitGroup
	ufs := make([]*upstream.Fs, len(upstreams))
	for i, u := range upstreams {
		wg.Add(1)
		i, u := i, u // Closure
		go func() {
			rfs := u.RootFs
			remote := path.Join(u.RootPath, filePath)
			if findEntry(ctx, rfs, remote) != nil {
				ufs[i] = u
			}
			wg.Done()
		}()
	}
	wg.Wait()
	var results []*upstream.Fs
	for _, f := range ufs {
		if f != nil {
			results = append(results, f)
		}
	}
	if len(results) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return results, nil
}

// Action category policy, governing the modification of files and directories
func (p *EpAll) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterRO(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return p.epall(ctx, upstreams, path)
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpAll) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterROEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return entries, nil
}

// Create category policy, governing the creation of files and directories
func (p *EpAll) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	upstreams, err := p.epall(ctx, upstreams, path+"/..")
	return upstreams, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpAll) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterNCEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return entries, nil
}
