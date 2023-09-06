// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcctx

import (
	"context"
	"sync"
)

// Tracker keeps track of launched goroutines with a context.
type Tracker struct {
	context.Context
	cancel func()
	wg     sync.WaitGroup
}

// NewTracker creates a Tracker bound to the provided context.
func NewTracker(ctx context.Context) *Tracker {
	ctx, cancel := context.WithCancel(ctx)
	return &Tracker{
		Context: ctx,
		cancel:  cancel,
	}
}

// Run starts a goroutine running the callback with the tracker as the context.
func (t *Tracker) Run(cb func(ctx context.Context)) {
	t.wg.Add(1)
	go t.track(cb)
}

// track is a helper to call done on the waitgroup after the callback returns.
func (t *Tracker) track(cb func(ctx context.Context)) {
	cb(t)
	t.wg.Done()
}

// Cancel cancels the tracker's context.
func (t *Tracker) Cancel() { t.cancel() }

// Wait blocks until all callbacks started with Run have exited.
func (t *Tracker) Wait() { t.wg.Wait() }
