// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"context"
	"time"
)

// Context passes along cancelation signal and request data (PID, GID,
// UID).  The name of this class predates the standard "context"
// package from Go, but it does implement the context.Context
// interface.
//
// When a FUSE request is canceled, the API routine should respond by
// returning the EINTR status code.
type Context struct {
	Caller
	Cancel <-chan struct{}
}

func (c *Context) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *Context) Done() <-chan struct{} {
	return c.Cancel
}

func (c *Context) Err() error {
	select {
	case <-c.Cancel:
		return context.Canceled
	default:
		return nil
	}
}

type callerKeyType struct{}

var callerKey callerKeyType

func FromContext(ctx context.Context) (*Caller, bool) {
	v, ok := ctx.Value(callerKey).(*Caller)
	return v, ok
}

func NewContext(ctx context.Context, caller *Caller) context.Context {
	return context.WithValue(ctx, callerKey, caller)
}

func (c *Context) Value(key interface{}) interface{} {
	if key == callerKey {
		return &c.Caller
	}
	return nil
}

var _ = context.Context((*Context)(nil))
