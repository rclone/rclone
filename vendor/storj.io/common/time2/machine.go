// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package time2

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"
)

// MachineOption allows optional configuration of the Machine.
type MachineOption = func(*machineBackend)

// WithTimeAt uses the provided time as the current time of the time machine.
func WithTimeAt(t time.Time) MachineOption {
	return func(backend *machineBackend) {
		backend.now = t
	}
}

// Machine provides facilities to manipulate time, timers, and tickers. It
// is designed for use in deterministic testing of time reliant code. Since
// deterministism is hard to build when influencing a clock from multiple
// goroutines, the Machine disallows concurrent access by-design.
type Machine struct {
	backend machineBackend
}

// NewMachine returns a new time machine configured with the given options.
func NewMachine(opts ...MachineOption) *Machine {
	now := time.Now().Truncate(time.Second)
	tm := &Machine{
		backend: machineBackend{
			now: now,
		},
	}
	tm.backend.cond.L = &tm.backend.mu
	for _, opt := range opts {
		opt(&tm.backend)
	}
	return tm
}

// Clock returns a clock controlled by the time machine.
func (tm *Machine) Clock() Clock {
	return Clock{backend: &tm.backend}
}

// Advance advances the clock forward, triggering all expired timers/tickers
// tracked by the time machine. It should not be called concurrently.
func (tm *Machine) Advance(d time.Duration) {
	tm.backend.blockThenAdvance(context.Background(), 0, d)
}

// Block blocks execution until the scheduled timer count reaches the provided
// minimum. Timers are scheduled until triggered or stopped. Tickers are always
// scheduled until stopped. It should not be called concurrently. Returns false
// if the context became done while blocking.
func (tm *Machine) Block(ctx context.Context, minimumTimerCount int) bool {
	return tm.backend.blockThenAdvance(ctx, minimumTimerCount, 0)
}

// BlockThenAdvance is a convenience method that blocks on the minimum timer
// count and then advances the clock, triggering any expired timers/tickers. It
// should not be called concurrently. Returns false if the context became done
// while blocking.
func (tm *Machine) BlockThenAdvance(ctx context.Context, minimumTimerCount int, d time.Duration) bool {
	return tm.backend.blockThenAdvance(ctx, minimumTimerCount, d)
}

// Now provides functionality equivalent to time.Now according to the
// the time machine clock.
func (tm *Machine) Now() time.Time {
	return tm.backend.Now()
}

// Since provides functionality equivalent to time.Since according to the
// the time machine clock.
func (tm *Machine) Since(t time.Time) time.Duration {
	return tm.backend.Since(t)
}

type machineBackend struct {
	mu        sync.Mutex
	cond      sync.Cond
	now       time.Time
	advancing bool
	timers    timerHeap

	// This variable is only used to aid detection of concurrent calls by
	// the race detector. It is unused otherwise.
	detectConcurrentCalls int
}

// Now provides functionality equivalent to time.Now according to the
// the time machine clock.
func (backend *machineBackend) Now() time.Time {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	return backend.now
}

// Since provides functionality equivalent to time.Since according to the
// the time machine clock.
func (backend *machineBackend) Since(t time.Time) time.Duration {
	return backend.Now().Sub(t)
}

// NewTicker provides functionality equivalent to time.NewTicker according to
// the time machine clock.
func (backend *machineBackend) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTicker"))
	}
	return &fakeTicker{timer: backend.newTimer(d, false)}
}

// NewTimer provides functionality equivalent to time.NewTimer according to
// the time machine clock.
func (backend *machineBackend) NewTimer(d time.Duration) Timer {
	if d <= 0 {
		panic(errors.New("non-positive interval for NewTimer"))
	}
	return backend.newTimer(d, true)
}

