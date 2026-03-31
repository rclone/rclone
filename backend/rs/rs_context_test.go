package rs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/require"
)

// slowPutFs delays before delegating Put so uploads can be cancelled mid-flight.
type slowPutFs struct {
	fs.Fs
	delay time.Duration
}

func (s *slowPutFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(s.delay):
	}
	return s.Fs.Put(ctx, in, src, options...)
}

func TestPutCancelsDuringSourceRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-ctx-read-"+string(rune('0'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			MaxParallelUploads: 2,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}
	data := bytes.Repeat([]byte("x"), 8000)
	// io.ReadAll may finish in one read for small inputs; pre-cancel ensures
	// NewContextReader's first Read observes context.Canceled.
	cancel()
	in := readers.NewContextReader(ctx, bytes.NewReader(data))
	_, err := f.Put(ctx, in, object.NewStaticObjectInfo("cancel.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "got %v", err)
}

func TestPutCancelsDuringUpload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-ctx-upload-"+string(rune('0'+i)))
		require.NoError(t, err)
		if i == 0 {
			backends[i] = &slowPutFs{Fs: b, delay: 400 * time.Millisecond}
		} else {
			backends[i] = b
		}
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			MaxParallelUploads: 4,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}
	data := bytes.Repeat([]byte("y"), 2048)
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	_, err := f.Put(ctx, bytes.NewReader(data), object.NewStaticObjectInfo("upcancel.bin", time.Unix(1700000001, 0), int64(len(data)), true, nil, nil))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "got %v", err)
}

func TestListAllObjectRemotesCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(context.Background(), ":memory:rs-ctx-list-"+string(rune('0'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{backends: backends, opt: Options{DataShards: 2, ParityShards: 2}}
	_, err := f.listAllObjectRemotes(ctx)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "got %v", err)
}

func TestHealCommandCancelledBeforeList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(context.Background(), ":memory:rs-ctx-heal-"+string(rune('0'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{backends: backends, opt: Options{DataShards: 2, ParityShards: 2}}
	_, err := f.healCommand(ctx, nil, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "got %v", err)
}
