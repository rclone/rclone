package vfs

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Open a file for write
func writeHandleCreate(t *testing.T) (r *fstest.Run, vfs *VFS, fh *WriteFileHandle) {
	r, vfs = newTestVFS(t)

	h, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	fh, ok := h.(*WriteFileHandle)
	require.True(t, ok)

	return r, vfs, fh
}

// Test write when underlying storage is readonly, must be run as non-root
func TestWriteFileHandleReadonly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping test on %s", runtime.GOOS)
	}
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	r, vfs, fh := writeHandleCreate(t)

	// Name
	assert.Equal(t, "file1", fh.Name())

	// Write a file, so underlying remote will be created
	_, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)

	err = fh.Close()
	assert.NoError(t, err)

	var info os.FileInfo
	info, err = os.Stat(r.FremoteName)
	assert.NoError(t, err)

	// Remove write permission
	oldMode := info.Mode()
	err = os.Chmod(r.FremoteName, oldMode^(oldMode&0222))
	assert.NoError(t, err)

	var h Handle
	h, err = vfs.OpenFile("file2", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)

	var ok bool
	fh, ok = h.(*WriteFileHandle)
	require.True(t, ok)

	// error is propagated to Close()
	_, err = fh.Write([]byte("hello"))
	assert.NoError(t, err)

	err = fh.Close()
	assert.NotNil(t, err)

	// Remove should fail
	err = vfs.Remove("file1")
	assert.NotNil(t, err)

	// Only file1 should exist
	_, err = vfs.Stat("file1")
	assert.NoError(t, err)

	_, err = vfs.Stat("file2")
	assert.Equal(t, true, errors.Is(err, os.ErrNotExist))

	// Restore old permission
	err = os.Chmod(r.FremoteName, oldMode)
	assert.NoError(t, err)
}

func TestWriteFileHandleMethods(t *testing.T) {
	r, vfs, fh := writeHandleCreate(t)

	// String
	assert.Equal(t, "file1 (w)", fh.String())
	assert.Equal(t, "<nil *WriteFileHandle>", (*WriteFileHandle)(nil).String())
	assert.Equal(t, "<nil *WriteFileHandle.file>", new(WriteFileHandle).String())

	// Node
	node := fh.Node()
	assert.Equal(t, "file1", node.Name())

	// Offset #1
	assert.Equal(t, int64(0), fh.Offset())
	assert.Equal(t, int64(0), node.Size())

	// Write (smoke test only since heavy lifting done in WriteAt)
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Offset #2
	assert.Equal(t, int64(5), fh.Offset())
	assert.Equal(t, int64(5), node.Size())

	// Stat
	var fi os.FileInfo
	fi, err = fh.Stat()
	assert.NoError(t, err)
	assert.Equal(t, int64(5), fi.Size())
	assert.Equal(t, "file1", fi.Name())

	// Read
	var buf = make([]byte, 16)
	_, err = fh.Read(buf)
	assert.Equal(t, EPERM, err)

	// ReadAt
	_, err = fh.ReadAt(buf, 0)
	assert.Equal(t, EPERM, err)

	// Sync
	err = fh.Sync()
	assert.NoError(t, err)

	// Truncate - can only truncate where the file pointer is
	err = fh.Truncate(5)
	assert.NoError(t, err)
	err = fh.Truncate(6)
	assert.Equal(t, EPERM, err)

	// Close
	assert.NoError(t, fh.Close())

	// Check double close
	err = fh.Close()
	assert.Equal(t, ECLOSED, err)

	// check vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"file1,5,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)

	// Check trying to open the file now it exists then closing it
	// immediately is OK
	h, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,5,false"})

	// Check trying to open the file and writing it now it exists
	// returns an error
	h, err = vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = h.Write([]byte("hello1"))
	require.Equal(t, EPERM, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,5,false"})

	// Check opening the file with O_TRUNC does actually truncate
	// it even if we don't write to it
	h, err = vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)
	err = h.Close()
	if !errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
		assert.NoError(t, err)
		checkListing(t, root, []string{"file1,0,false"})
	}

	// Check opening the file with O_TRUNC and writing does work
	h, err = vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)
	_, err = h.WriteString("hello12")
	require.NoError(t, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,7,false"})
}

