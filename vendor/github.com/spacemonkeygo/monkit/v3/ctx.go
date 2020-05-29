// Copyright (C) 2016 Space Monkey, Inc.
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
	"context"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3/monotime"
)

// Span represents a 'span' of execution. A span is analogous to a stack frame.
// Spans are constructed as a side-effect of Tasks.
type Span struct {
	// sync/atomic things
	mtx spinLock

	// immutable things from construction
	id     int64
	start  time.Time
	f      *Func
	trace  *Trace
	parent *Span
	args   []interface{}
	context.Context

	// protected by mtx
	done        bool
	orphaned    bool
	children    spanBag
	annotations []Annotation
}

// SpanFromCtx loads the current Span from the given context. This assumes
// the context already had a Span created through a Task.
func SpanFromCtx(ctx context.Context) *Span {
	if s, ok := ctx.(*Span); ok && s != nil {
		return s
	} else if s, ok := ctx.Value(spanKey).(*Span); ok && s != nil {
		return s
	}
	return nil
}

func newSpan(ctx context.Context, f *Func, args []interface{},
	id int64, trace *Trace) (sctx context.Context, exit func(*error)) {

	var s, parent *Span
	if s, ok := ctx.(*Span); ok && s != nil {
		ctx = s.Context
		if trace == nil {
			parent = s
			trace = parent.trace
		}
	} else if s, ok := ctx.Value(spanKey).(*Span); ok && s != nil {
		if trace == nil {
			parent = s
			trace = parent.trace
		}
	} else if trace == nil {
		trace = NewTrace(id)
		f.scope.r.observeTrace(trace)
	}

	observer := trace.getObserver()

	s = &Span{
		id:      id,
		start:   monotime.Now(),
		f:       f,
		trace:   trace,
		parent:  parent,
		args:    args,
		Context: ctx,
	}

	trace.incrementSpans()

	if parent != nil {
		f.start(parent.f)
		parent.addChild(s)
	} else {
		f.start(nil)
		f.scope.r.rootSpanStart(s)
	}

	sctx = s
	if observer != nil {
		sctx = observer.Start(sctx, s)
	}

	return sctx, func(errptr *error) {
		rec := recover()
		panicked := rec != nil

		finish := monotime.Now()

		var err error
		if errptr != nil {
			err = *errptr
		}
		s.f.end(err, panicked, finish.Sub(s.start))

		var children []*Span
		s.mtx.Lock()
		s.done = true
		orphaned := s.orphaned
		s.children.Iterate(func(child *Span) {
			children = append(children, child)
		})
		s.mtx.Unlock()
		for _, child := range children {
			child.orphan()
		}

		if s.parent != nil {
			s.parent.removeChild(s)
			if orphaned {
				s.f.scope.r.orphanEnd(s)
			}
		} else {
			s.f.scope.r.rootSpanEnd(s)
		}

		trace.decrementSpans()

		// Re-fetch the observer, in case the value has changed since newSpan
		// was called
		if observer := trace.getObserver(); observer != nil {
			observer.Finish(sctx, s, err, panicked, finish)
		}

		if panicked {
			panic(rec)
		}
	}
}

var taskSecret context.Context = &taskSecretT{}

// Tasks are created (sometimes implicitly) from Funcs. A Task should be called
// at the start of a monitored task, and its return value should be called
// at the stop of said task.
type Task func(ctx *context.Context, args ...interface{}) func(*error)

// Task returns a new Task for use, creating an associated Func if necessary.
// It also adds a new Span to the given ctx during execution. Expected usage
// like:
//
//   var mon = monkit.Package()
//
//   func MyFunc(ctx context.Context, arg1, arg2 string) (err error) {
//     defer mon.Task()(&ctx, arg1, arg2)(&err)
//     ...
//   }
//
// or
//
//   var (
//     mon = monkit.Package()
//     funcTask = mon.Task()
//   )
//
//   func MyFunc(ctx context.Context, arg1, arg2 string) (err error) {
//     defer funcTask(&ctx, arg1, arg2)(&err)
//     ...
//   }
//
// Task allows you to include SeriesTags. WARNING: Each unique tag key/value
// combination creates a unique Func and a unique series. SeriesTags should
// only be used for low-cardinality values that you intentionally wish to
// result in a unique series. Example:
//
//   func MyFunc(ctx context.Context, arg1, arg2 string) (err error) {
//     defer mon.Task(monkit.NewSeriesTag("key1", "val1"))(&ctx)(&err)
//     ...
//   }
//
// Task uses runtime.Caller to determine the associated Func name. See
// TaskNamed if you want to supply your own name. See Func.Task if you already
// have a Func.
//
// If you want to control Trace creation, see Func.ResetTrace and
// Func.RemoteTrace
func (s *Scope) Task(tags ...SeriesTag) Task {
	var initOnce sync.Once
	var f *Func
	init := func() {
		f = s.FuncNamed(callerFunc(3), tags...)
	}
	return Task(func(ctx *context.Context,
		args ...interface{}) func(*error) {
		ctx = cleanCtx(ctx)
		if ctx == &taskSecret && taskArgs(f, args) {
			return nil
		}
		initOnce.Do(init)
		s, exit := newSpan(*ctx, f, args, NewId(), nil)
		if ctx != &unparented {
			*ctx = s
		}
		return exit
	})
}

