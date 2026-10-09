package upstream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLister is a countFn which returns the values set and records
// how many times it was called
type testLister struct {
	mu      sync.Mutex
	objects int64
	size    int64
	err     error
	calls   atomic.Int32
	block   chan struct{} // if set, listing waits for this to be closed
}

func (l *testLister) list(ctx context.Context) (int64, int64, error) {
	l.calls.Add(1)
	l.mu.Lock()
	block := l.block
	l.mu.Unlock()
	if block != nil {
		<-block
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.objects, l.size, l.err
}

func (l *testLister) set(objects, size int64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.objects, l.size, l.err = objects, size, err
}

func (l *testLister) setBlock(block chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.block = block
}

// waitIdle waits for the listing in progress in c to finish
func waitIdle(t *testing.T, c *listCounter) {
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.running == nil
	}, 5*time.Second, time.Millisecond)
}

func TestListCounterSingleListing(t *testing.T) {
	l := &testLister{objects: 10, size: 1000}
	block := make(chan struct{})
	l.setBlock(block)
	c := newListCounter("test", l.list, time.Hour)

	// Lots of concurrent callers should only cause one listing
	const n = 10
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			objects, size, err := c.get()
			assert.NoError(t, err)
			assert.Equal(t, int64(10), objects)
			assert.Equal(t, int64(1000), size)
		})
	}
	require.Eventually(t, func() bool { return l.calls.Load() == 1 }, 5*time.Second, time.Millisecond)
	close(block)
	wg.Wait()
	assert.Equal(t, int32(1), l.calls.Load())

	// The cached value is used from now on
	objects, _, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(10), objects)
	assert.Equal(t, int32(1), l.calls.Load())
}

func TestListCounterAdd(t *testing.T) {
	l := &testLister{objects: 10, size: 1000}
	c := newListCounter("test", l.list, time.Hour)

	// Adding before the first listing does nothing
	c.add(5, 500)
	objects, size, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(10), objects)
	assert.Equal(t, int64(1000), size)

	c.add(2, 200)
	c.add(-1, -100)
	objects, size, err = c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(11), objects)
	assert.Equal(t, int64(1100), size)

	// Never goes negative
	c.add(-100, -100000)
	objects, size, err = c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(0), objects)
	assert.Equal(t, int64(0), size)
	assert.Equal(t, int32(1), l.calls.Load())
}

func TestListCounterRefreshInBackground(t *testing.T) {
	l := &testLister{objects: 10, size: 1000}
	c := newListCounter("test", l.list, time.Hour)
	_, _, err := c.get()
	require.NoError(t, err)

	// Expire the result and block the next listing
	block := make(chan struct{})
	l.setBlock(block)
	l.set(20, 2000, nil)
	c.markStale()

	// The old value is returned straight away and a listing is started
	objects, _, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(10), objects)
	require.Eventually(t, func() bool { return l.calls.Load() == 2 }, 5*time.Second, time.Millisecond)

	// Changes made during the listing are kept once it finishes
	c.add(1, 100)
	objects, _, err = c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(11), objects)
	assert.Equal(t, int32(2), l.calls.Load())

	close(block)
	waitIdle(t, c)
	objects, size, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(21), objects)
	assert.Equal(t, int64(2100), size)
	assert.Equal(t, int32(2), l.calls.Load())
}

func TestListCounterExpiry(t *testing.T) {
	l := &testLister{objects: 10}
	c := newListCounter("test", l.list, 0)
	_, _, err := c.get()
	require.NoError(t, err)

	// With a 0 interval every get refreshes in the background
	l.set(20, 0, nil)
	objects, _, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(10), objects)
	waitIdle(t, c)
	objects, _, err = c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(20), objects)
	waitIdle(t, c)
	assert.Equal(t, int32(3), l.calls.Load())
}

func TestListCounterNeverExpires(t *testing.T) {
	l := &testLister{objects: 10}
	c := newListCounter("test", l.list, -1)
	for range 3 {
		objects, _, err := c.get()
		require.NoError(t, err)
		assert.Equal(t, int64(10), objects)
	}
	assert.Equal(t, int32(1), l.calls.Load())

	// But it is listed again if marked stale
	l.set(20, 0, nil)
	c.markStale()
	_, _, _ = c.get()
	waitIdle(t, c)
	objects, _, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(20), objects)
	assert.Equal(t, int32(2), l.calls.Load())
}

func TestListCounterError(t *testing.T) {
	errList := errors.New("list failed")
	l := &testLister{err: errList}
	c := newListCounter("test", l.list, time.Hour)

	// The error is returned and not retried straight away
	for range 3 {
		_, _, err := c.get()
		assert.Equal(t, errList, err)
	}
	assert.Equal(t, int32(1), l.calls.Load())

	// It is retried after the interval
	l.set(10, 1000, nil)
	c.mu.Lock()
	c.expiry = time.Now().Add(-time.Second)
	c.mu.Unlock()
	objects, _, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(10), objects)
	assert.Equal(t, int32(2), l.calls.Load())

	// A failed refresh keeps the old value
	l.set(0, 0, errList)
	c.markStale()
	_, _, _ = c.get()
	waitIdle(t, c)
	objects, _, err = c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(10), objects)
	assert.Equal(t, int32(3), l.calls.Load())
}

func TestListCounterDirNotFound(t *testing.T) {
	l := &testLister{err: fs.ErrorDirNotFound}
	c := newListCounter("test", l.list, time.Hour)
	objects, size, err := c.get()
	require.NoError(t, err)
	assert.Equal(t, int64(0), objects)
	assert.Equal(t, int64(0), size)
}

func TestListUsage(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub", "subsub"), 0777))
	for _, file := range []struct {
		path string
		size int
	}{
		{"a.txt", 10},
		{"b.jpg", 20},
		{"sub/c.txt", 30},
		{"sub/subsub/d.txt", 40},
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, file.path), make([]byte, file.size), 0666))
	}
	f, err := fs.NewFs(ctx, dir)
	require.NoError(t, err)

	// The filters and depth limit of the command being run should be ignored
	ctx, ci := fs.AddConfig(ctx)
	ci.MaxDepth = 1
	ci.NoTraverse = true
	ctx, fi := filter.AddConfig(ctx)
	require.NoError(t, fi.AddFile("b.jpg"))

	objects, size, err := listUsage(ctx, f)
	require.NoError(t, err)
	assert.Equal(t, int64(4), objects)
	assert.Equal(t, int64(100), size)
}
