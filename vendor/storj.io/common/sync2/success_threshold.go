// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package sync2

import (
	"context"
	"math"
	"sync"
	"sync/atomic"

	"github.com/zeebo/errs"
)

// SuccessThreshold tracks task formed by a known amount of concurrent tasks.
// It notifies the caller when reached a specific successful threshold without
// interrupting the remaining tasks.
type SuccessThreshold struct {
	noCopy noCopy //nolint:structcheck

	toSucceed int64
	pending   int64

	successes int64
	failures  int64

	done chan struct{}
	once sync.Once
}

// NewSuccessThreshold creates a SuccessThreshold with the tasks number and
// successThreshold.
//
// It returns an error if tasks is less or equal than 1 or successThreshold
// is less or equal than 0 or greater or equal than 1.
func NewSuccessThreshold(tasks int, successThreshold float64) (*SuccessThreshold, error) {
	switch {
	case tasks <= 1:
		return nil, errs.New(
			"invalid number of tasks. It must be greater than 1, got %d", tasks,
		)
	case successThreshold <= 0 || successThreshold > 1:
		return nil, errs.New(
			"invalid successThreshold. It must be greater than 0 and less or equal to 1, got %f", successThreshold,
		)
	}

	tasksToSuccess := int64(math.Ceil(float64(tasks) * successThreshold))

	// just in case of floating point issues
	if tasksToSuccess > int64(tasks) {
		tasksToSuccess = int64(tasks)
	}

	return &SuccessThreshold{
		toSucceed: tasksToSuccess,
		pending:   int64(tasks),
		done:      make(chan struct{}),
	}, nil
}

// Success tells the SuccessThreshold that one tasks was successful.
func (threshold *SuccessThreshold) Success() {
	atomic.AddInt64(&threshold.successes, 1)

	if atomic.AddInt64(&threshold.toSucceed, -1) <= 0 {
		threshold.markAsDone()
	}

	if atomic.AddInt64(&threshold.pending, -1) <= 0 {
		threshold.markAsDone()
	}
}

// Failure tells the SuccessThreshold that one task was a failure.
func (threshold *SuccessThreshold) Failure() {
	atomic.AddInt64(&threshold.failures, 1)

	if atomic.AddInt64(&threshold.pending, -1) <= 0 {
		threshold.markAsDone()
	}
}

// Wait blocks the caller until the successThreshold is reached or all the tasks
// have finished.
func (threshold *SuccessThreshold) Wait(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-threshold.done:
	}
}

// markAsDone finalizes threshold closing the completed channel just once.
// It's safe to be called multiple times.
func (threshold *SuccessThreshold) markAsDone() {
	threshold.once.Do(func() {
		close(threshold.done)
	})
}

// SuccessCount returns the number of successes so far.
func (threshold *SuccessThreshold) SuccessCount() int {
	return int(atomic.LoadInt64(&threshold.successes))
}

// FailureCount returns the number of failures so far.
func (threshold *SuccessThreshold) FailureCount() int {
	return int(atomic.LoadInt64(&threshold.failures))
}
