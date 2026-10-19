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
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ErrRetry may be returned by the open function to tell Get that it stopped
// waiting because a handle was returned to the pool (see Pool.Avail), so Get
// should look for a free handle again.
var ErrRetry = errors.New("filepool: retry getting a free handle")

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

	avail chan struct{} // signalled when a handle is put back
}

// New returns a Pool.
//
// open opens a fresh handle on its own connection. release closes a handle and
// returns its connection: err is the error that made the handle unusable (nil
// when the handle is simply being drained) and the returned error is the result
// of closing it.
func New[T any](ctx context.Context, open func(context.Context) (T, error), release func(handle T, err error) error) *Pool[T] {
	return &Pool[T]{ctx: ctx, open: open, release: release, avail: make(chan struct{}, 1)}
}

// Avail returns a channel which is signalled when a handle is put back in the
// pool. An open function which may block waiting for a connection can select
// on it and return ErrRetry, so that it doesn't wait for a new connection while
// a free handle is available.
func (p *Pool[T]) Avail() <-chan struct{} {
	return p.avail
}

// Get returns a free handle, opening a new one if none are free.
func (p *Pool[T]) Get() (T, error) {
	for {
		p.mu.Lock()
		if n := len(p.free); n > 0 {
			h := p.free[n-1]
			p.free = p.free[:n-1]
			p.mu.Unlock()
			return h, nil
		}
		p.mu.Unlock()
		h, err := p.open(p.ctx)
		if errors.Is(err, ErrRetry) {
			continue
		}
		return h, err
	}
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
	select {
	case p.avail <- struct{}{}:
	default:
	}
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
