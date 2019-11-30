package policy

import (
	"context"
	"sync"
	
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/backend/union/upstream"
)

func init(){
	registerPolicy("epall", &EpAll{})
}

// EpAll stands for existing path, All
// Action category: apply to all found.
// Create category: apply to all found.
// Search category: same as epff.
type EpAll struct {
	EpFF
}

func (p *EpAll) epall(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	var wg sync.WaitGroup
	ufs := make([]*upstream.Fs, len(upstreams))
	for i, u := range upstreams {
		wg.Add(1)
		i, u := i, u // Closure
		go func() {
			if exists(ctx, u, path) {
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

// Create category policy, governing the creation of files and directories
func (p *EpAll) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	upstreams, err := p.epall(ctx, upstreams, path)
	return upstreams, err
}