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
	"sync"
	"sync/atomic"
	"time"
)

// SpanObserver is the interface plugins must implement if they want to observe
// all spans on a given trace as they happen.
type SpanObserver interface {
	// Start is called when a Span starts
	Start(s *Span)

	// Finish is called when a Span finishes, along with an error if any, whether
	// or not it panicked, and what time it finished.
	Finish(s *Span, err error, panicked bool, finish time.Time)
}

// Trace represents a 'trace' of execution. A 'trace' is the collection of all
// of the 'spans' kicked off from the same root execution context. A trace is
// a concurrency-supporting analog of a stack trace, where a span is somewhat
// like a stack frame.
type Trace struct {
	// sync/atomic things
	spanCount     int64
	spanObservers *spanObserverTuple

	// immutable things from construction
	id int64

	// protected by mtx
	mtx  sync.Mutex
	vals map[interface{}]interface{}
}

// NewTrace creates a new Trace.
func NewTrace(id int64) *Trace {
	return &Trace{id: id}
}

func (t *Trace) getObserver() SpanCtxObserver {
	observers := loadSpanObserverTuple(&t.spanObservers)
	if observers == nil {
		return nil
	}
	if loadSpanObserverTuple(&observers.cdr) == nil {
		return observers.car
	}
	return observers
}

// ObserveSpans lets you register a SpanObserver for all future Spans on the
// Trace. The returned cancel method will remove your observer from the trace.
func (t *Trace) ObserveSpans(observer SpanObserver) (cancel func()) {
	return t.ObserveSpansCtx(spanObserverToSpanCtxObserver{observer: observer})
}

// ObserveSpansCtx lets you register a SpanCtxObserver for all future Spans on the
// Trace. The returned cancel method will remove your observer from the trace.
func (t *Trace) ObserveSpansCtx(observer SpanCtxObserver) (cancel func()) {
	for {
		existing := loadSpanObserverTuple(&t.spanObservers)
		ref := &spanObserverTuple{car: observer, cdr: existing}
		if compareAndSwapSpanObserverTuple(&t.spanObservers, existing, ref) {
			return func() { t.removeObserver(ref) }
		}
	}
}

func (t *Trace) removeObserver(ref *spanObserverTuple) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for {
		if removeObserverFrom(&t.spanObservers, ref) {
			return
		}
	}
}

func removeObserverFrom(parent **spanObserverTuple, ref *spanObserverTuple) (
	success bool) {
	existing := loadSpanObserverTuple(parent)
	if existing == nil {
		return true
	}
	if existing != ref {
		return removeObserverFrom(&existing.cdr, ref)
	}
	return compareAndSwapSpanObserverTuple(parent, existing,
		loadSpanObserverTuple(&existing.cdr))
}

// Id returns the id of the Trace
func (t *Trace) Id() int64 { return t.id }

// GetAll returns values associated with a trace. See SetAll.
func (t *Trace) GetAll() (val map[interface{}]interface{}) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	new := make(map[interface{}]interface{}, len(t.vals))
	for k, v := range t.vals {
		new[k] = v
	}
	return new
}

// Get returns a value associated with a key on a trace. See Set.
func (t *Trace) Get(key interface{}) (val interface{}) {
	t.mtx.Lock()
	if t.vals != nil {
		val = t.vals[key]
	}
	t.mtx.Unlock()
	return val
}

// Set sets a value associated with a key on a trace. See Get.
func (t *Trace) Set(key, val interface{}) {
	t.mtx.Lock()
	if t.vals == nil {
		t.vals = map[interface{}]interface{}{key: val}
	} else {
		t.vals[key] = val
	}
	t.mtx.Unlock()
}

// copyFrom replace all key/value on a trace with a new sets of key/value.
func (t *Trace) copyFrom(s *Trace) {
	vals := s.GetAll()
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.vals = vals
}

func (t *Trace) incrementSpans() { atomic.AddInt64(&t.spanCount, 1) }
func (t *Trace) decrementSpans() { atomic.AddInt64(&t.spanCount, -1) }

// Spans returns the number of spans currently associated with the Trace.
func (t *Trace) Spans() int64 { return atomic.LoadInt64(&t.spanCount) }