func (backend *machineBackend) blockThenAdvance(ctx context.Context, minimumTimerCount int, d time.Duration) bool {
	if d < 0 {
		// We cannot go back, marty!
		panic(errors.New("negative delta for advance"))
	}

	// This counter gives the race detector a chance to notice concurrent calls
	// to these functions, which is against the design of the time machine.
	backend.detectConcurrentCalls++

	backend.mu.Lock()
	defer backend.mu.Unlock()

	if backend.advancing {
		panic(errors.New("concurrent call to Advance/Block/BlockThenAdvance"))
	}

	backend.advancing = true
	defer func() {
		backend.advancing = false
	}()

	ctx, cancel := context.WithCancel(ctx)

	// Unblock the condition variable if the context is finished. It would
	// be nice to use primitives from the sync2 package, but we can't since
	// that would introduce an import cycle.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		backend.cond.Broadcast()
	}()
	defer func() {
		cancel()
		wg.Wait()
	}()

	for len(backend.timers) < minimumTimerCount {
		if ctx.Err() != nil {
			return false
		}
		backend.cond.Wait()
	}

	now := backend.now.Add(d)

	for len(backend.timers) > 0 {
		timer := backend.timers[0]
		if now.Before(timer.when) {
			break
		}

		// Do a non-blocking send into the buffered channel. This preserves go
		// runtime behavior that the first ticks time is what is present on
		// the channel.
		select {
		case timer.ch <- timer.when:
		default:
		}

		// Reschedule if the timer is on an interval (i.e. a ticker).
		if timer.interval > 0 {
			timer.when = timer.when.Add(timer.interval)
			heap.Fix(&backend.timers, 0)
		} else {
			dropTimer(&backend.timers, timer)
		}
	}

	backend.now = now
	return true
}

func (backend *machineBackend) newTimer(interval time.Duration, oneShot bool) *fakeTimer {
	backend.mu.Lock()
	defer backend.mu.Unlock()

	when := backend.now.Add(interval)
	if oneShot {
		// Disable the interval for one-shot timers
		interval = 0
	}

	timer := &fakeTimer{
		backend:  backend,
		ch:       make(chan time.Time, 1),
		when:     when,
		interval: interval,
	}

	// Add the new timer and broadcast to wake blockers.
	heap.Push(&backend.timers, timer)
	backend.cond.Broadcast()
	return timer
}

func (backend *machineBackend) resetTimer(timer *fakeTimer, d time.Duration) (active bool) {
	if d <= 0 {
		panic(errors.New("non-positive interval for Reset"))
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()

	timer.when = backend.now.Add(d)
	if timer.interval > 0 {
		timer.interval = d
	}

	for i, candidate := range backend.timers {
		if candidate == timer {
			heap.Fix(&backend.timers, i)
			return true
		}
	}

	heap.Push(&backend.timers, timer)
	backend.cond.Broadcast()
	return false
}

func (backend *machineBackend) stopTimer(timer *fakeTimer) bool {
	backend.mu.Lock()
	defer backend.mu.Unlock()

	return dropTimer(&backend.timers, timer)
}

type fakeTimer struct {
	backend  *machineBackend
	ch       chan time.Time
	when     time.Time
	interval time.Duration
}

// Chan returns a channel on which timer expiry is delivered.
func (timer *fakeTimer) Chan() <-chan time.Time {
	return timer.ch
}

// Reset provides functionality equivalent to the time.Timer method of the same name.
func (timer *fakeTimer) Reset(d time.Duration) bool {
	return timer.backend.resetTimer(timer, d)
}

// Stop provides functionality equivalent to the time.Timer method of the same name.
func (timer *fakeTimer) Stop() bool {
	return timer.backend.stopTimer(timer)
}

type fakeTicker struct {
	timer *fakeTimer
}

// Chan returns a channel on which ticks are delivered.
func (ticker *fakeTicker) Chan() <-chan time.Time { return ticker.timer.Chan() }

// Reset provides functionality equivalent to the time.Ticker method of the same name.
func (ticker *fakeTicker) Reset(d time.Duration) { ticker.timer.Reset(d) }

// Stop provides functionality equivalent to the time.Ticker method of the same name.
func (ticker *fakeTicker) Stop() { ticker.timer.Stop() }

func dropTimer(timers *timerHeap, timer *fakeTimer) (dropped bool) {
	for i, candidate := range *timers {
		if candidate == timer {
			heap.Remove(timers, i)
			return true
		}
	}
	return false
}

type timerHeap []*fakeTimer

func (h timerHeap) Len() int            { return len(h) }
func (h timerHeap) Less(i, j int) bool  { return h[i].when.Before(h[j].when) }
func (h timerHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *timerHeap) Push(x interface{}) { *h = append(*h, x.(*fakeTimer)) }

func (h *timerHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