// Task returns a new Task for use on this Func. It also adds a new Span to
// the given ctx during execution.
//
//   var mon = monkit.Package()
//
//   func MyFunc(ctx context.Context, arg1, arg2 string) (err error) {
//     f := mon.Func()
//     defer f.Task(&ctx, arg1, arg2)(&err)
//     ...
//   }
//
// It's more expected for you to use mon.Task directly. See RemoteTrace or
// ResetTrace if you want greater control over creating new traces.
func (f *Func) Task(ctx *context.Context, args ...interface{}) func(*error) {
	ctx = cleanCtx(ctx)
	if ctx == &taskSecret && taskArgs(f, args) {
		return nil
	}
	s, exit := newSpan(*ctx, f, args, NewId(), nil)
	if ctx != &unparented {
		*ctx = s
	}
	return exit
}

// RemoteTrace is like Func.Task, except you can specify the trace and span id.
// Needed for things like the Zipkin plugin.
func (f *Func) RemoteTrace(ctx *context.Context, spanId int64, trace *Trace,
	args ...interface{}) func(*error) {
	ctx = cleanCtx(ctx)
	if trace != nil {
		f.scope.r.observeTrace(trace)
	}
	s, exit := newSpan(*ctx, f, args, spanId, trace)
	if ctx != &unparented {
		*ctx = s
	}
	return exit
}

// ResetTrace is like Func.Task, except it always creates a new Trace.
func (f *Func) ResetTrace(ctx *context.Context,
	args ...interface{}) func(*error) {
	ctx = cleanCtx(ctx)
	if ctx == &taskSecret && taskArgs(f, args) {
		return nil
	}
	trace := NewTrace(NewId())
	f.scope.r.observeTrace(trace)
	s, exit := newSpan(*ctx, f, args, trace.Id(), trace)
	if ctx != &unparented {
		*ctx = s
	}
	return exit
}

// RestartTrace is like Func.Task, except it always creates a new Trace and inherient
// all tags from the existing trace.
func (f *Func) RestartTrace(ctx *context.Context, args ...interface{}) func(*error) {
	existingSpan := SpanFromCtx(*ctx)
	if existingSpan == nil {
		return f.ResetTrace(ctx, args)
	}
	existingTrace := existingSpan.Trace()
	if existingTrace == nil {
		return f.ResetTrace(ctx, args)
	}

	ctx = cleanCtx(ctx)
	if ctx == &taskSecret && taskArgs(f, args) {
		return nil
	}
	trace := NewTrace(NewId())
	trace.copyFrom(existingTrace)
	f.scope.r.observeTrace(trace)
	s, exit := newSpan(*ctx, f, args, trace.Id(), trace)

	if ctx != &unparented {
		*ctx = s
	}
	return exit
}

var unparented = context.Background()

func cleanCtx(ctx *context.Context) *context.Context {
	if ctx == nil {
		return &unparented
	}
	if *ctx == nil {
		*ctx = context.Background()
		// possible upshot of what we just did:
		//
		//   func MyFunc(ctx context.Context) {
		//     // ctx == nil here
		//     defer mon.Task()(&ctx)(nil)
		//     // ctx != nil here
		//   }
		//
		//   func main() { MyFunc(nil) }
		//
	}
	return ctx
}

// SpanCtxObserver is the interface plugins must implement if they want to observe
// all spans on a given trace as they happen, or add to contexts as they
// pass through mon.Task()(&ctx)(&err) calls.
type SpanCtxObserver interface {
	// Start is called when a Span starts. Start should return the context
	// this span should use going forward. ctx is the context it is currently
	// using.
	Start(ctx context.Context, s *Span) context.Context

	// Finish is called when a Span finishes, along with an error if any, whether
	// or not it panicked, and what time it finished.
	Finish(ctx context.Context, s *Span, err error, panicked bool, finish time.Time)
}

type spanObserverToSpanCtxObserver struct {
	observer SpanObserver
}

func (so spanObserverToSpanCtxObserver) Start(ctx context.Context, s *Span) context.Context {
	so.observer.Start(s)
	return ctx
}

func (so spanObserverToSpanCtxObserver) Finish(ctx context.Context, s *Span, err error, panicked bool, finish time.Time) {
	so.observer.Finish(s, err, panicked, finish)
}

type spanObserverTuple struct {
	// cdr is atomic
	cdr *spanObserverTuple
	// car never changes
	car SpanCtxObserver
}

func (l *spanObserverTuple) Start(ctx context.Context, s *Span) context.Context {
	ctx = l.car.Start(ctx, s)
	cdr := loadSpanObserverTuple(&l.cdr)
	if cdr != nil {
		ctx = cdr.Start(ctx, s)
	}
	return ctx
}

func (l *spanObserverTuple) Finish(ctx context.Context, s *Span, err error, panicked bool,
	finish time.Time) {
	l.car.Finish(ctx, s, err, panicked, finish)
	cdr := loadSpanObserverTuple(&l.cdr)
	if cdr != nil {
		cdr.Finish(ctx, s, err, panicked, finish)
	}
}

type resetContext struct {
	context.Context
}

func (r resetContext) Value(key interface{}) interface{} {
	if key == spanKey {
		return nil
	}
	return r.Context.Value(key)
}

// ResetContextSpan returns a new context with Span information removed.
func ResetContextSpan(ctx context.Context) context.Context {
	return resetContext{Context: ctx}
}
