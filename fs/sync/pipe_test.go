package sync

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check interface satisfied
var _ heap.Interface = (*pipe)(nil)

func TestPipe(t *testing.T) {
	var queueLength int
	var queueSize int64
	stats := func(n int, size int64) {
		queueLength, queueSize = n, size
	}

	// Make a new pipe
	p, err := newPipe("", stats, 10)
	require.NoError(t, err)

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
	pairD := fs.ObjectPair{Src: obj1, Dst: obj1} // this object should not count to the stats

	// Put an object
	ok := p.Put(ctx, pair1)
	assert.Equal(t, true, ok)
	checkStats(1, 5)

	// Put an object to be deleted
	ok = p.Put(ctx, pairD)
	assert.Equal(t, true, ok)
	checkStats(2, 5)

	// Close the pipe showing reading on closed pipe is OK
	p.Close()

	// Read from pipe
	pair2, ok := p.Get(ctx)
	assert.Equal(t, pair1, pair2)
	assert.Equal(t, true, ok)
	checkStats(1, 0)

	// Read from pipe
	pair2, ok = p.Get(ctx)
	assert.Equal(t, pairD, pair2)
	assert.Equal(t, true, ok)
	checkStats(0, 0)

	// Check read on closed pipe
	pair2, ok = p.Get(ctx)
	assert.Equal(t, fs.ObjectPair{}, pair2)
	assert.Equal(t, false, ok)

	// Check panic on write to closed pipe
	assert.Panics(t, func() { p.Put(ctx, pair1) })

	// Make a new pipe
	p, err = newPipe("", stats, 10)
	require.NoError(t, err)
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
	p, err := newPipe("", stats, 10)
	require.NoError(t, err)

	var wg sync.WaitGroup
	obj1 := mockobject.New("potato").WithContent([]byte("hello"), mockobject.SeekModeNone)
	pair1 := fs.ObjectPair{Src: obj1, Dst: nil}
	ctx := context.Background()
	var count atomic.Int64

	for j := 0; j < readWriters; j++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < N; i++ {
				// Read from pipe
				pair2, ok := p.Get(ctx)
				assert.Equal(t, pair1, pair2)
				assert.Equal(t, true, ok)
				count.Add(-1)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < N; i++ {
				// Put an object
				ok := p.Put(ctx, pair1)
				assert.Equal(t, true, ok)
				count.Add(1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int64(0), count.Load())
}

func TestPipeOrderBy(t *testing.T) {
	var (
		stats = func(n int, size int64) {}
		ctx   = context.Background()
		obj1  = mockobject.New("b").WithContent([]byte("1"), mockobject.SeekModeNone)
		obj2  = mockobject.New("a").WithContent([]byte("22"), mockobject.SeekModeNone)
		pair1 = fs.ObjectPair{Src: obj1}
		pair2 = fs.ObjectPair{Src: obj2}
	)

	for _, test := range []struct {
		orderBy  string
		swapped1 bool
		swapped2 bool
		fraction int
	}{
		{"", false, true, -1},
		{"size", false, false, -1},
		{"name", true, true, -1},
		{"modtime", false, true, -1},
		{"size,ascending", false, false, -1},
		{"name,asc", true, true, -1},
		{"modtime,ascending", false, true, -1},
		{"size,descending", true, true, -1},
		{"name,desc", false, false, -1},
		{"modtime,descending", true, false, -1},
		{"size,mixed,50", false, false, 25},
		{"size,mixed,51", true, true, 75},
	} {
		t.Run(test.orderBy, func(t *testing.T) {
			p, err := newPipe(test.orderBy, stats, 10)
			require.NoError(t, err)

			readAndCheck := func(swapped bool) {
				var readFirst, readSecond fs.ObjectPair
				var ok1, ok2 bool
				if test.fraction < 0 {
					readFirst, ok1 = p.Get(ctx)
					readSecond, ok2 = p.Get(ctx)
				} else {
					readFirst, ok1 = p.GetMax(ctx, test.fraction)
					readSecond, ok2 = p.GetMax(ctx, test.fraction)
				}
				assert.True(t, ok1)
				assert.True(t, ok2)

				if swapped {
					assert.True(t, readFirst == pair2 && readSecond == pair1)
				} else {
					assert.True(t, readFirst == pair1 && readSecond == pair2)
				}
			}

			ok := p.Put(ctx, pair1)
			assert.True(t, ok)
			ok = p.Put(ctx, pair2)
			assert.True(t, ok)

			readAndCheck(test.swapped1)

			// insert other way round

			ok = p.Put(ctx, pair2)
			assert.True(t, ok)
			ok = p.Put(ctx, pair1)
			assert.True(t, ok)

			readAndCheck(test.swapped2)
		})
	}
}

func TestNewLess(t *testing.T) {
	t.Run("blankOK", func(t *testing.T) {
		less, _, err := newLess("")
		require.NoError(t, err)
		assert.Nil(t, less)
	})

	t.Run("tooManyParts", func(t *testing.T) {
		_, _, err := newLess("size,asc,toomanyparts")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad --order-by string")
	})

	t.Run("tooManyParts2", func(t *testing.T) {
		_, _, err := newLess("size,mixed,50,toomanyparts")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad --order-by string")
	})

	t.Run("badMixed", func(t *testing.T) {
		_, _, err := newLess("size,mixed,32.7")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad mixed fraction")
	})

	t.Run("unknownComparison", func(t *testing.T) {
		_, _, err := newLess("potato")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown --order-by comparison")
	})

	t.Run("unknownSortDirection", func(t *testing.T) {
		_, _, err := newLess("name,sideways")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown --order-by sort direction")
	})

	var (
		obj1  = mockobject.New("b").WithContent([]byte("1"), mockobject.SeekModeNone)
		obj2  = mockobject.New("a").WithContent([]byte("22"), mockobject.SeekModeNone)
		pair1 = fs.ObjectPair{Src: obj1}
		pair2 = fs.ObjectPair{Src: obj2}
	)

	for _, test := range []struct {
		orderBy        string
		pair1LessPair2 bool
		pair2LessPair1 bool
		wantFraction   int
	}{
		{"size", true, false, -1},
		{"name", false, true, -1},
		{"modtime", false, false, -1},
		{"size,ascending", true, false, -1},
		{"name,asc", false, true, -1},
		{"modtime,ascending", false, false, -1},
		{"size,descending", false, true, -1},
		{"name,desc", true, false, -1},
		{"modtime,descending", true, true, -1},
		{"modtime,mixed", false, false, 50},
		{"modtime,mixed,30", false, false, 30},
	} {
		t.Run(test.orderBy, func(t *testing.T) {
			less, gotFraction, err := newLess(test.orderBy)
			assert.Equal(t, test.wantFraction, gotFraction)
			require.NoError(t, err)
			require.NotNil(t, less)
			pair1LessPair2 := less(pair1, pair2)
			assert.Equal(t, test.pair1LessPair2, pair1LessPair2)
			pair2LessPair1 := less(pair2, pair1)
			assert.Equal(t, test.pair2LessPair1, pair2LessPair1)
		})
	}

}
