package sync

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
)

func TestPipe(t *testing.T) {
	var queueLength int
	var queueSize int64
	stats := func(n int, size int64) {
		queueLength, queueSize = n, size
	}

	// Make a new pipe
	p := newPipe(stats, 10)

	checkStats := func(expectedN int, expectedSize int64) {
		n, size := p.Stats()
		assert.Equal(t, expectedN, n)
		assert.Equal(t, expectedSize, size)
		assert.Equal(t, expectedN, queueLength)
		assert.Equal(t, expectedSize, queueSize)
	}

	checkStats(0, 0)

	ctx := context.Background()

	obj1 := mockobject.New("potato").WithContent([]byte("hello"), mockobject.SeekModeNone)

	pair1 := fs.ObjectPair{Src: obj1, Dst: nil}

	// Put an object
	ok := p.Put(ctx, pair1)
	assert.Equal(t, true, ok)
	checkStats(1, 5)

	// Close the pipe showing reading on closed pipe is OK
	p.Close()

	// Read from pipe
	pair2, ok := p.Get(ctx)
	assert.Equal(t, pair1, pair2)
	assert.Equal(t, true, ok)
	checkStats(0, 0)

	// Check read on closed pipe
	pair2, ok = p.Get(ctx)
	assert.Equal(t, fs.ObjectPair{}, pair2)
	assert.Equal(t, false, ok)

	// Check panic on write to closed pipe
	assert.Panics(t, func() { p.Put(ctx, pair1) })

	// Make a new pipe
	p = newPipe(stats, 10)
	ctx2, cancel := context.WithCancel(ctx)

	// cancel it in the background - check read ceases
	go cancel()
	pair2, ok = p.Get(ctx2)
	assert.Equal(t, fs.ObjectPair{}, pair2)
	assert.Equal(t, false, ok)

	// check we can't write
	ok = p.Put(ctx2, pair1)
	assert.Equal(t, false, ok)

}

// TestPipeConcurrent runs concurrent Get and Put to flush out any
// race conditions and concurrency problems.
func TestPipeConcurrent(t *testing.T) {
	const (
		N           = 1000
		readWriters = 10
	)

	stats := func(n int, size int64) {}

	// Make a new pipe
	p := newPipe(stats, 10)

	var wg sync.WaitGroup
	obj1 := mockobject.New("potato").WithContent([]byte("hello"), mockobject.SeekModeNone)
	pair1 := fs.ObjectPair{Src: obj1, Dst: nil}
	ctx := context.Background()
	var count int64

	for j := 0; j < readWriters; j++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < N; i++ {
				// Read from pipe
				pair2, ok := p.Get(ctx)
				assert.Equal(t, pair1, pair2)
				assert.Equal(t, true, ok)
				atomic.AddInt64(&count, -1)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < N; i++ {
				// Put an object
				ok := p.Put(ctx, pair1)
				assert.Equal(t, true, ok)
				atomic.AddInt64(&count, 1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), count)
}
