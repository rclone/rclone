// Package transferaccounter provides utilities for accounting server side transfers.
package transferaccounter

import (
	"context"
	"sync/atomic"
)

// Context key type for accounter
type accounterContextKeyType struct{}

// Context key for accounter
var accounterContextKey = accounterContextKeyType{}

// TransferAccounter is used to account server side and other transfers.
type TransferAccounter struct {
	add     func(n int64)
	total   atomic.Int64
	started bool
}

// New creates a TransferAccounter using the add function passed in.
//
// Note that the add function should be goroutine safe.
//
// It adds the new TransferAccounter to the context.
func New(ctx context.Context, add func(n int64)) (context.Context, *TransferAccounter) {
	ta := &TransferAccounter{
		add: add,
	}
	newCtx := context.WithValue(ctx, accounterContextKey, ta)
	return newCtx, ta
}

// Start the transfer. Call this before calling Add().
func (ta *TransferAccounter) Start() {
	ta.started = true
}

// Started returns if the transfer has had Start() called or not.
func (ta *TransferAccounter) Started() bool {
	return ta.started
}

// Add n bytes to the transfer
func (ta *TransferAccounter) Add(n int64) {
	ta.add(n)
	ta.total.Add(n)
}

// Reset reverses out all accounted stats if Started() has been called
func (ta *TransferAccounter) Reset() {
	if ta.started {
		ta.Add(-ta.total.Load())
	}
}

// A transfer accounter which does nothing
var nullAccounter = &TransferAccounter{
	add: func(n int64) {},
}

// Get returns a *TransferAccounter from the ctx.
//
// If none is found it will return a dummy one to keep the code simple.
func Get(ctx context.Context) *TransferAccounter {
	if ctx == nil {
		return nullAccounter
	}
	c := ctx.Value(accounterContextKey)
	if c == nil {
		return nullAccounter
	}
	return c.(*TransferAccounter)
}
