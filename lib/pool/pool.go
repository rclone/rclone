// Package pool implements a memory pool similar in concept to
// sync.Pool but with more determinism.
package pool

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ncw/rclone/lib/mmap"
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
	for i := 0; i < n; i++ {
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

// Get a buffer from the pool or allocate one
func (bp *Pool) Get() []byte {
	bp.mu.Lock()
	var buf []byte
	waitTime := time.Millisecond
	for {
		if len(bp.cache) > 0 {
			buf = bp.get()
			break
		} else {
			var err error
			buf, err = bp.alloc(bp.bufferSize)
			if err == nil {
				bp.alloced++
				break
			}
			log.Printf("Failed to get memory for buffer, waiting for %v: %v", waitTime, err)
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

// freeBuffer returns mem to the os if required - call with lock held
func (bp *Pool) freeBuffer(mem []byte) {
	err := bp.free(mem)
	if err != nil {
		log.Printf("Failed to free memory: %v", err)
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
