package operations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
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

// Skip if not multithread, returning the chunkSize otherwise
func skipIfNotMultithread(ctx context.Context, t *testing.T, r *fstest.Run) int {
	features := r.Fremote.Features()
	if features.OpenChunkWriter == nil && features.OpenWriterAt == nil {
		t.Skip("multithread writing not supported")
	}

	// Only support one hash for the local backend otherwise we end up spending a huge amount of CPU on hashing!
	if r.Fremote.Features().IsLocal {
		oldHashes := hash.SupportOnly([]hash.Type{r.Fremote.Hashes().GetOne()})
		t.Cleanup(func() {
			_ = hash.SupportOnly(oldHashes)
		})
	}

	ci := fs.GetConfig(ctx)
	chunkSize := int(ci.MultiThreadChunkSize)
	if features.OpenChunkWriter != nil {
		//OpenChunkWriter func(ctx context.Context, remote string, src ObjectInfo, options ...OpenOption) (info ChunkWriterInfo, writer ChunkWriter, err error)
		const fileName = "chunksize-probe"
		src := object.NewStaticObjectInfo(fileName, time.Now(), int64(100*fs.Mebi), true, nil, nil)
		info, writer, err := features.OpenChunkWriter(ctx, fileName, src)
		require.NoError(t, err)
		chunkSize = int(info.ChunkSize)
		err = writer.Abort(ctx)
		require.NoError(t, err)
	}
	return chunkSize
}

func TestMultithreadCopy(t *testing.T) {
	r := fstest.NewRun(t)
	ctx := context.Background()
	chunkSize := skipIfNotMultithread(ctx, t, r)
	// Check every other transfer for metadata
	checkMetadata := false
	ctx, ci := fs.AddConfig(ctx)

	for _, upload := range []bool{false, true} {
		for _, test := range []struct {
			size    int
			streams int
		}{
			{size: chunkSize*2 - 1, streams: 2},
			{size: chunkSize * 2, streams: 2},
			{size: chunkSize*2 + 1, streams: 2},
		} {
			checkMetadata = !checkMetadata
			ci.Metadata = checkMetadata
			fileName := fmt.Sprintf("test-multithread-copy-%v-%d-%d", upload, test.size, test.streams)
			t.Run(fmt.Sprintf("upload=%v,size=%v,streams=%v", upload, test.size, test.streams), func(t *testing.T) {
				if *fstest.SizeLimit > 0 && int64(test.size) > *fstest.SizeLimit {
					t.Skipf("exceeded file size limit %d > %d", test.size, *fstest.SizeLimit)
				}
				var (
					contents     = random.String(test.size)
					t1           = fstest.Time("2001-02-03T04:05:06.499999999Z")
					file1        fstest.Item
					src, dst     fs.Object
					err          error
					testMetadata = fs.Metadata{
						// System metadata supported by all backends
						"mtime": t1.Format(time.RFC3339Nano),
						// User metadata
						"potato": "jersey",
					}
				)

				var fSrc, fDst fs.Fs
				if upload {
					file1 = r.WriteFile(fileName, contents, t1)
					r.CheckRemoteItems(t)
					r.CheckLocalItems(t, file1)
					fDst, fSrc = r.Fremote, r.Flocal
				} else {
					file1 = r.WriteObject(ctx, fileName, contents, t1)
					r.CheckRemoteItems(t, file1)
					r.CheckLocalItems(t)
					fDst, fSrc = r.Flocal, r.Fremote
				}
				src, err = fSrc.NewObject(ctx, fileName)
				require.NoError(t, err)

				do, canSetMetadata := src.(fs.SetMetadataer)
				if checkMetadata && canSetMetadata {
					// Set metadata on the source if required
					err := do.SetMetadata(ctx, testMetadata)
					if err == fs.ErrorNotImplemented {
						canSetMetadata = false
					} else {
						require.NoError(t, err)
						fstest.CheckEntryMetadata(ctx, t, r.Flocal, src, testMetadata)
					}
				}

				accounting.GlobalStats().ResetCounters()
				tr := accounting.GlobalStats().NewTransfer(src, nil)

				defer func() {
					tr.Done(ctx, err)
				}()

				dst, err = multiThreadCopy(ctx, fDst, fileName, src, test.streams, tr)
				require.NoError(t, err)

				assert.Equal(t, src.Size(), dst.Size())
				assert.Equal(t, fileName, dst.Remote())
				fstest.CheckListingWithPrecision(t, fSrc, []fstest.Item{file1}, nil, fs.GetModifyWindow(ctx, fDst, fSrc))
				fstest.CheckListingWithPrecision(t, fDst, []fstest.Item{file1}, nil, fs.GetModifyWindow(ctx, fDst, fSrc))

				if checkMetadata && canSetMetadata && fDst.Features().ReadMetadata {
					fstest.CheckEntryMetadata(ctx, t, fDst, dst, testMetadata)
				}

				require.NoError(t, dst.Remove(ctx))
				require.NoError(t, src.Remove(ctx))

			})
		}
	}
}

