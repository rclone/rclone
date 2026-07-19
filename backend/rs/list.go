package rs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

type listShardResult struct {
	entries fs.DirEntries
	err     error
}

// List returns directory entries using quorum voting across shard backends.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if len(f.backends) == 0 {
		return nil, fs.ErrorDirNotFound
	}
	n := len(f.backends)
	shardOut := make([]listShardResult, n)
	g, gctx := errgroup.WithContext(ctx)
	for i := range f.backends {
		i := i
		g.Go(func() error {
			entries, err := f.backends[i].List(gctx, dir)
			shardOut[i].entries = entries
			shardOut[i].err = err
			if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
				return err
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	votes := make(map[string]*mergedEntryVotes)
	healthy := 0
	notFound := 0
	for i := 0; i < n; i++ {
		entries, err := shardOut[i].entries, shardOut[i].err
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
			remote := e.Remote()
			v := votes[remote]
			if v == nil {
				v = newMergedEntryVotes(n)
				votes[remote] = v
			}
			if obj, ok := e.(fs.Object); ok {
				v.fileVotes++
				f.recordShardFileEntry(ctx, v, i, obj)
			} else {
				v.dirVotes++
			}
		}
	}
	if notFound == len(f.backends) {
		return nil, fs.ErrorDirNotFound
	}
	if healthy < f.readQuorum() {
		return nil, fmt.Errorf("rs: insufficient shard listings for quorum on %q: available=%d required=%d", dir, healthy, f.readQuorum())
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
		if v.fileVotes > 0 && v.fileVotes < f.readQuorum() {
			fs.Logf(f, "rs: list %q omitting broken object %q: fileVotes=%d required=%d", dir, remote, v.fileVotes, f.readQuorum())
			continue
		}
		if v.fileVotes >= f.readQuorum() {
			meta, err := f.buildListObjectMeta(ctx, dir, remote, v)
			if err != nil {
				fs.Logf(f, "rs: list %q failed to resolve object %q: %v", dir, remote, err)
				continue
			}
			o, err := f.newObjectFromListMetadata(ctx, remote, meta)
			if err != nil {
				fs.Logf(f, "rs: list %q failed to resolve object %q: %v", dir, remote, err)
				continue
			}
			out = append(out, o)
			continue
		}
		if v.dirVotes >= f.readQuorum() {
			out = append(out, fs.NewDir(remote, time.Time{}))
		}
	}
	return out, nil
}

// NewObject returns the logical object if any shard has a particle for remote.
// Uses shard sizes and ModTimes when sufficient; reads one footer only as fallback.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	n := len(f.backends)
	type shardHit struct {
		ok   bool
		size int64
		hasMT bool
		mt   time.Time
	}
	hits := make([]shardHit, n)
	g, gctx := errgroup.WithContext(ctx)
	for i := range f.backends {
		i := i
		g.Go(func() error {
			obj, err := f.backends[i].NewObject(gctx, remote)
			if err != nil {
				return nil
			}
			hits[i].ok = true
			hits[i].size = obj.Size()
			if f.backends[i].Precision() != fs.ModTimeNotSupported {
				hits[i].hasMT = true
				hits[i].mt = obj.ModTime(gctx).Truncate(time.Second)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	shardFile := make([]bool, n)
	shardSize := make([]int64, n)
	shardHasModTime := make([]bool, n)
	shardModTime := make([]time.Time, n)
	for i := range hits {
		if hits[i].ok {
			shardFile[i] = true
			shardSize[i] = hits[i].size
			shardHasModTime[i] = hits[i].hasMT
			shardModTime[i] = hits[i].mt
		}
	}

	lowest := lowestListingShard(shardFile)
	if lowest < 0 {
		return nil, fs.ErrorObjectNotFound
	}

	k := f.opt.DataShards
	listSize, hasListSize := resolveListSize(k, shardFile, shardSize)
	listModTime, hasListModTime := resolveListModTime(f, "", remote, shardFile, shardHasModTime, shardModTime)

	o := &Object{
		fs:             f,
		remote:         remote,
		primaryIndex:   lowest,
		hasListSize:    hasListSize,
		listSize:       listSize,
		hasListModTime: hasListModTime,
		listModTime:    listModTime,
	}

	if hasListSize && hasListModTime {
		return o, nil
	}

	for i := 0; i < n; i++ {
		if !hits[i].ok || hits[i].size < int64(FooterSize) {
			continue
		}
		obj, err := f.backends[i].NewObject(ctx, remote)
		if err != nil {
			continue
		}
		ft, err := readFooterFromParticle(ctx, obj)
		if err != nil {
			continue
		}
		o.primaryIndex = i
		o.footer = ft
		return o, nil
	}

	if hasListSize {
		return o, nil
	}
	return nil, fs.ErrorObjectNotFound
}
