package rs

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/hash"
	"github.com/stretchr/testify/require"
)

// featuresStubFs wraps a shard Fs and overrides selected capability flags for masking tests.
type featuresStubFs struct {
	fs.Fs
	canHaveEmptyDirectories bool
	move                    bool
	setTier                 bool
}

func (s featuresStubFs) Features() *fs.Features {
	base := s.Fs.Features()
	ft := *base
	ft.CanHaveEmptyDirectories = s.canHaveEmptyDirectories
	if !s.move {
		ft.Move = nil
	}
	if !s.setTier {
		ft.SetTier = false
	}
	return &ft
}

func testFeaturesFs(t *testing.T, backends []fs.Fs) *Fs {
	t.Helper()
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   len(backends) - 1,
			ParityShards: 1,
		},
		hashSet: hash.NewHashSet(),
	}
	f.initFeatures(context.Background())
	return f
}

func TestFeaturesAllLocalShardsCanHaveEmptyDirectories(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, root+"/shard-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := testFeaturesFs(t, backends)
	require.True(t, f.Features().CanHaveEmptyDirectories, "all-local shards should allow empty directories")
	require.True(t, f.Features().SlowHash, "SlowHash: logical Hash() reads EC footer per object (extra round-trip)")
}

func TestFeaturesEmptyDirsFalseWhenAnyShardLacks(t *testing.T) {
	backends := makeMemoryBackendsForFeatures(t, 4)
	backends[2] = featuresStubFs{
		Fs:                      backends[2],
		canHaveEmptyDirectories: false,
		move:                    true,
		setTier:                 true,
	}
	f := testFeaturesFs(t, backends)
	require.False(t, f.Features().CanHaveEmptyDirectories, "one shard without empty dirs must mask rs off")
}

func TestFeaturesMoveMaskedWhenShardLacksMove(t *testing.T) {
	backends := makeMemoryBackendsForFeatures(t, 4)
	backends[1] = featuresStubFs{
		Fs:                      backends[1],
		canHaveEmptyDirectories: true,
		move:                    false,
		setTier:                 true,
	}
	f := testFeaturesFs(t, backends)
	require.Nil(t, f.Features().Move, "Move must be dropped when any shard lacks it")
}

func TestFeaturesSetTierNeverAdvertised(t *testing.T) {
	backends := makeMemoryBackendsForFeatures(t, 4)
	f := testFeaturesFs(t, backends)
	require.False(t, f.Features().SetTier, "rs logical objects have no tier API")
	require.False(t, f.Features().GetTier)
}

func TestFeaturesBucketBasedNeverAdvertised(t *testing.T) {
	backends := makeMemoryBackendsForFeatures(t, 4)
	f := testFeaturesFs(t, backends)
	require.False(t, f.Features().BucketBased)
	require.False(t, f.Features().BucketBasedRootOK)
}

func makeMemoryBackendsForFeatures(t *testing.T, n int) []fs.Fs {
	t.Helper()
	ctx := context.Background()
	backends := make([]fs.Fs, n)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-features-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	return backends
}