type errorObject struct {
	fs.Object
	size int64
	wg   *sync.WaitGroup
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
//
// Remember this is called multiple times whenever the backend seeks (eg having read checksum)
func (o errorObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.Debugf(nil, "Open with options = %v", options)
	rc, err := o.Object.Open(ctx, options...)
	if err != nil {
		return nil, err
	}
	// Return an error reader for the second segment
	for _, option := range options {
		if ropt, ok := option.(*fs.RangeOption); ok {
			end := ropt.End + 1
			if end >= o.size {
				// Give the other chunks a chance to start
				time.Sleep(time.Second)
				// Wait for chunks to upload first
				o.wg.Wait()
				fs.Debugf(nil, "Returning error reader")
				return errorReadCloser{rc}, nil
			}
		}
	}
	o.wg.Add(1)
	return wgReadCloser{rc, o.wg}, nil
}

type errorReadCloser struct {
	io.ReadCloser
}

func (rc errorReadCloser) Read(p []byte) (n int, err error) {
	fs.Debugf(nil, "BOOM: simulated read failure")
	return 0, errors.New("BOOM: simulated read failure")
}

type wgReadCloser struct {
	io.ReadCloser
	wg *sync.WaitGroup
}

func (rc wgReadCloser) Close() (err error) {
	rc.wg.Done()
	return rc.ReadCloser.Close()
}

// Make sure aborting the multi-thread copy doesn't overwrite an existing file.
func TestMultithreadCopyAbort(t *testing.T) {
	r := fstest.NewRun(t)
	ctx := context.Background()
	chunkSize := skipIfNotMultithread(ctx, t, r)
	size := 2*chunkSize + 1

	if *fstest.SizeLimit > 0 && int64(size) > *fstest.SizeLimit {
		t.Skipf("exceeded file size limit %d > %d", size, *fstest.SizeLimit)
	}

	// first write a canary file which we are trying not to overwrite
	const fileName = "test-multithread-abort"
	contents := random.String(100)
	t1 := fstest.Time("2001-02-03T04:05:06.499999999Z")
	canary := r.WriteObject(ctx, fileName, contents, t1)
	r.CheckRemoteItems(t, canary)

	// Now write a local file to upload
	file1 := r.WriteFile(fileName, random.String(size), t1)
	r.CheckLocalItems(t, file1)

	src, err := r.Flocal.NewObject(ctx, fileName)
	require.NoError(t, err)
	accounting.GlobalStats().ResetCounters()
	tr := accounting.GlobalStats().NewTransfer(src, nil)

	defer func() {
		tr.Done(ctx, err)
	}()
	wg := new(sync.WaitGroup)
	dst, err := multiThreadCopy(ctx, r.Fremote, fileName, errorObject{src, int64(size), wg}, 1, tr)
	assert.Error(t, err)
	assert.Nil(t, dst)

	if r.Fremote.Features().PartialUploads {
		r.CheckRemoteItems(t)

	} else {
		r.CheckRemoteItems(t, canary)
		o, err := r.Fremote.NewObject(ctx, fileName)
		require.NoError(t, err)
		require.NoError(t, o.Remove(ctx))
	}
}
