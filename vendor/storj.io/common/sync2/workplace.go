// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package sync2

import (
	"context"
	"sync"
)

// Workplace allows controlling separate jobs that must not run concurrently.
type Workplace struct {
	mu     sync.Mutex
	done   bool
	active *worker
}

// NewWorkPlace creates a new work place.
func NewWorkPlace() *Workplace {
	return &Workplace{}
}

// worker represents an active work.
type worker struct {
	// jobTag is a unique identifier for the work.
	jobTag interface{}
	// cancel cancels the running job.
	cancel func()
	// done will be closed after the func returns.
	done chan struct{}
}

// Start tries to spawn a goroutine in background. It returns false, when it cannot cancel the previous work,
// the context is cancelled or the workplace itself has been canceled.
func (place *Workplace) Start(root context.Context, jobTag interface{}, shouldCancel func(jobTag interface{}) bool, fn func(ctx context.Context)) (started bool) {
	place.mu.Lock()
	defer place.mu.Unlock()

	// if we are done, don't do anything.
	if root.Err() != nil || place.done {
		return false
	}

	// is there any job already running?
	before := place.active
	if before != nil {
		if shouldCancel == nil {
			return false
		}
		// check whether we should cancel the existing job or not
		if !shouldCancel(before.jobTag) {
			return false
		}
	}

	// create a new context, so we can cancel any worker.
	ctx, cancel := context.WithCancel(root)

	// create the next worker...
	next := &worker{
		jobTag: jobTag,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	place.active = next

	go func() {
		defer cancel()
		defer func() {
			place.mu.Lock()
			defer place.mu.Unlock()

			// clear the active worker...
			// if some other worker canceled us, we shouldn't override their state.
			if place.active == next {
				place.active = nil
			}
			close(next.done)
		}()

		// wait for the previous job to finish.
		if before != nil {
			before.cancel()

			<-before.done
		}

		// start the work.
		fn(ctx)
	}()

	return true
}

// Cancel cancels any active place and prevents new ones from being started.
// It does not wait for the active job to be finished.
func (place *Workplace) Cancel() {
	place.mu.Lock()
	defer place.mu.Unlock()

	place.done = true
	if place.active != nil {
		place.active.cancel()
	}
}

// Done returns channel for waiting for the current job to be completed.
// If there's no active job, it'll return a closed channel.
func (place *Workplace) Done() <-chan struct{} {
	place.mu.Lock()
	defer place.mu.Unlock()

	if place.active != nil {
		return place.active.done
	}

	return doneChan
}

var doneChan = make(chan struct{})

func init() { close(doneChan) }
