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
	"time"

	"github.com/spacemonkeygo/monkit/v3/monotime"
)

const (
	ticksToKeep = 24
	timePerTick = 10 * time.Minute
)

var (
	defaultTicker = ticker{}
)

type meterBucket struct {
	count int64
	start time.Time
}

// Meter keeps track of events and their rates over time.
// Implements the StatSource interface. You should construct using NewMeter,
// though expected usage is like:
//
//   var (
//     mon   = monkit.Package()
//     meter = mon.Meter("meter")
//   )
//
//   func MyFunc() {
//     ...
//     meter.Mark(4) // 4 things happened
//     ...
//   }
//
type Meter struct {
	mtx    sync.Mutex
	total  int64
	slices [ticksToKeep]meterBucket
	key    SeriesKey
}

// NewMeter constructs a Meter
func NewMeter(key SeriesKey) *Meter {
	rv := &Meter{key: key}
	now := monotime.Now()
	for i := 0; i < ticksToKeep; i++ {
		rv.slices[i].start = now
	}
	defaultTicker.register(rv)
	return rv
}

// Reset resets all internal state.
//
// Useful when monitoring a counter that has overflowed.
func (e *Meter) Reset(new_total int64) {
	e.mtx.Lock()
	e.total = new_total
	now := monotime.Now()
	for _, slice := range e.slices {
		slice.count = 0
		slice.start = now
	}
	e.mtx.Unlock()
}

// SetTotal sets the initial total count of the meter.
func (e *Meter) SetTotal(total int64) {
	e.mtx.Lock()
	e.total = total
	e.mtx.Unlock()
}

// Mark marks amount events occurring in the current time window.
func (e *Meter) Mark(amount int) {
	e.mtx.Lock()
	e.slices[ticksToKeep-1].count += int64(amount)
	e.mtx.Unlock()
}

// Mark64 marks amount events occurring in the current time window (int64 version).
func (e *Meter) Mark64(amount int64) {
	e.mtx.Lock()
	e.slices[ticksToKeep-1].count += amount
	e.mtx.Unlock()
}

func (e *Meter) tick(now time.Time) {
	e.mtx.Lock()
	// only advance meter buckets if something happened. otherwise
	// rare events will always just have zero rates.
	if e.slices[ticksToKeep-1].count != 0 {
		e.total += e.slices[0].count
		copy(e.slices[:], e.slices[1:])
		e.slices[ticksToKeep-1] = meterBucket{count: 0, start: now}
	}
	e.mtx.Unlock()
}

func (e *Meter) stats(now time.Time) (rate float64, total int64) {
	current := int64(0)
	e.mtx.Lock()
	start := e.slices[0].start
	for i := 0; i < ticksToKeep; i++ {
		current += e.slices[i].count
	}
	total = e.total
	e.mtx.Unlock()
	total += current
	duration := now.Sub(start).Seconds()
	if duration > 0 {
		rate = float64(current) / duration
	} else {
		rate = 0
	}
	return rate, total
}

// Rate returns the rate over the internal sliding window
func (e *Meter) Rate() float64 {
	rate, _ := e.stats(monotime.Now())
	return rate
}

// Total returns the total over the internal sliding window
func (e *Meter) Total() float64 {
	_, total := e.stats(monotime.Now())
	return float64(total)
}

// Stats implements the StatSource interface
func (e *Meter) Stats(cb func(key SeriesKey, field string, val float64)) {
	rate, total := e.stats(monotime.Now())
	cb(e.key, "rate", rate)
	cb(e.key, "total", float64(total))
}

// DiffMeter is a StatSource that shows the difference between
// the rates of two meters. Expected usage like:
//
//   var (
//     mon = monkit.Package()
//     herps = mon.Meter("herps")
//     derps = mon.Meter("derps")
//     herpToDerp = mon.DiffMeter("herp_to_derp", herps, derps)
//   )
//
type DiffMeter struct {
	meter1, meter2 *Meter
	key            SeriesKey
}

// Constructs a DiffMeter.
func NewDiffMeter(key SeriesKey, meter1, meter2 *Meter) *DiffMeter {
	return &DiffMeter{key: key, meter1: meter1, meter2: meter2}
}

// Stats implements the StatSource interface
func (m *DiffMeter) Stats(cb func(key SeriesKey, field string, val float64)) {
	now := monotime.Now()
	rate1, total1 := m.meter1.stats(now)
	rate2, total2 := m.meter2.stats(now)
	cb(m.key, "rate", rate1-rate2)
	cb(m.key, "total", float64(total1-total2))
}

type ticker struct {
	mtx     sync.Mutex
	started bool
	meters  []*Meter
}

func (t *ticker) register(m *Meter) {
	t.mtx.Lock()
	if !t.started {
		t.started = true
		go t.run()
	}
	t.meters = append(t.meters, m)
	t.mtx.Unlock()
}

func (t *ticker) run() {
	for {
		time.Sleep(timePerTick)
		t.mtx.Lock()
		meters := t.meters // this is safe since we only use append
		t.mtx.Unlock()
		now := monotime.Now()
		for _, m := range meters {
			m.tick(now)
		}
	}
}
