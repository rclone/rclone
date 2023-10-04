package operations

import (
	"context"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/random"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoMultiThreadCopy(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	f, err := mockfs.NewFs(ctx, "potato", "", nil)
	require.NoError(t, err)
	src := mockobject.New("file.txt").WithContent([]byte(random.String(100)), mockobject.SeekModeNone)
	srcFs, err := mockfs.NewFs(ctx, "sausage", "", nil)
	require.NoError(t, err)
	src.SetFs(srcFs)

	oldStreams := ci.MultiThreadStreams
	oldCutoff := ci.MultiThreadCutoff
	oldIsSet := ci.MultiThreadSet
	defer func() {
		ci.MultiThreadStreams = oldStreams
		ci.MultiThreadCutoff = oldCutoff
		ci.MultiThreadSet = oldIsSet
	}()

	ci.MultiThreadStreams, ci.MultiThreadCutoff = 4, 50
	ci.MultiThreadSet = false

	nullWriterAt := func(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
		panic("don't call me")
	}
	f.Features().OpenWriterAt = nullWriterAt

	assert.True(t, doMultiThreadCopy(ctx, f, src))

	ci.MultiThreadStreams = 0
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	ci.MultiThreadStreams = 1
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	ci.MultiThreadStreams = 2
	assert.True(t, doMultiThreadCopy(ctx, f, src))

	ci.MultiThreadCutoff = 200
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	ci.MultiThreadCutoff = 101
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	ci.MultiThreadCutoff = 100
	assert.True(t, doMultiThreadCopy(ctx, f, src))

	f.Features().OpenWriterAt = nil
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	f.Features().OpenWriterAt = nullWriterAt
	assert.True(t, doMultiThreadCopy(ctx, f, src))

	f.Features().IsLocal = true
	srcFs.Features().IsLocal = true
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	ci.MultiThreadSet = true
	assert.True(t, doMultiThreadCopy(ctx, f, src))
	ci.MultiThreadSet = false
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	srcFs.Features().IsLocal = false
	assert.True(t, doMultiThreadCopy(ctx, f, src))
	srcFs.Features().IsLocal = true
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	f.Features().IsLocal = false
	assert.True(t, doMultiThreadCopy(ctx, f, src))
	srcFs.Features().IsLocal = false
	assert.True(t, doMultiThreadCopy(ctx, f, src))

	srcFs.Features().NoMultiThreading = true
	assert.False(t, doMultiThreadCopy(ctx, f, src))
	srcFs.Features().NoMultiThreading = false
	assert.True(t, doMultiThreadCopy(ctx, f, src))
}

func TestMultithreadCalculateNumChunks(t *testing.T) {
	for _, test := range []struct {
		size          int64
		chunkSize     int64
		wantNumChunks int
	}{
		{size: 1, chunkSize: multithreadChunkSize, wantNumChunks: 1},
		{size: 1 << 20, chunkSize: 1, wantNumChunks: 1 << 20},
		{size: 1 << 20, chunkSize: 2, wantNumChunks: 1 << 19},
		{size: (1 << 20) + 1, chunkSize: 2, wantNumChunks: (1 << 19) + 1},
		{size: (1 << 20) - 1, chunkSize: 2, wantNumChunks: 1 << 19},
	} {
		t.Run(fmt.Sprintf("%+v", test), func(t *testing.T) {
			mc := &multiThreadCopyState{
				size: test.size,
			}
			mc.numChunks = calculateNumChunks(test.size, test.chunkSize)
			assert.Equal(t, test.wantNumChunks, mc.numChunks)
		})
	}
}

func TestMultithreadCopy(t *testing.T) {
	r := fstest.NewRun(t)
	ctx := context.Background()

	for _, test := range []struct {
		size    int
		streams int
	}{
		{size: multithreadChunkSize*2 - 1, streams: 2},
		{size: multithreadChunkSize * 2, streams: 2},
		{size: multithreadChunkSize*2 + 1, streams: 2},
	} {
		t.Run(fmt.Sprintf("%+v", test), func(t *testing.T) {
			if *fstest.SizeLimit > 0 && int64(test.size) > *fstest.SizeLimit {
				t.Skipf("exceeded file size limit %d > %d", test.size, *fstest.SizeLimit)
			}
			var err error
			contents := random.String(test.size)
			t1 := fstest.Time("2001-02-03T04:05:06.499999999Z")
			file1 := r.WriteObject(ctx, "file1", contents, t1)
			r.CheckRemoteItems(t, file1)
			r.CheckLocalItems(t)

			src, err := r.Fremote.NewObject(ctx, "file1")
			require.NoError(t, err)
			accounting.GlobalStats().ResetCounters()
			tr := accounting.GlobalStats().NewTransfer(src)

			defer func() {
				tr.Done(ctx, err)
			}()
			dst, err := multiThreadCopy(ctx, r.Flocal, "file1", src, 2, tr)
			require.NoError(t, err)
			assert.Equal(t, src.Size(), dst.Size())
			assert.Equal(t, "file1", dst.Remote())

			fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1}, nil, fs.GetModifyWindow(ctx, r.Flocal, r.Fremote))
			require.NoError(t, dst.Remove(ctx))
		})
	}

}
