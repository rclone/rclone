package policy

import (
	"context"
	"path"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

func init() {
	registerPolicy("epff", &EpFF{})
}

// EpFF stands for existing path, first found
// Given the order of the candidates, act on the first one found where the relative path exists.
type EpFF struct{}

func (p *EpFF) epff(ctx context.Context, upstreams []*upstream.Fs, filePath string) (*upstream.Fs, error) {
	ch := make(chan *upstream.Fs, len(upstreams))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, u := range upstreams {
		u := u // Closure
		go func() {
			rfs := u.RootFs
			remote := path.Join(u.RootPath, filePath)
			if findEntry(ctx, rfs, remote) == nil {
				u = nil
			}
			ch <- u
		}()
	}
	var u *upstream.Fs
	for range upstreams {
		u = <-ch
		if u != nil {
			break
		}
	}
	if u == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return u, nil
}

// Action category policy, governing the modification of files and directories
func (p *EpFF) Action(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterRO(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.epff(ctx, upstreams, path)
	return []*upstream.Fs{u}, err
}

// ActionEntries is ACTION category policy but receiving a set of candidate entries
func (p *EpFF) ActionEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterROEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return entries[:1], nil
}

// Create category policy, governing the creation of files and directories
func (p *EpFF) Create(ctx context.Context, upstreams []*upstream.Fs, path string) ([]*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	upstreams = filterNC(upstreams)
	if len(upstreams) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	u, err := p.epff(ctx, upstreams, path+"/..")
	return []*upstream.Fs{u}, err
}

// CreateEntries is CREATE category policy but receiving a set of candidate entries
func (p *EpFF) CreateEntries(entries ...upstream.Entry) ([]upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	entries = filterNCEntries(entries)
	if len(entries) == 0 {
		return nil, fs.ErrorPermissionDenied
	}
	return entries[:1], nil
}

// Search category policy, governing the access to files and directories
func (p *EpFF) Search(ctx context.Context, upstreams []*upstream.Fs, path string) (*upstream.Fs, error) {
	if len(upstreams) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return p.epff(ctx, upstreams, path)
}

// SearchEntries is SEARCH category policy but receiving a set of candidate entries
func (p *EpFF) SearchEntries(entries ...upstream.Entry) (upstream.Entry, error) {
	if len(entries) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	return entries[0], nil
}
