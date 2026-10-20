package downloaders

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/ranges"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

type testItem struct {
	mu   sync.Mutex
	t    *testing.T
	rs   ranges.Ranges
	size int64
}

// HasRange returns true if the current ranges entirely include range
func (item *testItem) HasRange(r ranges.Range) bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.rs.Present(r)
}

// FindMissing adjusts r returning a new ranges.Range which only
// contains the range which needs to be downloaded. This could be
// empty - check with IsEmpty. It also adjust this to make sure it is
// not larger than the file.
func (item *testItem) FindMissing(r ranges.Range) (outr ranges.Range) {
	item.mu.Lock()
	defer item.mu.Unlock()
	outr = item.rs.FindMissing(r)
	// Clip returned block to size of file
	outr.Clip(item.size)
	return outr
}

// WriteAtNoOverwrite writes b to the file, but will not overwrite
// already present ranges.
//
// This is used by the downloader to write bytes to the file.
//
// It returns n the total bytes processed and skipped the number of
// bytes which were processed but not actually written to the file.
func (item *testItem) WriteAtNoOverwrite(b []byte, off int64) (n int, skipped int, err error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	item.rs.Insert(ranges.Range{Pos: off, Size: int64(len(b))})

	// Check contents is correct
	in := readers.NewPatternReader(item.size)
	checkBuf := make([]byte, len(b))
	_, err = in.Seek(off, io.SeekStart)
	require.NoError(item.t, err)
	n, _ = in.Read(checkBuf)
	require.Equal(item.t, len(b), n)
	assert.Equal(item.t, checkBuf, b)

	return n, 0, nil
}

func TestDownloaders(t *testing.T) {
	r := fstest.NewRun(t)

	var (
		ctx    = context.Background()
		remote = "potato.txt"
		size   = int64(50*1024*1024 - 1234)
	)

	// Write the test file
	in := io.NopCloser(readers.NewPatternReader(size))
	src, err := operations.RcatSize(ctx, r.Fremote, remote, in, size, time.Now(), nil)
	require.NoError(t, err)
	assert.Equal(t, size, src.Size())

	newTest := func() (*testItem, *Downloaders) {
		item := &testItem{
			t:    t,
			size: size,
		}
		opt := vfscommon.Opt
		dls := New(ctx, item, &opt, remote, src)
		return item, dls
	}
	cancel := func(dls *Downloaders) {
		assert.NoError(t, dls.Close(nil))
	}

	t.Run("Download", func(t *testing.T) {
		item, dls := newTest()
		defer cancel(dls)

		for _, r := range []ranges.Range{
			{Pos: 100, Size: 250},
			{Pos: 500, Size: 250},
			{Pos: 25000000, Size: 250},
		} {
			err := dls.Download(r)
			require.NoError(t, err)
			assert.True(t, item.HasRange(r))
		}
	})

	t.Run("EnsureDownloader", func(t *testing.T) {
		item, dls := newTest()
		defer cancel(dls)
		r := ranges.Range{Pos: 40 * 1024 * 1024, Size: 250}
		err := dls.EnsureDownloader(r)
		require.NoError(t, err)
		assert.Eventually(t, func() bool {
			return item.HasRange(r)
		}, 10*time.Second, 10*time.Millisecond)
	})

	// A waiter for a range beyond the item's size but within the source
	// object's size must still be dispatched. _ensureDownloader will not
	// start a downloader for it, so nothing else would ever wake it.
	t.Run("DownloadBeyondItemSize", func(t *testing.T) {
		item := &testItem{
			t:    t,
			size: 1024 * 1024,
		}
		opt := vfscommon.Opt
		dls := New(ctx, item, &opt, remote, src)
		defer cancel(dls)

		done := make(chan error, 1)
		go func() {
			done <- dls.Download(ranges.Range{Pos: 40 * 1024 * 1024, Size: 250})
		}()

		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(30 * time.Second):
			t.Fatal("Download did not return: the waiter was never dispatched")
		}
	})

	// A successful write must not be turned into a failure by a stale
	// error left over from a previous problem (e.g. the cache running
	// out of space earlier). Regression test: this used to make Write
	// return dls.lastErr even when the write itself succeeded, which
	// then got wrapped again by download() and stored back as the new
	// lastErr - causing the wrapped error message to grow without bound
	// on every subsequent write.
	t.Run("WriteDoesNotFailOnStaleError", func(t *testing.T) {
		item, dls := newTest()
		defer cancel(dls)

		dls.mu.Lock()
		dls.errorCount = maxErrorCount + 1
		dls.lastErr = fmt.Errorf("vfs reader: failed to write to cache file: %w", errors.New("no space left on device"))
		// A waiter for a range that isn't downloaded yet, so kickWaiters
		// doesn't dispatch it immediately and reaches the errorCount check.
		dls.waiters = append(dls.waiters, waiter{
			r:       ranges.Range{Pos: 10 * 1024 * 1024, Size: 250},
			errChan: make(chan error, 1),
		})
		dls.mu.Unlock()

		dl := &downloader{
			dls:       dls,
			quit:      make(chan struct{}),
			kick:      make(chan struct{}, 1),
			offset:    0,
			maxOffset: 1024,
		}

		p := make([]byte, 16)
		_, err := io.ReadFull(readers.NewPatternReader(item.size), p)
		require.NoError(t, err)

		n, err := dl.Write(p)
		require.NoError(t, err, "a successful write must not fail due to unrelated stale state")
		assert.Equal(t, len(p), n)
	})
}
