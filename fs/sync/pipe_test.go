package sync

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	}{
		{"", false, true},
		{"size", false, false},
		{"name", true, true},
		{"modtime", false, true},
		{"size,ascending", false, false},
		{"name,asc", true, true},
		{"modtime,ascending", false, true},
		{"size,descending", true, true},
		{"name,desc", false, false},
		{"modtime,descending", true, false},
	} {
		t.Run(test.orderBy, func(t *testing.T) {
			p, err := newPipe(test.orderBy, stats, 10)
			require.NoError(t, err)

			ok := p.Put(ctx, pair1)
			assert.True(t, ok)
			ok = p.Put(ctx, pair2)
			assert.True(t, ok)

			readAndCheck := func(swapped bool) {
				readFirst, ok := p.Get(ctx)
				assert.True(t, ok)
				readSecond, ok := p.Get(ctx)
				assert.True(t, ok)

				if swapped {
					assert.True(t, readFirst == pair2 && readSecond == pair1)
				} else {
					assert.True(t, readFirst == pair1 && readSecond == pair2)
				}
			}

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
		less, err := newLess("")
		require.NoError(t, err)
		assert.Nil(t, less)
	})

	t.Run("tooManyParts", func(t *testing.T) {
		_, err := newLess("too,many,parts")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad --order-by string")
	})

	t.Run("unknownComparison", func(t *testing.T) {
		_, err := newLess("potato")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown --order-by comparison")
	})

	t.Run("unknownSortDirection", func(t *testing.T) {
		_, err := newLess("name,sideways")
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
	}{
		{"size", true, false},
		{"name", false, true},
		{"modtime", false, false},
		{"size,ascending", true, false},
		{"name,asc", false, true},
		{"modtime,ascending", false, false},
		{"size,descending", false, true},
		{"name,desc", true, false},
		{"modtime,descending", true, true},
	} {
		t.Run(test.orderBy, func(t *testing.T) {
			less, err := newLess(test.orderBy)
			require.NoError(t, err)
			require.NotNil(t, less)
			pair1LessPair2 := less(pair1, pair2)
			assert.Equal(t, test.pair1LessPair2, pair1LessPair2)
			pair2LessPair1 := less(pair2, pair1)
			assert.Equal(t, test.pair2LessPair1, pair2LessPair1)
		})
	}

}
