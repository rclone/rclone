package rs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rclone/rclone/fs"
)

type mergedEntryVotes struct {
	fileVotes int
	dirVotes  int
}

// List returns directory entries using quorum voting across shard backends.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if len(f.backends) == 0 {
		return nil, fs.ErrorDirNotFound
	}
	votes := make(map[string]*mergedEntryVotes)
	healthy := 0
	notFound := 0
	for i, b := range f.backends {
		entries, err := b.List(ctx, dir)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if errors.Is(err, fs.ErrorDirNotFound) {
				healthy++
				notFound++
				continue
			}
			fs.Logf(f, "rs: list %q shard=%d failed: %v", dir, i, err)
			continue
		}
		healthy++
		for _, e := range entries {
			v := votes[e.Remote()]
			if v == nil {
				v = &mergedEntryVotes{}
				votes[e.Remote()] = v
			}
			if _, ok := e.(fs.Object); ok {
				v.fileVotes++
			} else {
				v.dirVotes++
			}
		}
	}
	if notFound == len(f.backends) {
		return nil, fs.ErrorDirNotFound
	}
	if healthy < f.writeQuorum() {
		return nil, fmt.Errorf("rs: insufficient shard listings for quorum on %q: available=%d required=%d", dir, healthy, f.writeQuorum())
	}
	remotes := make([]string, 0, len(votes))
	for remote := range votes {
		remotes = append(remotes, remote)
	}
	sort.Strings(remotes)

	out := make(fs.DirEntries, 0, len(remotes))
	for _, remote := range remotes {
		v := votes[remote]
		if v.fileVotes > 0 && v.dirVotes > 0 {
			fs.Logf(f, "rs: list %q remote=%q has conflicting types across shards (fileVotes=%d dirVotes=%d)", dir, remote, v.fileVotes, v.dirVotes)
		}
		if v.fileVotes >= f.writeQuorum() {
			o, err := f.NewObject(ctx, remote)
			if err != nil {
				fs.Logf(f, "rs: list %q failed to resolve object %q: %v", dir, remote, err)
				continue
			}
			out = append(out, o)
			continue
		}
		if v.dirVotes >= f.writeQuorum() {
			out = append(out, fs.NewDir(remote, time.Time{}))
		}
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
