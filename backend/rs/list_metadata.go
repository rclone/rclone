package rs

import (
	"context"
	"fmt"
	"time"

	"github.com/rclone/rclone/fs"
)

type mergedEntryVotes struct {
	fileVotes int
	dirVotes  int

	shardFile       []bool
	shardSize       []int64 // particle size when shardFile[i]
	shardHasModTime []bool
	shardModTime    []time.Time
}

func newMergedEntryVotes(n int) *mergedEntryVotes {
	return &mergedEntryVotes{
		shardFile:       make([]bool, n),
		shardSize:       make([]int64, n),
		shardHasModTime: make([]bool, n),
		shardModTime:    make([]time.Time, n),
	}
}

type listObjectMeta struct {
	lowestShard    int
	hasListSize    bool
	listSize       int64
	hasListModTime bool
	listModTime    time.Time
	needFooter     bool
}

func (f *Fs) recordShardFileEntry(ctx context.Context, v *mergedEntryVotes, shard int, obj fs.Object) {
	v.shardFile[shard] = true
	v.shardSize[shard] = obj.Size()
	if f.backends[shard].Precision() != fs.ModTimeNotSupported {
		v.shardModTime[shard] = obj.ModTime(ctx).Truncate(time.Second)
		v.shardHasModTime[shard] = true
	}
}

func lowestListingShard(shardFile []bool) int {
	for i, ok := range shardFile {
		if ok {
			return i
		}
	}
	return -1
}

func resolveListSize(k int, shardFile []bool, shardSize []int64) (int64, bool) {
	dataSizes := make([]int64, k)
	for i := 0; i < k; i++ {
		if !shardFile[i] || shardSize[i] < int64(FooterSize) {
			return 0, false
		}
		dataSizes[i] = shardSize[i]
	}
	return ContentLengthFromDataShardPayloads(dataSizes, k)
}

func resolveListModTime(f *Fs, dir, remote string, shardFile, shardHasModTime []bool, shardModTime []time.Time) (time.Time, bool) {
	lowest := -1
	var pick time.Time
	for i := range shardFile {
		if !shardFile[i] || !shardHasModTime[i] {
			continue
		}
		mt := shardModTime[i]
		if lowest < 0 {
			lowest = i
			pick = mt
			continue
		}
		if pick.Sub(mt).Abs() > time.Second {
			fs.Logf(f, "rs: list %q remote=%q shard mtime skew: shard=%d %s vs shard=%d %s (using lowest index)",
				dir, remote, lowest, pick.Format(time.RFC3339), i, mt.Format(time.RFC3339))
		}
	}
	if lowest < 0 {
		return time.Time{}, false
	}
	return pick, true
}

func (f *Fs) buildListObjectMeta(ctx context.Context, dir, remote string, v *mergedEntryVotes) (listObjectMeta, error) {
	meta := listObjectMeta{lowestShard: lowestListingShard(v.shardFile)}
	if meta.lowestShard < 0 {
		return meta, fmt.Errorf("rs: no shard listed file %q", remote)
	}
	k := f.opt.DataShards
	if size, ok := resolveListSize(k, v.shardFile, v.shardSize); ok {
		meta.hasListSize = true
		meta.listSize = size
	} else {
		meta.needFooter = true
	}
	if mt, ok := resolveListModTime(f, dir, remote, v.shardFile, v.shardHasModTime, v.shardModTime); ok {
		meta.hasListModTime = true
		meta.listModTime = mt
	} else {
		meta.needFooter = true
	}
	_ = ctx
	return meta, nil
}

func (f *Fs) newObjectFromListMetadata(ctx context.Context, remote string, meta listObjectMeta) (*Object, error) {
	o := &Object{
		fs:           f,
		remote:       remote,
		primaryIndex: meta.lowestShard,
		hasListSize:  meta.hasListSize,
		listSize:     meta.listSize,
		hasListModTime: meta.hasListModTime,
		listModTime:  meta.listModTime,
	}
	if meta.needFooter {
		if err := o.ensureFooter(ctx); err != nil {
			return nil, err
		}
	}
	return o, nil
}
