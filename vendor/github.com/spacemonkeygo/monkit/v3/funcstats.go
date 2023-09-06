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
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3/monotime"
)

// FuncStats keeps track of statistics about a possible function's execution.
// Should be created with NewFuncStats, though expected creation is through a
// Func object:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     f := mon.Func()
//     ...
//   }
//
type FuncStats struct {
	// sync/atomic things
	current         int64
	highwater       int64
	parentsAndMutex funcSet

	// mutex things (reuses mutex from parents)
	errors       map[string]int64
	panics       int64
	successTimes DurationDist
	failureTimes DurationDist
	key          SeriesKey
}

func initFuncStats(f *FuncStats, key SeriesKey) {
	f.key = key
	f.errors = map[string]int64{}

	key.Measurement += "_times"
	initDurationDist(&f.successTimes, key.WithTag("kind", "success"))
	initDurationDist(&f.failureTimes, key.WithTag("kind", "failure"))
}

// NewFuncStats creates a FuncStats
func NewFuncStats(key SeriesKey) (f *FuncStats) {
	f = &FuncStats{}
	initFuncStats(f, key)
	return f
}

// Reset resets all recorded data.
func (f *FuncStats) Reset() {
	atomic.StoreInt64(&f.current, 0)
	atomic.StoreInt64(&f.highwater, 0)
	f.parentsAndMutex.Lock()
	f.errors = make(map[string]int64, len(f.errors))
	f.panics = 0
	f.successTimes.Reset()
	f.failureTimes.Reset()
	f.parentsAndMutex.Unlock()
}

func (f *FuncStats) start(parent *Func) {
	f.parentsAndMutex.Add(parent)
	current := atomic.AddInt64(&f.current, 1)
	for {
		highwater := atomic.LoadInt64(&f.highwater)
		if current <= highwater ||
			atomic.CompareAndSwapInt64(&f.highwater, highwater, current) {
			break
		}
	}
}

func (f *FuncStats) end(err error, panicked bool, duration time.Duration) {
	atomic.AddInt64(&f.current, -1)
	f.parentsAndMutex.Lock()
	if panicked {
		f.panics += 1
		f.failureTimes.Insert(duration)
		f.parentsAndMutex.Unlock()
		return
	}
	if err == nil {
		f.successTimes.Insert(duration)
		f.parentsAndMutex.Unlock()
		return
	}
	f.failureTimes.Insert(duration)
	f.errors[getErrorName(err)] += 1
	f.parentsAndMutex.Unlock()
}

// Current returns how many concurrent instances of this function are currently
// being observed.
func (f *FuncStats) Current() int64 { return atomic.LoadInt64(&f.current) }

// Highwater returns the highest value Current() would ever return.
func (f *FuncStats) Highwater() int64 { return atomic.LoadInt64(&f.highwater) }

// Success returns the number of successes that have been observed
func (f *FuncStats) Success() (rv int64) {
	f.parentsAndMutex.Lock()
	rv = f.successTimes.Count
	f.parentsAndMutex.Unlock()
	return rv
}

// Panics returns the number of panics that have been observed
func (f *FuncStats) Panics() (rv int64) {
	f.parentsAndMutex.Lock()
	rv = f.panics
	f.parentsAndMutex.Unlock()
	return rv
}

// Errors returns the number of errors observed by error type. The error type
// is determined by handlers from AddErrorNameHandler, or a default that works
// with most error types.
func (f *FuncStats) Errors() (rv map[string]int64) {
	f.parentsAndMutex.Lock()
	rv = make(map[string]int64, len(f.errors))
	for errname, count := range f.errors {
		rv[errname] = count
	}
	f.parentsAndMutex.Unlock()
	return rv
}

func (f *FuncStats) parents(cb func(f *Func)) {
	f.parentsAndMutex.Iterate(cb)
}

// Stats implements the StatSource interface
func (f *FuncStats) Stats(cb func(key SeriesKey, field string, val float64)) {
	cb(f.key, "current", float64(f.Current()))
	cb(f.key, "highwater", float64(f.Highwater()))

	f.parentsAndMutex.Lock()
	panics := f.panics
	errs := make(map[string]int64, len(f.errors))
	for errname, count := range f.errors {
		errs[errname] = count
	}
	st := f.successTimes.Copy()
	ft := f.failureTimes.Copy()
	f.parentsAndMutex.Unlock()

	cb(f.key, "successes", float64(st.Count))
	e_count := int64(0)
	for errname, count := range errs {
		e_count += count
		cb(f.key.WithTag("error_name", errname), "count", float64(count))
	}
	cb(f.key, "errors", float64(e_count))
	cb(f.key, "panics", float64(panics))
	cb(f.key, "failures", float64(e_count+panics))
	cb(f.key, "total", float64(st.Count+e_count+panics))

	st.Stats(cb)
	ft.Stats(cb)
}

// SuccessTimes returns a DurationDist of successes
func (f *FuncStats) SuccessTimes() *DurationDist {
	f.parentsAndMutex.Lock()
	d := f.successTimes.Copy()
	f.parentsAndMutex.Unlock()
	return d
}

// FailureTimes returns a DurationDist of failures (includes panics and errors)
func (f *FuncStats) FailureTimes() *DurationDist {
	f.parentsAndMutex.Lock()
	d := f.failureTimes.Copy()
	f.parentsAndMutex.Unlock()
	return d
}

// Observe starts the stopwatch for observing this function and returns a
// function to be called at the end of the function execution. Expected usage
// like:
//
//   func MyFunc() (err error) {
//     defer funcStats.Observe()(&err)
//     ...
//   }
//
func (f *FuncStats) Observe() func(errptr *error) {
	f.start(nil)
	start := monotime.Now()
	return func(errptr *error) {
		rec := recover()
		panicked := rec != nil
		finish := monotime.Now()
		var err error
		if errptr != nil {
			err = *errptr
		}
		f.end(err, panicked, finish.Sub(start))
		if panicked {
			panic(rec)
		}
	}
}
