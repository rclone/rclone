package rs

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/require"
)

// aboutStubFs wraps a shard Fs with a fixed About result for quota tests.
type aboutStubFs struct {
	fs.Fs
	usage    *fs.Usage
	aboutErr error
	noAbout  bool
}

func (s aboutStubFs) Features() *fs.Features {
	ft := *s.Fs.Features()
	if s.noAbout {
		ft.About = nil
	} else {
		u, err := s.usage, s.aboutErr
		ft.About = func(ctx context.Context) (*fs.Usage, error) {
			if err != nil {
				return nil, err
			}
			return u, nil
		}
	}
	return &ft
}

func testAboutFs(t *testing.T, backends []fs.Fs, k, m int) *Fs {
	t.Helper()
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   k,
			ParityShards: m,
		},
		hashSet: hash.NewHashSet(),
	}
	f.initFeatures(context.Background())
	return f
}

func usageWith(free, total, used int64) *fs.Usage {
	u := &fs.Usage{}
	if free >= 0 {
		u.Free = fs.NewUsageValue(free)
	}
	if total >= 0 {
		u.Total = fs.NewUsageValue(total)
	}
	if used >= 0 {
		u.Used = fs.NewUsageValue(used)
	}
	return u
}

func TestAboutAggregateLogicalMin(t *testing.T) {
	usages := []*fs.Usage{
		usageWith(100, 1000, 900),
		usageWith(200, 1000, 800),
		usageWith(50, 1000, 950),
		usageWith(300, 1000, 700),
	}
	got, idx, shardFree := aggregateLogicalUsage(usages, 3)
	require.Equal(t, 2, idx)
	require.Equal(t, int64(50), shardFree)
	require.NotNil(t, got.Free)
	require.Equal(t, int64(150), *got.Free)
	require.NotNil(t, got.Total)
	require.Equal(t, int64(3000), *got.Total)
	require.NotNil(t, got.Used)
	require.Equal(t, int64(2850), *got.Used) // Total - Free
}

func TestAboutNilFreePropagation(t *testing.T) {
	usages := []*fs.Usage{
		usageWith(100, 1000, 900),
		{Total: fs.NewUsageValue(int64(1000)), Used: fs.NewUsageValue(int64(900)), Free: nil},
		usageWith(50, 1000, 950),
		usageWith(300, 1000, 700),
	}
	got, _, _ := aggregateLogicalUsage(usages, 3)
	require.Nil(t, got.Free)
	require.NotNil(t, got.Total)
	require.Equal(t, int64(3000), *got.Total)
	require.NotNil(t, got.Used)
	require.Equal(t, int64(2100), *got.Used) // k * min(Used_i); Total−Free not used when Free unknown
}

func TestAboutObjectsMinWhenReported(t *testing.T) {
	usages := []*fs.Usage{
		{Objects: fs.NewUsageValue(int64(100))},
		{Objects: fs.NewUsageValue(int64(42))},
		{Objects: nil},
	}
	got, _, _ := aggregateLogicalUsage(usages, 2)
	require.NotNil(t, got.Objects)
	require.Equal(t, int64(42), *got.Objects)
}

func TestAboutObjectsNilWhenNoneReported(t *testing.T) {
	usages := []*fs.Usage{
		usageWith(100, 1000, 900),
		usageWith(100, 1000, 900),
	}
	got, _, _ := aggregateLogicalUsage(usages, 2)
	require.Nil(t, got.Objects)
}

func TestAboutAllLocalShards(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	k, m := 3, 1
	backends := make([]fs.Fs, k+m)
	var shardFrees []int64
	for i := range backends {
		dir := root + "/shard-" + strconv.Itoa(i)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		b, err := cache.Get(ctx, dir)
		require.NoError(t, err)
		backends[i] = b
		su, err := b.Features().About(ctx)
		require.NoError(t, err)
		require.NotNil(t, su)
		require.NotNil(t, su.Free)
		shardFrees = append(shardFrees, *su.Free)
	}
	minFree := shardFrees[0]
	for _, v := range shardFrees[1:] {
		if v < minFree {
			minFree = v
		}
	}

	f := testAboutFs(t, backends, k, m)
	require.NotNil(t, f.Features().About)
	u, err := f.About(ctx)
	require.NoError(t, err)
	require.NotNil(t, u.Free)
	require.InDelta(t, float64(int64(k)*minFree), float64(*u.Free), float64(k)*1024*1024,
		"logical free should be k * min(shard free) within ~1MiB per shard rounding")
}

func makeLocalBackendsForAbout(t *testing.T, n int) []fs.Fs {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	backends := make([]fs.Fs, n)
	for i := range backends {
		dir := root + "/about-shard-" + strconv.Itoa(i)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		b, err := cache.Get(ctx, dir)
		require.NoError(t, err)
		backends[i] = b
	}
	return backends
}

func TestAboutLimitingShard(t *testing.T) {
	ctx := context.Background()
	k, m := 3, 1
	base := makeLocalBackendsForAbout(t, k+m)
	const smallFree = 4096
	backends := make([]fs.Fs, len(base))
	for i := range base {
		u := usageWith(1<<30, 1<<31, (1<<31)-(1<<30))
		if i == 2 {
			u = usageWith(smallFree, 1<<20, (1<<20)-smallFree)
		}
		backends[i] = aboutStubFs{Fs: base[i], usage: u}
	}
	f := testAboutFs(t, backends, k, m)
	u, err := f.About(ctx)
	require.NoError(t, err)
	require.NotNil(t, u.Free)
	require.Equal(t, int64(k*smallFree), *u.Free)
}

func TestAboutShardWithoutAboutMasksFeature(t *testing.T) {
	backends := makeLocalBackendsForAbout(t, 4)
	backends[1] = aboutStubFs{Fs: backends[1], noAbout: true}
	f := testAboutFs(t, backends, 3, 1)
	require.Nil(t, f.Features().About)
}

func TestAboutErrorsWhenShardAboutFails(t *testing.T) {
	ctx := context.Background()
	k, m := 3, 1
	base := makeLocalBackendsForAbout(t, k+m)
	backends := make([]fs.Fs, len(base))
	for i := range base {
		stub := aboutStubFs{Fs: base[i], usage: usageWith(1000, 2000, 1000)}
		if i == 1 {
			stub.aboutErr = errors.New("injected About failure")
		}
		backends[i] = stub
	}
	f := testAboutFs(t, backends, k, m)
	_, err := f.About(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "shard 1")
}

func TestAboutErrorsWhenShardReturnsNilUsage(t *testing.T) {
	ctx := context.Background()
	k, m := 3, 1
	base := makeLocalBackendsForAbout(t, k+m)
	backends := make([]fs.Fs, len(base))
	for i := range base {
		u := usageWith(1000, 2000, 1000)
		if i == 0 {
			u = nil
		}
		backends[i] = aboutStubFs{Fs: base[i], usage: u}
	}
	f := testAboutFs(t, backends, k, m)
	_, err := f.About(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "shard 0")
	require.Contains(t, err.Error(), "nil usage")
}
