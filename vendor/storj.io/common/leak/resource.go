// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Package leak provides a way to track resources when race detector is enabled.
package leak

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// Resource implements tracking a nested resources.
//
// Any child resource must be closed before the parent, otherwise
// closing the parent returns an error.
//
// Note, in most cases using [Ref] is preferred as it won't affect
// performance for production builds.
type Resource struct {
	name    string
	parent  *Resource
	callers frames

	mu   sync.Mutex
	open map[*Resource]struct{}
}

type frames [5]uintptr

func callers(skipCallers int) frames {
	var fs frames
	runtime.Callers(skipCallers+1, fs[:])
	return fs
}

// RootResource returns a root resource tracker.
func RootResource(skipCallers int) *Resource {
	return &Resource{
		name:    "root",
		callers: callers(skipCallers + 1),
		open:    map[*Resource]struct{}{},
	}
}

// Child returns a child resource.
func (parent *Resource) Child(name string, skipCallers int) *Resource {
	child := RootResource(skipCallers + 1)
	child.name = name
	child.parent = parent
	parent.add(child)
	return child
}

func (parent *Resource) add(child *Resource) {
	parent.mu.Lock()
	defer parent.mu.Unlock()

	parent.open[child] = struct{}{}
}

func (parent *Resource) del(child *Resource) {
	parent.mu.Lock()
	defer parent.mu.Unlock()

	delete(parent.open, child)
}

// StartStack returns formatted stack where the resource was created.
func (parent *Resource) StartStack() string {
	return parent.callers.String()
}

// Close closes the resource and checks whether all the children have been closed.
func (parent *Resource) Close() error {
	if parent.parent != nil {
		parent.parent.del(parent)
	}

	// the common case
	if len(parent.open) == 0 {
		return nil
	}

	// some children haven't been freed

	var s strings.Builder
	fmt.Fprintf(&s, "--- Resource %s started at ---\n", parent.name)
	fmt.Fprint(&s, parent.callers.String())

	type tag struct {
		name    string
		callers frames
	}

	unique := map[tag]int{}
	for r := range parent.open {
		unique[tag{name: r.name, callers: r.callers}]++
	}

	for r, count := range unique {
		fmt.Fprintf(&s, "--- Unclosed %s opened from (count=%d) ---\n", r.name, count)
		fmt.Fprint(&s, r.callers.String())
	}

	fmt.Fprintf(&s, "--- Closing called from ---\n")
	closingFrames := callers(2)
	fmt.Fprintf(&s, "%s", closingFrames.String())

	return errors.New(s.String())
}

// String implements fmt.Stringer.
func (fs *frames) String() string {
	var s strings.Builder
	frames := runtime.CallersFrames((*fs)[:])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.File, "runtime/") {
			break
		}
		fmt.Fprintf(&s, "%s\n", frame.Function)
		fmt.Fprintf(&s, "\t%s:%d\n", frame.File, frame.Line)
		if !more {
			break
		}
	}
	return s.String()
}
