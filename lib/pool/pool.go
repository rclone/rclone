// Package pool implements a memory pool similar in concept to
// sync.Pool but with more determinism.
package pool

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/lib/mmap"
)

// Pool of internal buffers
type Pool struct {
	cache      chan []byte
	bufferSize int
	timer      *time.Timer
	inUse      int32
	flushTime  time.Duration
	alloc      func(int) ([]byte, error)
	free       func([]byte) error
}

// New makes a buffer pool
//
// flushTime is the interval the buffer pools is flushed
// bufferSize is the size of the allocations
// poolSize is the maximum number of free buffers in the pool
// useMmap should be set to use mmap allocations
func New(flushTime time.Duration, bufferSize, poolSize int, useMmap bool) *Pool {
	bp := &Pool{
		cache:      make(chan []byte, poolSize),
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

	if flushTime > 0 {
		bp.timer = time.AfterFunc(flushTime, bp.Flush)
	}

	return bp
}

// Flush the entire buffer pool
func (bp *Pool) Flush() {
	for {
		select {
		case b := <-bp.cache:
			bp.freeBuffer(b)
		default:
			return
		}
	}
}

// InUse returns the approximate number of buffers in use which
// haven't been returned to the pool.
func (bp *Pool) InUse() int {
	return int(atomic.LoadInt32(&bp.inUse))
}

// starts or resets the buffer flusher timer
func (bp *Pool) kickFlusher() {
	if bp.timer != nil {
		bp.timer.Reset(bp.flushTime)
	}
}

// Get a buffer from the pool or allocate one
func (bp *Pool) Get() []byte {
	select {
	case b := <-bp.cache:
		return b
	default:
	}
	mem, err := bp.alloc(bp.bufferSize)
	if err != nil {
		log.Printf("Failed to get memory for buffer, waiting for a freed one: %v", err)
		return <-bp.cache
	}
	atomic.AddInt32(&bp.inUse, 1)
	return mem
}

// freeBuffer returns mem to the os if required
func (bp *Pool) freeBuffer(mem []byte) {
	err := bp.free(mem)
	if err != nil {
		log.Printf("Failed to free memory: %v", err)
	} else {
		atomic.AddInt32(&bp.inUse, -1)
	}
}

// Put returns the buffer to the buffer cache or frees it
//
// Note that if you try to return a buffer of the wrong size to Put it
// will panic.
func (bp *Pool) Put(mem []byte) {
	mem = mem[0:cap(mem)]
	if len(mem) != bp.bufferSize {
		panic(fmt.Sprintf("Returning buffer sized %d but expecting %d", len(mem), bp.bufferSize))
	}
	select {
	case bp.cache <- mem:
		bp.kickFlusher()
		return
	default:
	}
	bp.freeBuffer(mem)
	mem = nil
}
