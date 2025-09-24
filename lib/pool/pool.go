// Package pool implements a memory pool similar in concept to
// sync.Pool but with more determinism.
package pool

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/mmap"
	"golang.org/x/sync/semaphore"
)

const (
	// BufferSize is the page size of the Global() pool
	BufferSize = 1024 * 1024
	// BufferCacheSize is the max number of buffers to keep in the cache for the Global() pool
	BufferCacheSize = 64
	// BufferCacheFlushTime is the max time to keep buffers in the Global() pool
	BufferCacheFlushTime = 5 * time.Second
)

// Pool of internal buffers
//
// We hold buffers in cache. Every time we Get or Put we update
// minFill which is the minimum len(cache) seen.
//
// Every flushTime we remove minFill buffers from the cache as they
// were not used in the previous flushTime interval.
type Pool struct {
	mu           sync.Mutex
	cache        [][]byte
	minFill      int // the minimum fill of the cache
	bufferSize   int
	poolSize     int
	timer        *time.Timer
	inUse        int
	alloced      int
	flushTime    time.Duration
	flushPending bool
	alloc        func(int) ([]byte, error)
	free         func([]byte) error
}

// totalMemory is a semaphore used to control total buffer usage of
// all Pools. It it may be nil in which case the total buffer usage
// will not be controlled. It counts memory in active use, it does not
// count memory cached in the pool.
var totalMemory *semaphore.Weighted

// Make sure we initialise the totalMemory semaphore once
var totalMemoryInit sync.Once

// New makes a buffer pool
//
// flushTime is the interval the buffer pools is flushed
// bufferSize is the size of the allocations
// poolSize is the maximum number of free buffers in the pool
// useMmap should be set to use mmap allocations
func New(flushTime time.Duration, bufferSize, poolSize int, useMmap bool) *Pool {
	bp := &Pool{
		cache:      make([][]byte, 0, poolSize),
		poolSize:   poolSize,
		flushTime:  flushTime,
		bufferSize: bufferSize,
	}
	if useMmap {
		bp.alloc = mmap.Alloc
		bp.free = mmap.Free
	} else {
		bp.alloc = func(size int) ([]byte, error) {
			return make([]byte, size), nil
		}
		bp.free = func([]byte) error {
			return nil
		}
	}

	// Initialise total memory limit if required
	totalMemoryInit.Do(func() {
		ci := fs.GetConfig(context.Background())

		// Set max buffer memory limiter
		if ci.MaxBufferMemory > 0 {
			totalMemory = semaphore.NewWeighted(int64(ci.MaxBufferMemory))
		}
	})

	bp.timer = time.AfterFunc(flushTime, bp.flushAged)
	return bp
}

// get gets the last buffer in bp.cache
//
// Call with mu held
func (bp *Pool) get() []byte {
	n := len(bp.cache) - 1
	buf := bp.cache[n]
	bp.cache[n] = nil // clear buffer pointer from bp.cache
	bp.cache = bp.cache[:n]
	return buf
}

// getN gets the last n buffers in bp.cache
//
// will panic if you ask for too many buffers
//
// Call with mu held
func (bp *Pool) getN(n int) [][]byte {
	i := len(bp.cache) - n
	bufs := slices.Clone(bp.cache[i:])
	bp.cache = slices.Delete(bp.cache, i, len(bp.cache))
	return bufs
}

// put puts the buffer on the end of bp.cache
//
// Call with mu held
func (bp *Pool) put(buf []byte) {
	bp.cache = append(bp.cache, buf)
}

// put puts the bufs on the end of bp.cache
//
// Call with mu held
func (bp *Pool) putN(bufs [][]byte) {
	bp.cache = append(bp.cache, bufs...)
}

// buffers returns the number of buffers in bp.ache
//
// Call with mu held
func (bp *Pool) buffers() int {
	return len(bp.cache)
}

// flush n entries from the entire buffer pool
// Call with mu held
func (bp *Pool) flush(n int) {
	for range n {
		bp.freeBuffer(bp.get())
	}
	bp.minFill = len(bp.cache)
}

// Flush the entire buffer pool
func (bp *Pool) Flush() {
	bp.mu.Lock()
	bp.flush(len(bp.cache))
	bp.mu.Unlock()
}

// Remove bp.minFill buffers
func (bp *Pool) flushAged() {
	bp.mu.Lock()
	bp.flushPending = false
	bp.flush(bp.minFill)
	// If there are still items in the cache, schedule another flush
	if len(bp.cache) != 0 {
		bp.kickFlusher()
	}
	bp.mu.Unlock()
}

// InUse returns the number of buffers in use which haven't been
// returned to the pool
func (bp *Pool) InUse() int {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.inUse
}

