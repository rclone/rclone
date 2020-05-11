// Copyright (C) 2015 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monkit

import (
	"sort"
	"sync"
)

type traceWatcherRef struct {
	watcher func(*Trace)
}

// Registry encapsulates all of the top-level state for a monitoring system.
// In general, only the Default registry is ever used.
type Registry struct {
	// sync/atomic things
	traceWatcher *traceWatcherRef

	watcherMtx     sync.Mutex
	watcherCounter int64
	traceWatchers  map[int64]func(*Trace)

	scopeMtx sync.Mutex
	scopes   map[string]*Scope

	spanMtx sync.Mutex
	spans   map[*Span]struct{}

	orphanMtx sync.Mutex
	orphans   map[*Span]struct{}
}

// NewRegistry creates a NewRegistry, though you almost certainly just want
// to use Default.
func NewRegistry() *Registry {
	return &Registry{
		traceWatchers: map[int64]func(*Trace){},
		scopes:        map[string]*Scope{},
		spans:         map[*Span]struct{}{},
		orphans:       map[*Span]struct{}{}}
}

// Package creates a new monitoring Scope, named after the top level package.
// It's expected that you'll have something like
//
//   var mon = monkit.Package()
//
// at the top of each package.
func (r *Registry) Package() *Scope {
	return r.ScopeNamed(callerPackage(1))
}

// ScopeNamed is like Package, but lets you choose the name.
func (r *Registry) ScopeNamed(name string) *Scope {
	r.scopeMtx.Lock()
	defer r.scopeMtx.Unlock()
	s, exists := r.scopes[name]
	if exists {
		return s
	}
	s = newScope(r, name)
	r.scopes[name] = s
	return s
}

func (r *Registry) observeTrace(t *Trace) {
	watcher := loadTraceWatcherRef(&r.traceWatcher)
	if watcher != nil {
		watcher.watcher(t)
	}
}

func (r *Registry) updateWatcher() {
	cbs := make([]func(*Trace), 0, len(r.traceWatchers))
	for _, cb := range r.traceWatchers {
		cbs = append(cbs, cb)
	}
	switch len(cbs) {
	case 0:
		storeTraceWatcherRef(&r.traceWatcher, nil)
	case 1:
		storeTraceWatcherRef(&r.traceWatcher,
			&traceWatcherRef{watcher: cbs[0]})
	default:
		storeTraceWatcherRef(&r.traceWatcher,
			&traceWatcherRef{watcher: func(t *Trace) {
				for _, cb := range cbs {
					cb(t)
				}
			}})
	}
}

// ObserveTraces lets you observe all traces flowing through the system.
// The passed in callback 'cb' will be called for every new trace as soon as
// it starts, until the returned cancel method is called.
// Note: this only applies to all new traces. If you want to find existing
// or running traces, please pull them off of live RootSpans.
func (r *Registry) ObserveTraces(cb func(*Trace)) (cancel func()) {
	// even though observeTrace doesn't get a mutex, it's only ever loading
	// the traceWatcher pointer, so we can use this mutex here to safely
	// coordinate the setting of the traceWatcher pointer.
	r.watcherMtx.Lock()
	defer r.watcherMtx.Unlock()

	cbId := r.watcherCounter
	r.watcherCounter += 1
	r.traceWatchers[cbId] = cb
	r.updateWatcher()

	return func() {
		r.watcherMtx.Lock()
		defer r.watcherMtx.Unlock()
		delete(r.traceWatchers, cbId)
		r.updateWatcher()
	}
}

func (r *Registry) rootSpanStart(s *Span) {
	r.spanMtx.Lock()
	r.spans[s] = struct{}{}
	r.spanMtx.Unlock()
}

func (r *Registry) rootSpanEnd(s *Span) {
	r.spanMtx.Lock()
	delete(r.spans, s)
	r.spanMtx.Unlock()
}

func (r *Registry) orphanedSpan(s *Span) {
	r.orphanMtx.Lock()
	r.orphans[s] = struct{}{}
	r.orphanMtx.Unlock()
}

func (r *Registry) orphanEnd(s *Span) {
	r.orphanMtx.Lock()
	delete(r.orphans, s)
	r.orphanMtx.Unlock()
}

// RootSpans will call 'cb' on all currently executing Spans with no live or
// reachable parent. See also AllSpans.
func (r *Registry) RootSpans(cb func(s *Span)) {
	r.spanMtx.Lock()
	spans := make([]*Span, 0, len(r.spans))
	for s := range r.spans {
		spans = append(spans, s)
	}
	r.spanMtx.Unlock()
	r.orphanMtx.Lock()
	orphans := make([]*Span, 0, len(r.orphans))
	for s := range r.orphans {
		orphans = append(orphans, s)
	}
	r.orphanMtx.Unlock()
	spans = append(spans, orphans...)
	sort.Sort(spanSorter(spans))
	for _, s := range spans {
		cb(s)
	}
}

func walkSpan(s *Span, cb func(s *Span)) {
	cb(s)
	s.Children(func(s *Span) {
		walkSpan(s, cb)
	})
}

// AllSpans calls 'cb' on all currently known Spans. See also RootSpans.
func (r *Registry) AllSpans(cb func(s *Span)) {
	r.RootSpans(func(s *Span) { walkSpan(s, cb) })
}

// Scopes calls 'cb' on all currently known Scopes.
func (r *Registry) Scopes(cb func(s *Scope)) {
	r.scopeMtx.Lock()
	c := make([]*Scope, 0, len(r.scopes))
	for _, s := range r.scopes {
		c = append(c, s)
	}
	r.scopeMtx.Unlock()
	sort.Sort(scopeSorter(c))
	for _, s := range c {
		cb(s)
	}
}

// Funcs calls 'cb' on all currently known Funcs.
func (r *Registry) Funcs(cb func(f *Func)) {
	r.Scopes(func(s *Scope) { s.Funcs(cb) })
}

// Stats implements the StatSource interface.
func (r *Registry) Stats(cb func(key SeriesKey, field string, val float64)) {
	r.Scopes(func(s *Scope) {
		s.Stats(func(key SeriesKey, field string, val float64) {
			cb(key.WithTag("scope", s.name), field, val)
		})
	})
}

var _ StatSource = (*Registry)(nil)

// Default is the default Registry
var Default = NewRegistry()

// ScopeNamed is just a wrapper around Default.ScopeNamed
func ScopeNamed(name string) *Scope { return Default.ScopeNamed(name) }

// RootSpans is just a wrapper around Default.RootSpans
func RootSpans(cb func(s *Span)) { Default.RootSpans(cb) }

// Scopes is just a wrapper around Default.Scopes
func Scopes(cb func(s *Scope)) { Default.Scopes(cb) }

// Funcs is just a wrapper around Default.Funcs
func Funcs(cb func(f *Func)) { Default.Funcs(cb) }

// Package is just a wrapper around Default.Package
func Package() *Scope { return Default.ScopeNamed(callerPackage(1)) }

// Stats is just a wrapper around Default.Stats
func Stats(cb func(key SeriesKey, field string, val float64)) { Default.Stats(cb) }

type spanSorter []*Span

func (s spanSorter) Len() int      { return len(s) }
func (s spanSorter) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s spanSorter) Less(i, j int) bool {
	ispan, jspan := s[i], s[j]
	iname, jname := ispan.f.FullName(), jspan.f.FullName()
	return (iname < jname) || (iname == jname && ispan.id < jspan.id)
}

type scopeSorter []*Scope

func (s scopeSorter) Len() int           { return len(s) }
func (s scopeSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s scopeSorter) Less(i, j int) bool { return s[i].name < s[j].name }
