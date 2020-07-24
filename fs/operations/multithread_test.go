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
	f := mockfs.NewFs("potato", "")
	src := mockobject.New("file.txt").WithContent([]byte(random.String(100)), mockobject.SeekModeNone)
	srcFs := mockfs.NewFs("sausage", "")
	src.SetFs(srcFs)

	oldStreams := fs.Config.MultiThreadStreams
	oldCutoff := fs.Config.MultiThreadCutoff
	oldIsSet := fs.Config.MultiThreadSet
	defer func() {
		fs.Config.MultiThreadStreams = oldStreams
		fs.Config.MultiThreadCutoff = oldCutoff
		fs.Config.MultiThreadSet = oldIsSet
	}()

	fs.Config.MultiThreadStreams, fs.Config.MultiThreadCutoff = 4, 50
	fs.Config.MultiThreadSet = false

	nullWriterAt := func(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
		panic("don't call me")
	}
	f.Features().OpenWriterAt = nullWriterAt

	assert.True(t, doMultiThreadCopy(f, src))

	fs.Config.MultiThreadStreams = 0
	assert.False(t, doMultiThreadCopy(f, src))
	fs.Config.MultiThreadStreams = 1
	assert.False(t, doMultiThreadCopy(f, src))
	fs.Config.MultiThreadStreams = 2
	assert.True(t, doMultiThreadCopy(f, src))

	fs.Config.MultiThreadCutoff = 200
	assert.False(t, doMultiThreadCopy(f, src))
	fs.Config.MultiThreadCutoff = 101
	assert.False(t, doMultiThreadCopy(f, src))
	fs.Config.MultiThreadCutoff = 100
	assert.True(t, doMultiThreadCopy(f, src))

	f.Features().OpenWriterAt = nil
	assert.False(t, doMultiThreadCopy(f, src))
	f.Features().OpenWriterAt = nullWriterAt
	assert.True(t, doMultiThreadCopy(f, src))

	f.Features().IsLocal = true
	srcFs.Features().IsLocal = true
	assert.False(t, doMultiThreadCopy(f, src))
	fs.Config.MultiThreadSet = true
	assert.True(t, doMultiThreadCopy(f, src))
	fs.Config.MultiThreadSet = false
	assert.False(t, doMultiThreadCopy(f, src))
	srcFs.Features().IsLocal = false
	assert.True(t, doMultiThreadCopy(f, src))
	srcFs.Features().IsLocal = true
	assert.False(t, doMultiThreadCopy(f, src))
	f.Features().IsLocal = false
	assert.True(t, doMultiThreadCopy(f, src))
	srcFs.Features().IsLocal = false
	assert.True(t, doMultiThreadCopy(f, src))
}

func TestMultithreadCalculateChunks(t *testing.T) {
	for _, test := range []struct {
		size         int64
		streams      int
		wantPartSize int64
		wantStreams  int
	}{
		{size: 1, streams: 10, wantPartSize: multithreadChunkSize, wantStreams: 1},
		{size: 1 << 20, streams: 1, wantPartSize: 1 << 20, wantStreams: 1},
		{size: 1 << 20, streams: 2, wantPartSize: 1 << 19, wantStreams: 2},
		{size: (1 << 20) + 1, streams: 2, wantPartSize: (1 << 19) + multithreadChunkSize, wantStreams: 2},
		{size: (1 << 20) - 1, streams: 2, wantPartSize: (1 << 19), wantStreams: 2},
	} {
		t.Run(fmt.Sprintf("%+v", test), func(t *testing.T) {
			mc := &multiThreadCopyState{
				size:    test.size,
				streams: test.streams,
			}
			mc.calculateChunks()
			assert.Equal(t, test.wantPartSize, mc.partSize)
			assert.Equal(t, test.wantStreams, mc.streams)
		})
	}
}

func TestMultithreadCopy(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

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
			file1 := r.WriteObject(context.Background(), "file1", contents, t1)
			fstest.CheckItems(t, r.Fremote, file1)
			fstest.CheckItems(t, r.Flocal)

			src, err := r.Fremote.NewObject(context.Background(), "file1")
			require.NoError(t, err)
			accounting.GlobalStats().ResetCounters()
			tr := accounting.GlobalStats().NewTransfer(src)

			defer func() {
				tr.Done(err)
			}()
			dst, err := multiThreadCopy(context.Background(), r.Flocal, "file1", src, 2, tr)
			require.NoError(t, err)
			assert.Equal(t, src.Size(), dst.Size())
			assert.Equal(t, "file1", dst.Remote())

			fstest.CheckListingWithPrecision(t, r.Flocal, []fstest.Item{file1}, nil, fs.GetModifyWindow(r.Flocal, r.Fremote))
			require.NoError(t, dst.Remove(context.Background()))
		})
	}

}
