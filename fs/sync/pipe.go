package sync

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs"
)

// pipe provides an unbounded channel like experience
//
// Note unlike channels these aren't strictly ordered.
type pipe struct {
	mu        sync.Mutex
	c         chan struct{}
	queue     []fs.ObjectPair
	closed    bool
	totalSize int64
	stats     func(items int, totalSize int64)
}

func newPipe(stats func(items int, totalSize int64), maxBacklog int) *pipe {
	return &pipe{
		c:     make(chan struct{}, maxBacklog),
		stats: stats,
	}
}

// Put an pair into the pipe
//
// It returns ok = false if the context was cancelled
//
// It will panic if you call it after Close()
func (p *pipe) Put(ctx context.Context, pair fs.ObjectPair) (ok bool) {
	if ctx.Err() != nil {
		return false
	}
	p.mu.Lock()
	p.queue = append(p.queue, pair)
	size := pair.Src.Size()
	if size > 0 {
		p.totalSize += size
	}
	p.stats(len(p.queue), p.totalSize)
	p.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case p.c <- struct{}{}:
	}
	return true
}

// Get a pair from the pipe
//
// It returns ok = false if the context was cancelled or Close() has
// been called.
func (p *pipe) Get(ctx context.Context) (pair fs.ObjectPair, ok bool) {
	if ctx.Err() != nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	case _, ok = <-p.c:
		if !ok {
			return
		}
	}
	p.mu.Lock()
	pair = p.queue[0]
	p.queue[0].Src = nil
	p.queue[0].Dst = nil
	p.queue = p.queue[1:]
	size := pair.Src.Size()
	if size > 0 {
		p.totalSize -= size
	}
	if p.totalSize < 0 {
		p.totalSize = 0
	}
	p.stats(len(p.queue), p.totalSize)
	p.mu.Unlock()
	return pair, true
}

// Stats reads the number of items in the queue and the totalSize
func (p *pipe) Stats() (items int, totalSize int64) {
	p.mu.Lock()
	items, totalSize = len(p.queue), p.totalSize
	p.mu.Unlock()
	return items, totalSize
}

// Close the pipe
//
// Writes to a closed pipe will panic as will double closing a pipe
func (p *pipe) Close() {
	p.mu.Lock()
	close(p.c)
	p.closed = true
	p.mu.Unlock()
}
