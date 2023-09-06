// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"sync"
)

// WorkGroup implements waitable and closable group of workers.
type WorkGroup struct {
	noCopy noCopy //nolint:structcheck

	mu   sync.Mutex
	cond sync.Cond

	initialized bool
	closed      bool
	workers     int
}

// init initializes work group.
func (group *WorkGroup) init() {
	if !group.initialized {
		group.cond.L = &group.mu
	}
}

// Go starts func and tracks the execution.
// Returns false when WorkGroup has been closed.
func (group *WorkGroup) Go(fn func()) bool {
	if !group.Start() {
		return false
	}
	go func() {
		defer group.Done()
		fn()
	}()
	return true
}

// Start returns true when work can be started.
func (group *WorkGroup) Start() bool {
	group.mu.Lock()
	defer group.mu.Unlock()

	group.init()
	if group.closed {
		return false
	}
	group.workers++
	return true
}

// Done finishes a pending work item.
func (group *WorkGroup) Done() {
	group.mu.Lock()
	defer group.mu.Unlock()

	group.workers--
	if group.workers < 0 {
		panic("worker count below zero")
	}
	if group.workers == 0 {
		group.cond.Broadcast()
	}
}

// Wait waits for all workers to finish.
func (group *WorkGroup) Wait() {
	group.mu.Lock()
	defer group.mu.Unlock()

	group.init()

	for group.workers != 0 {
		group.cond.Wait()
	}
}

// Close prevents from new work being started.
func (group *WorkGroup) Close() {
	group.mu.Lock()
	defer group.mu.Unlock()

	group.init()
	group.closed = true
}
