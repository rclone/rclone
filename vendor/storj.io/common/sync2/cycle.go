// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package sync2

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	monkit "github.com/spacemonkeygo/monkit/v3"
	"golang.org/x/sync/errgroup"
)

// Cycle implements a controllable recurring event.
//
// Cycle control methods PANICS after Close has been called and don't have any
// effect after Stop has been called.
//
// Start or Run (only one of them, not both) must be only called once.
type Cycle struct {
	noCopy noCopy // nolint: structcheck

	stopsent int32
	runexec  int32

	interval time.Duration

	ticker  *time.Ticker
	control chan interface{}

	stopping chan struct{}
	stopped  chan struct{}

	init sync.Once
}

type (
	// cycle control messages
	cyclePause          struct{}
	cycleContinue       struct{}
	cycleChangeInterval struct{ Interval time.Duration }
	cycleTrigger        struct{ done chan struct{} }
)

// NewCycle creates a new cycle with the specified interval.
func NewCycle(interval time.Duration) *Cycle {
	cycle := &Cycle{}
	cycle.SetInterval(interval)
	return cycle
}

// SetInterval allows to change the interval before starting.
func (cycle *Cycle) SetInterval(interval time.Duration) {
	cycle.interval = interval
}

func (cycle *Cycle) initialize() {
	cycle.init.Do(func() {
		cycle.stopped = make(chan struct{})
		cycle.stopping = make(chan struct{})
		cycle.control = make(chan interface{})
	})
}

// Start runs the specified function with an errgroup.
func (cycle *Cycle) Start(ctx context.Context, group *errgroup.Group, fn func(ctx context.Context) error) {
	atomic.CompareAndSwapInt32(&cycle.runexec, 0, 1)
	group.Go(func() error {
		return cycle.Run(ctx, fn)
	})
}

// Run runs the specified in an interval.
//
// Every interval `fn` is started.
// When `fn` is not fast enough, it may skip some of those executions.
//
// Run PANICS if it's called after Stop has been called.
func (cycle *Cycle) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	atomic.CompareAndSwapInt32(&cycle.runexec, 0, 1)
	cycle.initialize()
	defer close(cycle.stopped)

	currentInterval := cycle.interval
	cycle.ticker = time.NewTicker(currentInterval)
	defer cycle.ticker.Stop()

	choreCtx := monkit.ResetContextSpan(ctx)

	if err := fn(choreCtx); err != nil {
		return err
	}
	for {
		// prioritize stopping messages
		select {
		case <-cycle.stopping:
			return nil

		case <-ctx.Done():
			// handle control messages
			return ctx.Err()

		default:
		}

		// handle other messages as well
		select {

		case message := <-cycle.control:
			// handle control messages

			switch message := message.(type) {

			case cycleChangeInterval:
				currentInterval = message.Interval
				cycle.ticker.Stop()
				cycle.ticker = time.NewTicker(currentInterval)

			case cyclePause:
				cycle.ticker.Stop()
				// ensure we don't have ticks left
				select {
				case <-cycle.ticker.C:
				default:
				}

			case cycleContinue:
				cycle.ticker.Stop()
				cycle.ticker = time.NewTicker(currentInterval)

			case cycleTrigger:
				// trigger the function
				if err := fn(choreCtx); err != nil {
					return err
				}
				if message.done != nil {
					close(message.done)
				}
			}

		case <-cycle.stopping:
			return nil

		case <-ctx.Done():
			// handle control messages
			return ctx.Err()

		case <-cycle.ticker.C:
			// trigger the function
			if err := fn(choreCtx); err != nil {
				return err
			}
		}
	}
}

// Close closes all resources associated with it.
//
// It MUST NOT be called concurrently.
func (cycle *Cycle) Close() {
	cycle.Stop()

	if atomic.LoadInt32(&cycle.runexec) == 1 {
		<-cycle.stopped
	}

	close(cycle.control)
}

// sendControl sends a control message
func (cycle *Cycle) sendControl(message interface{}) {
	cycle.initialize()
	select {
	case cycle.control <- message:
	case <-cycle.stopped:
	}
}

// Stop stops the cycle permanently
func (cycle *Cycle) Stop() {
	cycle.initialize()
	if atomic.CompareAndSwapInt32(&cycle.stopsent, 0, 1) {
		close(cycle.stopping)
	}

	if atomic.LoadInt32(&cycle.runexec) == 1 {
		<-cycle.stopped
	}
}

// ChangeInterval allows to change the ticker interval after it has started.
func (cycle *Cycle) ChangeInterval(interval time.Duration) {
	cycle.sendControl(cycleChangeInterval{interval})
}

// Pause pauses the cycle.
func (cycle *Cycle) Pause() {
	cycle.sendControl(cyclePause{})
}

// Restart restarts the ticker from 0.
func (cycle *Cycle) Restart() {
	cycle.sendControl(cycleContinue{})
}

// Trigger ensures that the loop is done at least once.
// If it's currently running it waits for the previous to complete and then runs.
func (cycle *Cycle) Trigger() {
	cycle.sendControl(cycleTrigger{})
}

// TriggerWait ensures that the loop is done at least once and waits for completion.
// If it's currently running it waits for the previous to complete and then runs.
func (cycle *Cycle) TriggerWait() {
	done := make(chan struct{})

	cycle.sendControl(cycleTrigger{done})
	select {
	case <-done:
	case <-cycle.stopped:
	}
}
