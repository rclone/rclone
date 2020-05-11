// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
)

// Limiter implements concurrent goroutine limiting.
type Limiter struct {
	noCopy noCopy // nolint: structcheck

	limit   chan struct{}
	working sync.WaitGroup
}

// NewLimiter creates a new limiter with limit set to n.
func NewLimiter(n int) *Limiter {
	limiter := &Limiter{}
	limiter.limit = make(chan struct{}, n)
	return limiter
}

// Go tries to start fn as a goroutine.
// When the limit is reached it will wait until it can run it
// or the context is canceled.
func (limiter *Limiter) Go(ctx context.Context, fn func()) bool {
	select {
	case limiter.limit <- struct{}{}:
	case <-ctx.Done():
		return false
	}

	limiter.working.Add(1)
	go func() {
		defer func() {
			<-limiter.limit
			limiter.working.Done()
		}()

		fn()
	}()

	return true
}

// Wait waits for all running goroutines to finish.
func (limiter *Limiter) Wait() {
	limiter.working.Wait()
}