// InPool returns the number of buffers in the pool
func (bp *Pool) InPool() int {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return len(bp.cache)
}

// Alloced returns the number of buffers allocated and not yet freed
func (bp *Pool) Alloced() int {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.alloced
}

// starts or resets the buffer flusher timer - call with mu held
func (bp *Pool) kickFlusher() {
	if bp.flushPending {
		return
	}
	bp.flushPending = true
	bp.timer.Reset(bp.flushTime)
}

// Make sure minFill is correct - call with mu held
func (bp *Pool) updateMinFill() {
	if len(bp.cache) < bp.minFill {
		bp.minFill = len(bp.cache)
	}
}

// acquire mem bytes of memory for the user
func (bp *Pool) acquire(mem int64) error {
	if totalMemory == nil {
		return nil
	}
	ctx := context.Background()
	return totalMemory.Acquire(ctx, mem)
}

// release mem bytes of memory from the user
func (bp *Pool) release(mem int64) {
	if totalMemory == nil {
		return
	}
	totalMemory.Release(mem)
}

// Get a buffer from the pool or allocate one
func (bp *Pool) Get() []byte {
	return bp.GetN(1)[0]
}

// GetN get n buffers atomically from the pool or allocate them
func (bp *Pool) GetN(n int) [][]byte {
	bp.mu.Lock()
	var (
		waitTime = time.Millisecond // retry time if allocation failed
		err      error              // allocation error
		buf      []byte             // allocated buffer
		bufs     [][]byte           // bufs so far
		have     int                // have this many buffers in bp.cache
		want     int                // want this many extra buffers
		acquired bool               // whether we have acquired the memory or not
	)
	for {
		acquired = false
		bp.mu.Unlock()
		err = bp.acquire(int64(bp.bufferSize) * int64(n))
		bp.mu.Lock()
		if err != nil {
			goto FAIL
		}
		acquired = true
		have = min(bp.buffers(), n)
		want = n - have
		bufs = bp.getN(have) // get as many buffers as we have from the cache
		for range want {
			buf, err = bp.alloc(bp.bufferSize)
			if err != nil {
				goto FAIL
			}
			bp.alloced++
			bufs = append(bufs, buf)
		}
		break
	FAIL:
		// Release the buffers and the allocation if it succeeded
		bp.putN(bufs)
		if acquired {
			bp.release(int64(bp.bufferSize) * int64(n))
		}
		fs.Logf(nil, "Failed to get memory for buffer, waiting for %v: %v", waitTime, err)
		bp.mu.Unlock()
		time.Sleep(waitTime)
		bp.mu.Lock()
		waitTime *= 2
		clear(bufs)
		bufs = nil
	}
	bp.inUse += n
	bp.updateMinFill()
	bp.mu.Unlock()
	return bufs
}

// freeBuffer returns mem to the os if required - call with lock held
func (bp *Pool) freeBuffer(mem []byte) {
	err := bp.free(mem)
	if err != nil {
		fs.Logf(nil, "Failed to free memory: %v", err)
	}
	bp.alloced--
}

// _put returns the buffer to the buffer cache or frees it
//
// call with lock held
//
// Note that if you try to return a buffer of the wrong size it will
// panic.
func (bp *Pool) _put(buf []byte) {
	buf = buf[0:cap(buf)]
	if len(buf) != bp.bufferSize {
		panic(fmt.Sprintf("Returning buffer sized %d but expecting %d", len(buf), bp.bufferSize))
	}
	if len(bp.cache) < bp.poolSize {
		bp.put(buf)
	} else {
		bp.freeBuffer(buf)
	}
	bp.release(int64(bp.bufferSize))
}

// Put returns the buffer to the buffer cache or frees it
//
// Note that if you try to return a buffer of the wrong size to Put it
// will panic.
func (bp *Pool) Put(buf []byte) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp._put(buf)
	bp.inUse--
	bp.updateMinFill()
	bp.kickFlusher()
}

// PutN returns the buffers to the buffer cache or frees it,
//
// Note that if you try to return a buffer of the wrong size to PutN it
// will panic.
func (bp *Pool) PutN(bufs [][]byte) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	for _, buf := range bufs {
		bp._put(buf)
	}
	bp.inUse -= len(bufs)
	bp.updateMinFill()
	bp.kickFlusher()
}

// bufferPool is a global pool of buffers
var bufferPool *Pool
var bufferPoolOnce sync.Once

// Global gets a global pool of BufferSize, BufferCacheSize, BufferCacheFlushTime.
func Global() *Pool {
	bufferPoolOnce.Do(func() {
		// Initialise the buffer pool when used
		ci := fs.GetConfig(context.Background())
		bufferPool = New(BufferCacheFlushTime, BufferSize, BufferCacheSize, ci.UseMmap)
	})
	return bufferPool
}
