// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package sync2

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
)

// ReadCache is a backwards compatible implementation.
type ReadCache = ReadCacheOf[any]

// ReadCacheOf implements refreshing of state based on a refresh timeout,
// but also allows for stale reads up to a certain duration.
type ReadCacheOf[T any] struct {
	noCopy noCopy //nolint:structcheck

	started Fence
	ctx     context.Context

	// read is a func that's called when a new update is needed.
	read func(ctx context.Context) (T, error)
	// refresh defines when the state should be updated.
	refresh time.Duration
	// stale defines when we must wait for the new state.
	stale time.Duration

	// mu protects the internal state of the cache.
	mu sync.Mutex
	// closed is set true when the read cache is shuting down.
	closed bool
	// result contains the last known state and any errors that
	// occurred during refreshing.
	result *readCacheResult[T]
	// pending is a channel for waiting for the current refresh.
	// it is only present, when there is an ongoing refresh.
	pending *readCacheWorker[T]
}

// NewReadCache returns a new ReadCacheOf.
func NewReadCache[T any](refresh time.Duration, stale time.Duration, read func(ctx context.Context) (T, error)) (*ReadCacheOf[T], error) {
	cache := &ReadCacheOf[T]{}
	return cache, cache.Init(refresh, stale, read)
}

// Init initializes the cache for in-place initialization. This is only needed when NewReadCache
// was not used to initialize it.
func (cache *ReadCacheOf[T]) Init(refresh time.Duration, stale time.Duration, read func(ctx context.Context) (T, error)) error {
	if refresh > stale {
		refresh = stale
	}
	if refresh <= 0 || stale <= 0 {
		return errs.New("refresh and stale must be positive. refresh=%v, stale=%v", refresh, stale)
	}
	cache.read = read
	cache.refresh = refresh
	cache.stale = stale
	return nil
}

// readCacheWorker contains the pending result.
type readCacheWorker[T any] struct {
	done   chan struct{}
	result *readCacheResult[T]
}

// readCacheResult contains the result of a read and info related to it.
type readCacheResult[T any] struct {
	start time.Time
	state T
	err   error
}

// Run starts the background process for the cache.
func (cache *ReadCacheOf[T]) Run(ctx context.Context) error {
	// set the root context
	cache.ctx = ctx
	cache.started.Release()

	// wait for things to start shutting down
	<-ctx.Done()

	// close the workers
	cache.mu.Lock()
	cache.closed = true
	pending := cache.pending
	cache.mu.Unlock()

	// wait for worker to exit
	if pending != nil {
		<-pending.done
	}

	return nil
}

// Get fetches the latest state and refreshes when it's needed.
func (cache *ReadCacheOf[T]) Get(ctx context.Context, now time.Time) (state T, err error) {
	if !cache.started.Wait(ctx) {
		var zero T
		return zero, ctx.Err()
	}

	// check whether we need to start a refresh
	cache.mu.Lock()
	mustWait := false
	if cache.result == nil || cache.result.err != nil || now.Sub(cache.result.start) >= cache.refresh {
		// check whether we must wait for the result:
		//   * we don't have anything in cache
		//   * the cache state has errored
		//   * we have reached the staleness deadline
		mustWait = cache.result == nil || cache.result.err != nil || now.Sub(cache.result.start) >= cache.stale
		if err := cache.startRefresh(now); err != nil {
			cache.mu.Unlock()
			var zero T
			return zero, err
		}
	}
	result, pending := cache.result, cache.pending
	cache.mu.Unlock()

	// wait for the new result, when needed
	if mustWait {
		select {
		case <-pending.done:
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		}
		result = pending.result
	}

	return result.state, result.err
}

// RefreshAndGet refreshes the cache and returns the latest result.
func (cache *ReadCacheOf[T]) RefreshAndGet(ctx context.Context, now time.Time) (state T, err error) {
	if !cache.started.Wait(ctx) {
		return state, ctx.Err()
	}

	cache.mu.Lock()
	if err := cache.startRefresh(now); err != nil {
		cache.mu.Unlock()
		var zero T
		return zero, err
	}
	pending := cache.pending
	cache.mu.Unlock()

	select {
	case <-pending.done:
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}

	return pending.result.state, pending.result.err
}

// Wait waits for any pending refresh and returns the result.
func (cache *ReadCacheOf[T]) Wait(ctx context.Context) (state T, err error) {
	if !cache.started.Wait(ctx) {
		var zero T
		return zero, ctx.Err()
	}

	cache.mu.Lock()
	result, pending := cache.result, cache.pending
	cache.mu.Unlock()

	if pending != nil {
		select {
		case <-pending.done:
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		}
		return pending.result.state, pending.result.err
	}

	return result.state, result.err
}

// startRefresh starts a new background refresh, when one isn't running
// already. It will return an error when the cache is shutting down.
//
// Note: this must only be called when `cache.mu` is being held.
func (cache *ReadCacheOf[T]) startRefresh(now time.Time) error {
	if cache.closed {
		return context.Canceled
	}
	if cache.pending != nil {
		return nil
	}

	pending := &readCacheWorker[T]{
		done:   make(chan struct{}),
		result: nil,
	}

	go func() {
		defer close(pending.done)

		state, err := cache.read(cache.ctx)
		cache.mu.Lock()
		result := &readCacheResult[T]{
			start: now,
			state: state,
			err:   err,
		}
		cache.result = result
		pending.result = result
		cache.pending = nil
		cache.mu.Unlock()
	}()

	cache.pending = pending
	return nil
}
