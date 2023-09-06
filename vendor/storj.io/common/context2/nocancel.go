// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package context2 contains utilities for contexts.
package context2

import (
	"context"
	"fmt"
	"time"
)

// WithoutCancellation returns a context that does not propagate Done message
// down to children. However, Values are propagated.
func WithoutCancellation(ctx context.Context) context.Context {
	return noCancelContext{ctx}
}

type noCancelContext struct {
	ctx context.Context
}

// Deadline returns the time when work done on behalf of this context
// should be canceled.
func (noCancelContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done returns empty channel.
func (noCancelContext) Done() <-chan struct{} {
	return nil
}

// Err always returns nil.
func (noCancelContext) Err() error {
	return nil
}

// String returns string.
func (ctx noCancelContext) String() string {
	return fmt.Sprintf("no cancel (%s)", ctx.ctx)
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Successive calls to Value with
// the same key returns the same result.
func (ctx noCancelContext) Value(key interface{}) interface{} {
	return ctx.ctx.Value(key)
}
