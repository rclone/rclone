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
	require.Equal(t, mtime.Truncate(time.Second), obj.ModTime(ctx))
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
	require.NotNil(t, obj.footer, "missing data-shard sizes should trigger footer read")
	require.Equal(t, int64(len(data)), obj.Size())
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
