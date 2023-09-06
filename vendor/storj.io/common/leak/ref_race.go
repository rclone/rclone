// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build race
// +build race

package leak

import "context"

// Ref implements a [Resource] tracker that is only enabled
// when compiling with `-race`.
type Ref struct {
	r *Resource
}

// Root returns a root reference to a tracker.
func Root(skipCallers int) Ref {
	return Ref{
		r: RootResource(skipCallers + 1),
	}
}

// Child returns a child ref.
func (ref Ref) Child(name string, skipCallers int) Ref {
	if ref.r == nil {
		return Ref{}
	}
	return Ref{
		r: ref.r.Child(name, skipCallers+1),
	}
}

// StartStack returns formatted stack where the resource was created.
func (ref Ref) StartStack() string {
	if ref.r == nil {
		return ""
	}
	return ref.r.StartStack()
}

// Close closes the ref and checks whether all the children have been closed.
func (ref Ref) Close() error {
	if ref.r == nil {
		return nil
	}
	return ref.r.Close()
}

type contextRefKey struct{}

// WithContext attaches a root context that handles tracking.
//
// The caller should close the ref with [Ref.Close].
func WithContext(parent context.Context) (Ref, context.Context) {
	ref := Root(1)
	ctx := context.WithValue(parent, contextRefKey{}, ref)
	return ref, ctx
}

// FromContext returns the attached root Ref.
func FromContext(parent context.Context) Ref {
	ref, ok := parent.Value(contextRefKey{}).(Ref)
	if !ok {
		return Ref{}
	}
	return ref
}
