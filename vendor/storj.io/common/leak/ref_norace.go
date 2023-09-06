// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !race
// +build !race

package leak

import "context"

// Ref implements a [Resource] tracker that is only enabled
// when compiling with `-race`.
type Ref struct {
}

// Root returns a root reference to a tracker.
func Root(skipCallers int) Ref {
	return Ref{}
}

// Child returns a child ref.
func (ref Ref) Child(name string, skipCallers int) Ref {
	return Ref{}
}

// StartStack returns formatted stack where the resource was created.
func (ref Ref) StartStack() string {
	return ""
}

// Close closes the ref and checks whether all the children have been closed.
//
// The caller should close the ref with [Ref.Close].
func (ref Ref) Close() error {
	return nil
}

// WithContext attaches a root context that handles tracking.
func WithContext(parent context.Context) (Ref, context.Context) {
	return Ref{}, parent
}

// FromContext returns the attached root Ref.
func FromContext(parent context.Context) Ref {
	return Ref{}
}
