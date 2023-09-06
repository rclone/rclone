// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package context2

import (
	"context"
	"sync"
)

// cancelContext implements a cancelable context with custom
// error message.
type cancelContext struct {
	context.Context
	cancelParent context.CancelFunc

	mu  sync.Mutex
	err error
}

// WithCustomCancel creates a new context that can be cancelled with
// a custom error.
func WithCustomCancel(parent context.Context) (ctx context.Context, cancel func(error)) {
	cancelCtx, cancelParent := context.WithCancel(parent)

	c := &cancelContext{
		Context:      cancelCtx,
		cancelParent: cancelParent,
	}

	return c, c.cancel
}

// cancel cancels the context with the specified error.
func (ctx *cancelContext) cancel(err error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// if we have already cancelled, we shouldn't
	// update the error.
	if ctx.err != nil {
		return
	}

	ctx.err = err
	ctx.cancelParent()
}

// Err returns the reason why the work should be canceled.
func (ctx *cancelContext) Err() error {
	// If our cancelable context hasn't failed, it means
	// we haven't gotten a cancel request.
	if ctx.Context.Err() == nil {
		return nil
	}

	ctx.mu.Lock()
	// When ctx.err == nil and the ctx.Context != nil,
	// it means the parent context has been cancelled
	// however the current context has not.
	if ctx.err == nil {
		// Propagate the error from the parent context.
		ctx.err = ctx.Context.Err()
	}
	ctx.mu.Unlock()

	return ctx.err
}
