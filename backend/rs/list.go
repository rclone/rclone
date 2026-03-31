package rs

import (
	"context"

	"github.com/rclone/rclone/fs"
)

// List returns directory entries using shard 0 as the namespace, resolving RS objects via NewObject.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	// v1: list from shard 0 namespace and resolve metadata lazily in NewObject/Open.
	if len(f.backends) == 0 {
		return nil, fs.ErrorDirNotFound
	}
	entries, err := f.backends[0].List(ctx, dir)
	if err != nil {
		return nil, err
	}
	out := make(fs.DirEntries, 0, len(entries))
	for _, e := range entries {
		if obj, ok := e.(fs.Object); ok {
			o, err := f.NewObject(ctx, obj.Remote())
			if err != nil {
				continue
			}
			out = append(out, o)
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// NewObject returns the logical object if any shard has a valid particle for remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	for i, b := range f.backends {
		obj, err := b.NewObject(ctx, remote)
		if err != nil {
			continue
		}
		ft, err := readFooterFromParticle(ctx, obj)
		if err != nil {
			continue
		}
		return &Object{fs: f, remote: remote, footer: ft, primaryIndex: i}, nil
	}
	return nil, fs.ErrorObjectNotFound
}
