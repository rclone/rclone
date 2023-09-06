// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package time2

import (
	"context"
	"time"
)

// Clock implements a clock. The zero value is valid and uses the real time
// functions.
type Clock struct {
	backend backend
}

// Now provides functionality equivalent to time.Now. It is safe to call on a
// zero value.
func (c Clock) Now() time.Time {
	if c.backend != nil {
		return c.backend.Now()
	}
	return time.Now()
}

// Since provides functionality equivalent to time.Since. It is safe to call on
// a zero value.
func (c Clock) Since(t time.Time) time.Duration {
	if c.backend != nil {
		return c.backend.Since(t)
	}
	return time.Since(t)
}

// NewTicker provides functionality equivalent to time.NewTicker. It is safe to
// call on a zero value.
func (c Clock) NewTicker(d time.Duration) Ticker {
	if c.backend != nil {
		return c.backend.NewTicker(d)
	}
	return realTicker{ticker: time.NewTicker(d)}
}

// NewTimer provides functionality equivalent to time.NewTimer. It is safe to
// call on a zero value.
func (c Clock) NewTimer(d time.Duration) Timer {
	if c.backend != nil {
		return c.backend.NewTimer(d)
	}
	return realTimer{timer: time.NewTimer(d)}
}

// Sleep is a convenience function that provides functionality equivalent to
// time.Sleep, except that it respects context cancellation. True is returned
// if the timer expired or false if the context was done.
func (c Clock) Sleep(ctx context.Context, d time.Duration) bool {
	timer := c.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.Chan():
		return true
	case <-ctx.Done():
		return false
	}
}

// Timer provides functionality equivalent to time.Timer.
type Timer interface {
	Chan() <-chan time.Time
	Reset(d time.Duration) bool
	Stop() bool
}

// Ticker provides functionality equivalent to time.Ticker.
type Ticker interface {
	Chan() <-chan time.Time
	Reset(d time.Duration)
	Stop()
}

type backend interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	NewTicker(d time.Duration) Ticker
	NewTimer(d time.Duration) Timer
}

type realTicker struct{ ticker *time.Ticker }

func (t realTicker) Chan() <-chan time.Time { return t.ticker.C }
func (t realTicker) Reset(d time.Duration)  { t.ticker.Reset(d) }
func (t realTicker) Stop()                  { t.ticker.Stop() }

type realTimer struct{ timer *time.Timer }

func (t realTimer) Chan() <-chan time.Time     { return t.timer.C }
func (t realTimer) Reset(d time.Duration) bool { return t.timer.Reset(d) }
func (t realTimer) Stop() bool                 { return t.timer.Stop() }