func TestWriteFileHandleWriteAt(t *testing.T) {
	r, vfs, fh := writeHandleCreate(t)

	// Preconditions
	assert.Equal(t, int64(0), fh.offset)
	assert.False(t, fh.writeCalled)

	// Write the data
	n, err := fh.WriteAt([]byte("hello"), 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// After write
	assert.Equal(t, int64(5), fh.offset)
	assert.True(t, fh.writeCalled)

	// Check can't seek
	n, err = fh.WriteAt([]byte("hello"), 100)
	assert.Equal(t, ESPIPE, err)
	assert.Equal(t, 0, n)

	// Write more data
	n, err = fh.WriteAt([]byte(" world"), 5)
	assert.NoError(t, err)
	assert.Equal(t, 6, n)

	// Close
	assert.NoError(t, fh.Close())

	// Check can't write on closed handle
	n, err = fh.WriteAt([]byte("hello"), 0)
	assert.Equal(t, ECLOSED, err)
	assert.Equal(t, 0, n)

	// check vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"file1,11,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello world", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)
}

func TestWriteFileHandleCloseWithError(t *testing.T) {
	// io.ErrUnexpectedEOF must fail the upload like any other reason.
	for _, reason := range []error{errors.New("upload interrupted"), io.ErrUnexpectedEOF} {
		t.Run(reason.Error(), func(t *testing.T) {
			r, vfs, fh := writeHandleCreate(t)

			// Write some data then abandon the write mid-stream
			n, err := fh.Write([]byte("hello"))
			require.NoError(t, err)
			assert.Equal(t, 5, n)

			err = fh.CloseWithError(reason)
			require.Error(t, err)
			assert.ErrorContains(t, err, reason.Error())

			// Check double close
			assert.Equal(t, ECLOSED, fh.Close())

			if *fstest.RemoteName == "" {
				_, err = vfs.Stat("file1")
				assert.Equal(t, ENOENT, err, "The abandoned file must not exist in the VFS or on the remote")
				fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, fs.ModTimeNotSupported)
			}
		})
	}

	// CloseWithError with a nil reason behaves like Close and commits the file
	t.Run("NilReason", func(t *testing.T) {
		r, vfs := newTestVFS(t)
		h, err := vfs.OpenFile("file2", os.O_WRONLY|os.O_CREATE, 0777)
		require.NoError(t, err)
		fh2, ok := h.(*WriteFileHandle)
		require.True(t, ok)
		_, err = fh2.Write([]byte("hello"))
		require.NoError(t, err)
		require.NoError(t, fh2.CloseWithError(nil))
		file2 := fstest.NewItem("file2", "hello", t1)
		fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file2}, []string{}, fs.ModTimeNotSupported)
	})
}

func TestWriteFileHandleFlush(t *testing.T) {
	_, vfs, fh := writeHandleCreate(t)

	// Check Flush already creates file for unwritten handles, without closing it
	err := fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)
	root, err := vfs.Root()
	assert.NoError(t, err)
	checkListing(t, root, []string{"file1,0,false"})

	// Write some data
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Check Flush closes file if write called
	err = fh.Flush()
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check flush does nothing if called again
	err = fh.Flush()
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check file was written properly
	root, err = vfs.Root()
	assert.NoError(t, err)
	checkListing(t, root, []string{"file1,5,false"})
}

func TestWriteFileHandleRelease(t *testing.T) {
	_, _, fh := writeHandleCreate(t)

	// Check Release closes file
	err := fh.Release()
	if errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
		t.Logf("skipping test: %v", err)
		return
	}
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check Release does nothing if called again
	err = fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)
}

var (
	canSetModTimeOnce  sync.Once
	canSetModTimeValue = true
)

// returns whether the remote can set modtime
func canSetModTime(t *testing.T, r *fstest.Run) bool {
	canSetModTimeOnce.Do(func() {
		mtime1 := time.Date(2008, time.November, 18, 17, 32, 31, 0, time.UTC)
		_ = r.WriteObject(context.Background(), "time_test", "stuff", mtime1)
		obj, err := r.Fremote.NewObject(context.Background(), "time_test")
		require.NoError(t, err)
		mtime2 := time.Date(2009, time.November, 18, 17, 32, 31, 0, time.UTC)
		err = obj.SetModTime(context.Background(), mtime2)
		switch err {
		case nil:
			canSetModTimeValue = true
		case fs.ErrorCantSetModTime, fs.ErrorCantSetModTimeWithoutDelete:
			canSetModTimeValue = false
		default:
			require.NoError(t, err)
		}
		require.NoError(t, obj.Remove(context.Background()))
		fs.Debugf(nil, "Can set mod time: %v", canSetModTimeValue)
	})
	return canSetModTimeValue
}

