// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// Semaphore implements a closable semaphore.
type Semaphore struct {
	noCopy noCopy // nolint: structcheck

	ctx   context.Context
	close func()
	sema  *semaphore.Weighted
}

// NewSemaphore creates a semaphore with the specified size.
func NewSemaphore(size int) *Semaphore {
	sema := &Semaphore{}
	sema.Init(size)
	return sema
}

// Init initializes semaphore to the specified size.
func (sema *Semaphore) Init(size int) {
	sema.ctx, sema.close = context.WithCancel(context.Background())
	sema.sema = semaphore.NewWeighted(int64(size))
}

// Close closes the semaphore from further use.
func (sema *Semaphore) Close() {
	sema.close()
}

// Lock locks the semaphore.
func (sema *Semaphore) Lock() bool {
	return sema.sema.Acquire(sema.ctx, 1) == nil
}

// Unlock unlocks the semaphore.
func (sema *Semaphore) Unlock() {
	sema.sema.Release(1)
}
