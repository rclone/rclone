// Package filepool keeps a set of reusable write handles open on a single
// remote path, one per connection, so several goroutines can write to the same
// file at once without sharing a handle.
//
// It is used by backends that implement fs.OpenWriterAter over a connection
// pool, where the core writes the chunks of a large file concurrently at
// non-overlapping offsets.
package filepool

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Pool hands out handles of type T for a single file. Handles are reused when
// free and opened on demand otherwise. It is safe for concurrent use.
//
// The zero value is not usable; call New.
type Pool[T any] struct {
	ctx     context.Context
	open    func(context.Context) (T, error)
	release func(handle T, err error) error

	mu   sync.Mutex
	free []T
}

// New returns a Pool.
//
// open opens a fresh handle on its own connection. release closes a handle and
// returns its connection: err is the error that made the handle unusable (nil
// when the handle is simply being drained) and the returned error is the result
// of closing it.
func New[T any](ctx context.Context, open func(context.Context) (T, error), release func(handle T, err error) error) *Pool[T] {
	return &Pool[T]{ctx: ctx, open: open, release: release}
}

// Get returns a free handle, opening a new one if none are free.
func (p *Pool[T]) Get() (T, error) {
	p.mu.Lock()
	if n := len(p.free); n > 0 {
		h := p.free[n-1]
		p.free = p.free[:n-1]
		p.mu.Unlock()
		return h, nil
	}
	p.mu.Unlock()
	return p.open(p.ctx)
}

// Put returns a handle to the pool. If err is non-nil the write that used the
// handle failed, so the handle is released instead of being reused.
func (p *Pool[T]) Put(handle T, err error) {
	if err != nil {
		_ = p.release(handle, err)
		return
	}
	p.mu.Lock()
	p.free = append(p.free, handle)
	p.mu.Unlock()
}

// Drain releases every free handle, closing them concurrently, and returns the
// first error encountered.
func (p *Pool[T]) Drain() error {
	p.mu.Lock()
	free := p.free
	p.free = nil
	p.mu.Unlock()

	g := new(errgroup.Group)
	for _, h := range free {
		g.Go(func() error { return p.release(h, nil) })
	}
	return g.Wait()
}
