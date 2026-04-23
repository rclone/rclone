package downloaders

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
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

	// poolSize returns len(dls.dls) under the mutex. Same-package test
	// only; external callers should not peek at the pool directly.
	poolSize := func(dls *Downloaders) int {
		dls.mu.Lock()
		defer dls.mu.Unlock()
		return len(dls.dls)
	}

	// observePeakPool fires n concurrent Download() calls for far-apart
	// ranges and returns the peak observed pool size. The stride is
	// deliberately larger than minWindow (1 MiB) so range-reuse never
	// folds two requests onto the same downloader.
	//
	// window bounds the worst-case wall time. target is an optimistic
	// early-exit: once peak >= target the loop waits briefly (to catch
	// a cap violation that appears just above target) and then returns
	// without burning the full window. Pass target=0 to always observe
	// for the full window.
	observePeakPool := func(t *testing.T, dls *Downloaders, n int, stride int64, window time.Duration, target int) int {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			r := ranges.Range{Pos: int64(i) * stride, Size: 1024}
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = dls.Download(r)
			}()
		}

		peak := 0
		deadline := time.Now().Add(window)
		for time.Now().Before(deadline) {
			if cur := poolSize(dls); cur > peak {
				peak = cur
			}
			if target > 0 && peak >= target {
				// Guard window: if the cap is leaky, a violation
				// typically shows up within a few polls of first
				// hitting target. Sample once more before returning.
				time.Sleep(50 * time.Millisecond)
				if cur := poolSize(dls); cur > peak {
					peak = cur
				}
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		// Don't wg.Wait here — cancel(dls) in the outer defer is what
		// unblocks Download() callers via _closeWaiters.
		t.Cleanup(wg.Wait)
		return peak
	}

	// newCapCtx returns a ctx with BufferSize pinned to 0 so the
	// reuse-window in _ensureDownloader collapses to minWindow (1 MiB).
	// Without this override the window is the global --buffer-size
	// (default 16 MiB) and our 2 MiB stride gets absorbed into a
	// single downloader, masking the pool size we want to measure.
	newCapCtx := func(parent context.Context) context.Context {
		capCtx, ci := fs.AddConfig(parent)
		ci.BufferSize = 0
		return capCtx
	}

	// Max: with DownloadersMax=4 and 32 concurrent far-apart requests,
	// the pool must never exceed 4. This is the entire behavioural
	// guarantee of the fix.
	t.Run("Max", func(t *testing.T) {
		capCtx := newCapCtx(ctx)
		item := &testItem{t: t, size: size}
		opt := vfscommon.Opt
		opt.DownloadersMax = 4
		dls := New(capCtx, item, &opt, remote, src)
		defer cancel(dls)

		peak := observePeakPool(t, dls, 32, 2*1024*1024, 2*time.Second, opt.DownloadersMax)

		assert.LessOrEqual(t, peak, opt.DownloadersMax,
			"pool exceeded DownloadersMax=%d: observed peak %d",
			opt.DownloadersMax, peak)
		assert.Equal(t, opt.DownloadersMax, peak,
			"pool never reached DownloadersMax=%d (peak %d) — cap not stressed by this workload",
			opt.DownloadersMax, peak)
	})

	// Unlimited: with DownloadersMax=0 the cap is off; the pool must be
	// able to exceed the cap's value. This is the control: if this one
	// also plateaus at 4, the Max test is measuring something other
	// than the cap (e.g. range-reuse).
	//
	// Relies on newCapCtx's BufferSize=0 to collapse the reuse-window to
	// minWindow (1 MiB); with a 2 MiB stride every valid-range request
	// then spawns its own downloader, and the 5 s idle timeout keeps
	// them alive for the observation window so the pool can grow.
	t.Run("Unlimited", func(t *testing.T) {
		capCtx := newCapCtx(ctx)
		item := &testItem{t: t, size: size}
		opt := vfscommon.Opt
		opt.DownloadersMax = 0
		dls := New(capCtx, item, &opt, remote, src)
		defer cancel(dls)

		const threshold = 5
		peak := observePeakPool(t, dls, 32, 2*1024*1024, 2*time.Second, threshold+1)

		// Clear daylight above any cap=4 value anyone might mistakenly
		// introduce later. On a 50 MiB file with a 2 MiB stride that's
		// up to ~25 live downloaders — plenty of headroom above 5.
		assert.Greater(t, peak, threshold,
			"with DownloadersMax=0 (unlimited), pool peak was expected to exceed %d, got %d",
			threshold, peak)
	})
}
