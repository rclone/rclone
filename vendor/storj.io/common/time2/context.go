// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information

package time2

import (
	"context"
	"time"
)

type clockKey struct{}

// Now invokes the method of the same name on the clock set on the context
// or the real clock when unset.
func Now(ctx context.Context) time.Time {
	return getClock(ctx).Now()
}

// Since invokes the method of the same name on the clock set on the context
// or the real clock when unset.
func Since(ctx context.Context, t time.Time) time.Duration {
	return getClock(ctx).Since(t)
}

// NewTicker invokes the method of the same name on the clock set on the context
// or the real clock when unset.
func NewTicker(ctx context.Context, d time.Duration) Ticker {
	return getClock(ctx).NewTicker(d)
}

// NewTimer invokes the method of the same name on the clock set on the context
// or the real clock when unset.
func NewTimer(ctx context.Context, d time.Duration) Timer {
	return getClock(ctx).NewTimer(d)
}

// Sleep implements sleeping with cancellation using the clock set on the
// context or the real clock when unset.
func Sleep(ctx context.Context, duration time.Duration) bool {
	return getClock(ctx).Sleep(ctx, duration)
}

// WithClock returns a context that will use the given clock.
func WithClock(ctx context.Context, clk Clock) context.Context {
	return context.WithValue(ctx, clockKey{}, clk)
}

// WithNewMachine provides a context that uses a clock controlled by
// the returned time machine.
func WithNewMachine(ctx context.Context, opts ...MachineOption) (context.Context, *Machine) {
	clk := NewMachine(opts...)
	return WithClock(ctx, clk.Clock()), clk
}

func getClock(ctx context.Context) Clock {
	clk, _ := ctx.Value(clockKey{}).(Clock)
	// The zero value is valid and returns a clock that uses real time.
	return clk
}
