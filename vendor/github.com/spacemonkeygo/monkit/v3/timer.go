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
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3/monotime"
)

// Timer is a threadsafe convenience wrapper around a DurationDist. You should
// construct with NewTimer(), though the expected usage is from a Scope like
// so:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     ...
//     timer := mon.Timer("event")
//     // perform event
//     timer.Stop()
//     ...
//   }
//
// Timers implement StatSource.
type Timer struct {
	mtx   sync.Mutex
	times *DurationDist
}

// NewTimer constructs a new Timer.
func NewTimer(key SeriesKey) *Timer {
	return &Timer{times: NewDurationDist(key)}
}

// Start constructs a RunningTimer
func (t *Timer) Start() *RunningTimer {
	return &RunningTimer{
		start: monotime.Now(),
		t:     t}
}

// RunningTimer should be constructed from a Timer.
type RunningTimer struct {
	start   time.Time
	t       *Timer
	stopped bool
}

// Elapsed just returns the amount of time since the timer started
func (r *RunningTimer) Elapsed() time.Duration {
	return time.Since(r.start)
}

// Stop stops the timer, adds the duration to the statistics information, and
// returns the elapsed time.
func (r *RunningTimer) Stop() time.Duration {
	elapsed := r.Elapsed()
	r.t.mtx.Lock()
	if !r.stopped {
		r.t.times.Insert(elapsed)
		r.stopped = true
	}
	r.t.mtx.Unlock()
	return elapsed
}

// Values returns the main timer values
func (t *Timer) Values() *DurationDist {
	t.mtx.Lock()
	rv := t.times.Copy()
	t.mtx.Unlock()
	return rv
}

// Stats implements the StatSource interface
func (t *Timer) Stats(cb func(key SeriesKey, field string, val float64)) {
	t.mtx.Lock()
	times := t.times.Copy()
	t.mtx.Unlock()

	times.Stats(cb)
}
