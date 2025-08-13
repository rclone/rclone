// Package pool implements a memory pool similar in concept to
// sync.Pool but with more determinism.
package pool

import (
	"context"
	"fmt"
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
// will not be controlled.
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

// put puts the buffer on the end of bp.cache
//
// Call with mu held
func (bp *Pool) put(buf []byte) {
	bp.cache = append(bp.cache, buf)
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

// acquire mem bytes of memory
func (bp *Pool) acquire(mem int64) error {
	if totalMemory == nil {
		return nil
	}
	ctx := context.Background()
	return totalMemory.Acquire(ctx, mem)
}

// release mem bytes of memory
func (bp *Pool) release(mem int64) {
	if totalMemory == nil {
		return
	}
	totalMemory.Release(mem)
}

// Reserve buffers for use. Blocks until they are free.
//
// Doesn't allocate any memory.
//
// Must be released by calling GetReserved() which releases 1 buffer or
// Release() to release any number of buffers.
func (bp *Pool) Reserve(buffers int) {
	waitTime := time.Millisecond
	for {
		err := bp.acquire(int64(buffers) * int64(bp.bufferSize))
		if err == nil {
			break
		}
		fs.Logf(nil, "Failed to get reservation for buffer, waiting for %v: %v", waitTime, err)
		time.Sleep(waitTime)
		waitTime *= 2
	}
}

// Release previously Reserved buffers.
//
// Doesn't free any memory.
func (bp *Pool) Release(buffers int) {
	bp.release(int64(buffers) * int64(bp.bufferSize))
}

// Get a buffer from the pool or allocate one
func (bp *Pool) getBlock(reserved bool) []byte {
	bp.mu.Lock()
	var buf []byte
	waitTime := time.Millisecond
	for {
		if len(bp.cache) > 0 {
			buf = bp.get()
			if reserved {
				// If got reserved memory from the cache we
				// can release the reservation of one buffer.
				bp.release(int64(bp.bufferSize))
			}
			break
		} else {
			var err error
			if !reserved {
				bp.mu.Unlock()
				err = bp.acquire(int64(bp.bufferSize))
				bp.mu.Lock()
			}
			if err == nil {
				buf, err = bp.alloc(bp.bufferSize)
				if err == nil {
					bp.alloced++
					break
				}
				if !reserved {
					bp.release(int64(bp.bufferSize))
				}
			}
			fs.Logf(nil, "Failed to get memory for buffer, waiting for %v: %v", waitTime, err)
			bp.mu.Unlock()
			time.Sleep(waitTime)
			bp.mu.Lock()
			waitTime *= 2
		}
	}
	bp.inUse++
	bp.updateMinFill()
	bp.mu.Unlock()
	return buf
}

// Get a buffer from the pool or allocate one
func (bp *Pool) Get() []byte {
	return bp.getBlock(false)
}

// GetReserved gets a reserved buffer from the pool or allocates one.
func (bp *Pool) GetReserved() []byte {
	return bp.getBlock(true)
}

// freeBuffer returns mem to the os if required - call with lock held
func (bp *Pool) freeBuffer(mem []byte) {
	err := bp.free(mem)
	if err != nil {
		fs.Logf(nil, "Failed to free memory: %v", err)
	} else {
		bp.release(int64(bp.bufferSize))
	}
	bp.alloced--
}

// Put returns the buffer to the buffer cache or frees it
//
// Note that if you try to return a buffer of the wrong size to Put it
// will panic.
func (bp *Pool) Put(buf []byte) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	buf = buf[0:cap(buf)]
	if len(buf) != bp.bufferSize {
		panic(fmt.Sprintf("Returning buffer sized %d but expecting %d", len(buf), bp.bufferSize))
	}
	if len(bp.cache) < bp.poolSize {
		bp.put(buf)
	} else {
		bp.freeBuffer(buf)
	}
	bp.inUse--
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
