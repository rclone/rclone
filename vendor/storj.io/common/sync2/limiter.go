// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
)

// Limiter implements concurrent goroutine limiting.
//
// After calling Wait or Close, no new goroutines are allowed
// to start.
type Limiter struct {
	noCopy noCopy //nolint:structcheck

	limit  chan struct{}
	close  sync.Once
	closed chan struct{}
}

// NewLimiter creates a new limiter with limit set to n.
func NewLimiter(n int) *Limiter {
	return &Limiter{
		limit:  make(chan struct{}, n),
		closed: make(chan struct{}),
	}
}

// Go tries to start fn as a goroutine.
// When the limit is reached it will wait until it can run it
// or the context is canceled.
func (limiter *Limiter) Go(ctx context.Context, fn func()) bool {
	if ctx.Err() != nil {
		return false
	}

	select {
	case limiter.limit <- struct{}{}:
	case <-limiter.closed:
		return false
	case <-ctx.Done():
		return false
	}

	go func() {
		defer func() { <-limiter.limit }()
		fn()
	}()

	return true
}

// Wait waits for all running goroutines to finish and
// disallows new goroutines to start.
func (limiter *Limiter) Wait() { limiter.Close() }

// Close waits for all running goroutines to finish and
// disallows new goroutines to start.
func (limiter *Limiter) Close() {
	limiter.close.Do(func() {
		close(limiter.closed)
		// ensure all goroutines are finished
		for i := 0; i < cap(limiter.limit); i++ {
			limiter.limit <- struct{}{}
		}
	})
}
