// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metaclient

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"time"

	"storj.io/common/sync2"
)

// ExponentialBackoff keeps track of how long we should sleep between
// failing attempts.
type ExponentialBackoff struct {
	delay time.Duration
	Max   time.Duration
	Min   time.Duration
}

func (e *ExponentialBackoff) init() {
	if e.Max == 0 {
		// maximum delay - pulled from net/http.Server.Serve
		e.Max = time.Second
	}
	if e.Min == 0 {
		// minimum delay - pulled from net/http.Server.Serve
		e.Min = 5 * time.Millisecond
	}
}

// Wait should be called when there is a failure. Each time it is called
// it will sleep an exponentially longer time, up to a max.
func (e *ExponentialBackoff) Wait(ctx context.Context) bool {
	e.init()
	if e.delay == 0 {
		e.delay = e.Min
	} else {
		e.delay *= 2
	}
	if e.delay > e.Max {
		e.delay = e.Max
	}
	return sync2.Sleep(ctx, e.delay)
}

// Maxed returns true if the wait time has maxed out.
func (e *ExponentialBackoff) Maxed() bool {
	e.init()
	return e.delay == e.Max
}

// WithRetry attempts to retry a function with exponential backoff. If the retry has occurred
// enough times that the delay is maxed out and the function still returns an error, the error
// is returned.
func WithRetry(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	delay := ExponentialBackoff{
		Min: 100 * time.Millisecond,
		Max: 3 * time.Second,
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		err = fn(ctx)
		if err != nil && needsRetry(err) {
			if !delay.Maxed() {
				if !delay.Wait(ctx) {
					return ctx.Err()
				}
				continue
			}
		}
		return err
	}
}

func needsRetry(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		mon.Event("uplink_error_eof")
		// Currently we don't retry with EOF because it's unclear if
		// a query succeeded or failed.
		return false
	}

	if errors.Is(err, syscall.ECONNRESET) {
		mon.Event("uplink_error_conn_reset_needed_retry")
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		mon.Event("uplink_error_conn_refused_needed_retry")
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		mon.Event("uplink_net_error_needed_retry")
		return true
	}

	return false
}
