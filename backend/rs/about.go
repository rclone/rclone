package rs

import (
	"context"
	"fmt"
	"math"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// About returns derived logical quota: capacity is gated by the fullest shard (weakest link).
// Each shard stores ~L/k payload bytes for L logical bytes, so logical free ≈ k × min(shard free).
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	k := f.opt.DataShards
	n := len(f.backends)
	usages := make([]*fs.Usage, n)
	g, gctx := errgroup.WithContext(ctx)
	for i := range f.backends {
		i := i
		g.Go(func() error {
			b := f.backends[i]
			doAbout := b.Features().About
			if doAbout == nil {
				return fmt.Errorf("rs: shard %d (%s): About not supported", i, b.String())
			}
			u, err := doAbout(gctx)
			if err != nil {
				return fmt.Errorf("rs: shard %d (%s): %w", i, b.String(), err)
			}
			if u == nil {
				return fmt.Errorf("rs: shard %d (%s): About returned nil usage", i, b.String())
			}
			usages[i] = u
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	usage, limitFreeIdx, limitFreeShard := aggregateLogicalUsage(usages, k)
	if limitFreeIdx >= 0 && usage.Free != nil {
		fs.Debugf(f, "rs About: limiting shard %d (%s): shard free=%d -> logical free=%d, k=%d",
			limitFreeIdx, f.backends[limitFreeIdx].String(), limitFreeShard, *usage.Free, k)
	}
	return usage, nil
}

// aggregateLogicalUsage combines per-shard About results into logical quota (see plan-e-about).
// limitFreeIdx and limitFreeShard are the shard that set logical Free (-1 if Free is nil).
func aggregateLogicalUsage(usages []*fs.Usage, k int) (usage *fs.Usage, limitFreeIdx int, limitFreeShard int64) {
	limitFreeIdx = -1
	usage = &fs.Usage{}

	freeVals := usageInts(usages, func(u *fs.Usage) *int64 { return u.Free })
	totalVals := usageInts(usages, func(u *fs.Usage) *int64 { return u.Total })
	usedVals := usageInts(usages, func(u *fs.Usage) *int64 { return u.Used })

	if logical, idx, shardVal, ok := logicalMinField(freeVals, k); ok {
		if logical == nil {
			usage.Free = nil
		} else {
			usage.Free = logical
			limitFreeIdx = idx
			limitFreeShard = shardVal
		}
	}

	if logical, _, _, ok := logicalMinField(totalVals, k); ok {
		if logical == nil {
			usage.Total = nil
		} else {
			usage.Total = logical
		}
	}

	if usage.Total != nil && usage.Free != nil {
		used := *usage.Total - *usage.Free
		usage.Used = fs.NewUsageValue(used)
	} else if logical, _, _, ok := logicalMinField(usedVals, k); ok {
		usage.Used = logical
	}

	if objects := minObjectsField(usages); len(objects) > 0 {
		if logical := minInt64Ptr(objects); logical == nil {
			usage.Objects = nil
		} else {
			usage.Objects = logical
		}
	}

	return usage, limitFreeIdx, limitFreeShard
}

func usageInts(usages []*fs.Usage, pick func(*fs.Usage) *int64) []*int64 {
	out := make([]*int64, len(usages))
	for i, u := range usages {
		out[i] = pick(u)
	}
	return out
}

// logicalMinField returns k*min(values). If any value is nil, logical is nil (union-style propagation).
// ok is false when no shard reported the field (all nil).
func logicalMinField(values []*int64, k int) (logical *int64, minIdx int, shardVal int64, ok bool) {
	minIdx = -1
	var minV int64
	for i, p := range values {
		if p == nil {
			return nil, -1, 0, true
		}
		if minIdx < 0 || *p < minV {
			minV = *p
			minIdx = i
			shardVal = *p
		}
	}
	if minIdx < 0 {
		return nil, -1, 0, false
	}
	if minV > math.MaxInt64/int64(k) {
		return fs.NewUsageValue(int64(math.MaxInt64)), minIdx, shardVal, true
	}
	return fs.NewUsageValue(minV * int64(k)), minIdx, shardVal, true
}

func minObjectsField(usages []*fs.Usage) []*int64 {
	var out []*int64
	for _, u := range usages {
		if u.Objects != nil {
			out = append(out, u.Objects)
		}
	}
	return out
}

func minInt64Ptr(values []*int64) *int64 {
	if len(values) == 0 {
		return nil
	}
	minV := int64(math.MaxInt64)
	for _, p := range values {
		if p == nil {
			return nil
		}
		if *p < minV {
			minV = *p
		}
	}
	return fs.NewUsageValue(minV)
}

var _ fs.Abouter = (*Fs)(nil)
