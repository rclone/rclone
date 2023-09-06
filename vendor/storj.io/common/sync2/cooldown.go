// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// Cooldown implements an event that can only occur once in a given timeframe.
//
// Cooldown control methods PANICS after Close has been called and don't have any
// effect after Stop has been called.
//
// Start or Run (only one of them, not both) must be only called once.
type Cooldown struct {
	noCopy noCopy //nolint:structcheck

	stopsent int32
	runexec  int32

	interval time.Duration

	init     sync.Once
	trigger  chan struct{}
	stopping chan struct{}
	stopped  chan struct{}
}

// NewCooldown creates a new cooldown with the specified interval.
func NewCooldown(interval time.Duration) *Cooldown {
	cooldown := &Cooldown{}
	cooldown.SetInterval(interval)
	return cooldown
}

// SetInterval allows to change the interval before starting.
func (cooldown *Cooldown) SetInterval(interval time.Duration) {
	cooldown.interval = interval
}

func (cooldown *Cooldown) initialize() {
	cooldown.init.Do(func() {
		cooldown.stopped = make(chan struct{})
		cooldown.stopping = make(chan struct{})
		cooldown.trigger = make(chan struct{}, 1)
	})
}

// Start runs the specified function with an errgroup.
func (cooldown *Cooldown) Start(ctx context.Context, group *errgroup.Group, fn func(ctx context.Context) error) {
	atomic.StoreInt32(&cooldown.runexec, 1)
	group.Go(func() error {
		return cooldown.Run(ctx, fn)
	})
}

// Run waits for a message on the trigger channel, then runs the specified function.
// Afterwards it will sleep for the cooldown duration and drain the trigger channel.
//
// Run PANICS if it's called after Stop has been called.
func (cooldown *Cooldown) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	atomic.StoreInt32(&cooldown.runexec, 1)
	cooldown.initialize()
	defer close(cooldown.stopped)
	for {
		// prioritize stopping messages
		select {
		case <-cooldown.stopping:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// handle trigger message
		select {
		case <-cooldown.trigger:
			// trigger the function
			if err := fn(ctx); err != nil {
				return err
			}
			if !Sleep(ctx, cooldown.interval) {
				return ctx.Err()
			}

			// drain the channel to prevent messages received during sleep from triggering the function again
			select {
			case <-cooldown.trigger:
			default:
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-cooldown.stopping:
			return nil
		}

	}
}

// Close closes all resources associated with it.
//
// It MUST NOT be called concurrently.
func (cooldown *Cooldown) Close() {
	cooldown.Stop()

	if atomic.LoadInt32(&cooldown.runexec) == 1 {
		<-cooldown.stopped
	}

	close(cooldown.trigger)
}

// Stop stops the cooldown permanently.
func (cooldown *Cooldown) Stop() {
	cooldown.initialize()
	if atomic.CompareAndSwapInt32(&cooldown.stopsent, 0, 1) {
		close(cooldown.stopping)
	}

	if atomic.LoadInt32(&cooldown.runexec) == 1 {
		<-cooldown.stopped
	}
}

// Trigger attempts to run the cooldown function.
// If the timer has not expired, the function will not run.
func (cooldown *Cooldown) Trigger() {
	cooldown.initialize()
	select {
	case cooldown.trigger <- struct{}{}:
	default:
	}
}
