// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
)

// ParentLimiter limits the concurrent goroutines that children run.
// See Child method.
type ParentLimiter struct {
	limiter *Limiter
}

// NewParentLimiter creates a new ParentLimiter with limit set to n.
func NewParentLimiter(n int) *ParentLimiter {
	return &ParentLimiter{
		limiter: NewLimiter(n),
	}
}

// Child create a new parent's child.
func (parent *ParentLimiter) Child() *ChildLimiter {
	return &ChildLimiter{
		parentLimiter: parent.limiter,
	}
}

// Wait waits for all the children's running goroutines to finish.
func (parent *ParentLimiter) Wait() {
	parent.limiter.Wait()
}

// ChildLimiter limits concurrent goroutines by its parent limit
// (ParentLimiter).
type ChildLimiter struct {
	parentLimiter *Limiter
	working       sync.WaitGroup
}

// Go tries to start fn as a goroutine.
// When the parent limit is reached it will wait until it can run it or the
// context is canceled.
// Cancel the context only interrupt the child goroutines waiting to run.
func (child *ChildLimiter) Go(ctx context.Context, fn func()) bool {
	child.working.Add(1)

	return child.parentLimiter.Go(ctx, func() {
		defer child.working.Done()
		fn()
	})
}

// Wait waits for all the child's running goroutines to finish.
func (child *ChildLimiter) Wait() {
	child.working.Wait()
}