// tests mod time on open files
func TestWriteFileModTimeWithOpenWriters(t *testing.T) {
	r, vfs, fh := writeHandleCreate(t)

	if !canSetModTime(t, r) {
		t.Skip("can't set mod time")
	}

	mtime := time.Date(2012, time.November, 18, 17, 32, 31, 0, time.UTC)

	_, err := fh.Write([]byte{104, 105})
	require.NoError(t, err)

	err = fh.Node().SetModTime(mtime)
	require.NoError(t, err)

	err = fh.Close()
	require.NoError(t, err)

	info, err := vfs.Stat("file1")
	require.NoError(t, err)

	if r.Fremote.Precision() != fs.ModTimeNotSupported {
		// avoid errors because of timezone differences
		assert.Equal(t, info.ModTime().Unix(), mtime.Unix())
	}
}

func testFileReadAt(t *testing.T, n int) {
	_, vfs, fh := writeHandleCreate(t)

	contents := []byte(random.String(n))
	if n != 0 {
		written, err := fh.Write(contents)
		require.NoError(t, err)
		assert.Equal(t, n, written)
	}

	// Close the file without writing to it if n==0
	err := fh.Close()
	if errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
		t.Logf("skipping test: %v", err)
		return
	}
	assert.NoError(t, err)

	// read the file back in using ReadAt into a buffer
	// this simulates what mount does
	rd, err := vfs.OpenFile("file1", os.O_RDONLY, 0)
	require.NoError(t, err)

	buf := make([]byte, 1024)
	read, err := rd.ReadAt(buf, 0)
	if err != io.EOF {
		assert.NoError(t, err)
	}
	assert.Equal(t, read, n)
	assert.Equal(t, contents, buf[:read])

	err = rd.Close()
	assert.NoError(t, err)
}

func TestFileReadAtZeroLength(t *testing.T) {
	testFileReadAt(t, 0)
}

func TestFileReadAtNonZeroLength(t *testing.T) {
	testFileReadAt(t, 100)
}

// stallingFs wraps an Fs so that streaming uploads accept a little data and
// then stall until their context is cancelled, like a remote that has
// stopped accepting data.
type stallingFs struct {
	fs.Fs
	stalled  chan struct{} // closed once the upload has stalled
	features *fs.Features
}

func newStallingFs(base fs.Fs) *stallingFs {
	f := &stallingFs{Fs: base, stalled: make(chan struct{})}
	features := *base.Features()
	features.PutStream = f.PutStream
	f.features = &features
	return f
}

// Features returns the optional features of this Fs
func (f *stallingFs) Features() *fs.Features { return f.features }

// PutStream reads a little of in then stalls until ctx is cancelled.
func (f *stallingFs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	buf := make([]byte, 1024)
	_, _ = in.Read(buf)
	close(f.stalled)
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestWriteFileHandleCloseWithErrorStalled checks that CloseWithError
// interrupts an upload the backend has stopped accepting data for. The
// writer is blocked in the pipe holding the handle's lock, so the abandon
// must fail the upload to unblock it, rather than waiting on the lock
// until the backend gives up of its own accord.
func TestWriteFileHandleCloseWithErrorStalled(t *testing.T) {
	r := fstest.NewRun(t)
	f := newStallingFs(r.Fremote)
	// A tweaked option so this VFS is not deduplicated onto an active VFS
	// of the unwrapped remote, whose config string it shares.
	opt := vfscommon.Opt
	opt.WriteWait += fs.Duration(time.Millisecond)
	vfs := New(context.Background(), f, &opt)
	t.Cleanup(vfs.Shutdown)

	h, err := vfs.OpenFile("stalled", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	fh, ok := h.(*WriteFileHandle)
	require.True(t, ok)

	// Write more than the transfer buffers absorb, so the write blocks on
	// the stalled upload while holding the handle's lock.
	writeDone := make(chan error, 1)
	go func() {
		_, err := fh.Write(make([]byte, 20<<20))
		writeDone <- err
	}()

	select {
	case <-f.stalled:
	case <-time.After(10 * time.Second):
		t.Fatal("upload never started")
	}

	// The abandon must interrupt the stalled upload promptly.
	closeDone := make(chan error, 1)
	go func() { closeDone <- fh.CloseWithError(errors.New("abandoned")) }()
	select {
	case err := <-closeDone:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("CloseWithError did not interrupt the stalled upload")
	}
	select {
	case err := <-writeDone:
		require.Error(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the blocked write was never unblocked")
	}
}
