package rs

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

func TestListMetadataFastPathNoFooter(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-list-meta")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	data := []byte("list-metadata-fast-path")
	mtime := time.Unix(1700007000, 123456789)
	info := object.NewStaticObjectInfo("fast.bin", mtime, int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	obj, ok := entries[0].(*Object)
	require.True(t, ok)
	require.Nil(t, obj.footer, "list fast path should not load footer")
	require.Equal(t, int64(len(data)), obj.Size())
	require.Nil(t, obj.footer, "Size() should not load footer when k data shard sizes suffice")
	require.Equal(t, mtime.Truncate(time.Second), obj.ModTime(ctx))
	require.Nil(t, obj.footer, "ModTime() should not load footer when shard list times suffice")
}

func TestListMetadataFooterFallbackMissingDataShard(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-list-meta-fallback")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	data := []byte("footer-fallback")
	info := object.NewStaticObjectInfo("fb.bin", time.Unix(1700007100, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Drop data shard 0; parity shards still list the name (fileVotes >= k).
	shard0, err := backends[0].NewObject(ctx, "fb.bin")
	require.NoError(t, err)
	require.NoError(t, shard0.Remove(ctx))

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	obj, ok := entries[0].(*Object)
	require.True(t, ok)
	require.Nil(t, obj.footer, "list should not load footer eagerly")
	require.Equal(t, int64(len(data)), obj.Size(), "Size() lazy-loads footer when k data sizes unavailable")
	require.NotNil(t, obj.footer, "Size() should load footer for missing data-shard sizes")
}

func TestModTimePrefersRemoteOverFooter(t *testing.T) {
	ctx := context.Background()
	o := &Object{
		hasListModTime: true,
		listModTime:    time.Unix(1700008000, 0),
		footer: &Footer{
			Mtime: time.Unix(1700009000, 0).UnixNano(),
		},
	}
	require.Equal(t, time.Unix(1700008000, 0), o.ModTime(ctx))
}

func TestSizePrefersRemoteOverFooter(t *testing.T) {
	o := &Object{
		hasListSize: true,
		listSize:    42,
		footer: &Footer{
			ContentLength: 99,
		},
	}
	require.Equal(t, int64(42), o.Size())
}

func TestNewObjectMetadataFastPathNoFooter(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-newobj-fast")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	mtime := time.Unix(1700008400, 0)
	data := []byte("newobj-fast-path")
	info := object.NewStaticObjectInfo("fastnew.bin", mtime, int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	obj, err := f.NewObject(ctx, "fastnew.bin")
	require.NoError(t, err)
	rsObj, ok := obj.(*Object)
	require.True(t, ok)
	require.Nil(t, rsObj.footer, "NewObject should not read footer when shard metadata suffices")
	require.Equal(t, int64(len(data)), rsObj.Size())
	require.Equal(t, mtime.Truncate(time.Second), rsObj.ModTime(ctx))
}

func TestNewObjectModTimeProbesShardNotFooter(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-newobj-probe-mt")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	mtime := time.Unix(1700008100, 0)
	info := object.NewStaticObjectInfo("probe.bin", mtime, 11, true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader([]byte("probe-bytes")), info)
	require.NoError(t, err)

	obj, err := f.NewObject(ctx, "probe.bin")
	require.NoError(t, err)
	rsObj, ok := obj.(*Object)
	require.True(t, ok)
	require.Nil(t, rsObj.footer, "NewObject fast path should not load footer")
	require.Equal(t, mtime.Truncate(time.Second), rsObj.ModTime(ctx))
}

func TestModTimeAfterOpenPrefersListOverFooter(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-open-probe-mt")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	mtime := time.Unix(1700008300, 0)
	data := []byte("open-probe")
	info := object.NewStaticObjectInfo("open.bin", mtime, int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	obj := entries[0].(*Object)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.NotNil(t, obj.footer)

	obj.footer.Mtime = time.Unix(2, 0).UnixNano()
	require.Equal(t, mtime.Truncate(time.Second), obj.ModTime(ctx))
}

func TestNewObjectSizeProbesShardsNotFooter(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-newobj-probe-size")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	data := []byte("size-probe")
	info := object.NewStaticObjectInfo("size.bin", time.Unix(1700008200, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	obj, err := f.NewObject(ctx, "size.bin")
	require.NoError(t, err)
	rsObj, ok := obj.(*Object)
	require.True(t, ok)
	require.Nil(t, rsObj.footer, "NewObject fast path should not load footer")
	require.Equal(t, int64(len(data)), rsObj.Size())
}

func TestListOmitsBrokenBelowReadQuorum(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-list-broken")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        4,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	info := object.NewStaticObjectInfo("broken.bin", time.Unix(1700007200, 0), 5, true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader([]byte("break")), info)
	require.NoError(t, err)
	for _, shard := range []int{2, 3} {
		o, err := backends[shard].NewObject(ctx, "broken.bin")
		require.NoError(t, err)
		require.NoError(t, o.Remove(ctx))
	}

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Empty(t, entries)
}
