// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"sync"
)

// Throttle implements two-sided throttling, between a consumer and producer
type Throttle struct {
	noCopy noCopy // nolint: structcheck

	mu       sync.Mutex
	consumer sync.Cond
	producer sync.Cond

	// error tracking for terminating Consume and Allocate
	errs []error

	// how much is available in the throttle
	// consumer decreases availability and blocks when it's below zero
	// producer increses availability and blocks as needed
	available int64
}

// NewThrottle returns a new Throttle primitive
func NewThrottle() *Throttle {
	var throttle Throttle
	throttle.consumer.L = &throttle.mu
	throttle.producer.L = &throttle.mu
	return &throttle
}

// Consume subtracts amount from the throttle
func (throttle *Throttle) Consume(amount int64) error {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	throttle.available -= amount
	throttle.producer.Signal()
	return throttle.combinedError()
}

// ConsumeOrWait tries to consume at most maxAmount
func (throttle *Throttle) ConsumeOrWait(maxAmount int64) (int64, error) {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()

	for throttle.alive() && throttle.available <= 0 {
		throttle.consumer.Wait()
	}

	available := throttle.available
	if available > maxAmount {
		available = maxAmount
	}
	throttle.available -= available
	throttle.producer.Signal()

	return available, throttle.combinedError()
}

// WaitUntilAbove waits until availability drops below limit
func (throttle *Throttle) WaitUntilAbove(limit int64) error {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	for throttle.alive() && throttle.available <= limit {
		throttle.consumer.Wait()
	}
	return throttle.combinedError()
}

// Produce adds amount to the throttle
func (throttle *Throttle) Produce(amount int64) error {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	throttle.available += amount
	throttle.consumer.Signal()
	return throttle.combinedError()
}

// ProduceAndWaitUntilBelow adds amount to the throttle and waits until it's below the given threshold
func (throttle *Throttle) ProduceAndWaitUntilBelow(amount, limit int64) error {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	throttle.available += amount
	throttle.consumer.Signal()
	for throttle.alive() && throttle.available >= limit {
		throttle.producer.Wait()
	}
	return throttle.combinedError()
}

// WaitUntilBelow waits until availability drops below limit
func (throttle *Throttle) WaitUntilBelow(limit int64) error {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	for throttle.alive() && throttle.available >= limit {
		throttle.producer.Wait()
	}
	return throttle.combinedError()
}

// Fail stops both consumer and allocator
func (throttle *Throttle) Fail(err error) {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()

	throttle.errs = append(throttle.errs, err)
	throttle.consumer.Signal()
	throttle.producer.Signal()
}

// must hold mutex when calling this
func (throttle *Throttle) alive() bool { return len(throttle.errs) == 0 }

func (throttle *Throttle) combinedError() error {
	if len(throttle.errs) == 0 {
		return nil
	}
	// TODO: combine errors
	return throttle.errs[0]
}

// Err returns the finishing error
func (throttle *Throttle) Err() error {
	throttle.mu.Lock()
	defer throttle.mu.Unlock()
	return throttle.combinedError()
}
