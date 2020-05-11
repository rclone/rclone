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
	"math"
	"sync"
)

// Counter keeps track of running totals, along with the highest and lowest
// values seen. The overall value can increment or decrement. Counter
// implements StatSource. Should be constructed with NewCounter(), though it
// may be more convenient to use the Counter accessor on a given Scope.
// Expected creation is like:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     mon.Counter("beans").Inc(1)
//   }
//
type Counter struct {
	mtx            sync.Mutex
	val, low, high int64
	nonempty       bool
	key            SeriesKey
}

// NewCounter constructs a counter
func NewCounter(key SeriesKey) *Counter {
	return &Counter{key: key}
}

func (c *Counter) set(val int64) {
	c.val = val
	if !c.nonempty || val < c.low {
		c.low = val
	}
	if !c.nonempty || c.high < val {
		c.high = val
	}
	c.nonempty = true
}

// Set will immediately change the value of the counter to whatever val is. It
// will appropriately update the high and low values, and return the former
// value.
func (c *Counter) Set(val int64) (former int64) {
	c.mtx.Lock()
	former = c.val
	c.set(val)
	c.mtx.Unlock()
	return former
}

// Inc will atomically increment the counter by delta and return the new value.
func (c *Counter) Inc(delta int64) (current int64) {
	c.mtx.Lock()
	c.set(c.val + delta)
	current = c.val
	c.mtx.Unlock()
	return current
}

// Dec will atomically decrement the counter by delta and return the new value.
func (c *Counter) Dec(delta int64) (current int64) {
	return c.Inc(-delta)
}

// High returns the highest value seen since construction or the last reset
func (c *Counter) High() (h int64) {
	c.mtx.Lock()
	h = c.high
	c.mtx.Unlock()
	return h
}

// Low returns the lowest value seen since construction or the last reset
func (c *Counter) Low() (l int64) {
	c.mtx.Lock()
	l = c.low
	c.mtx.Unlock()
	return l
}

// Current returns the current value
func (c *Counter) Current() (cur int64) {
	c.mtx.Lock()
	cur = c.val
	c.mtx.Unlock()
	return cur
}

// Reset resets all values including high/low counters and returns what they
// were.
func (c *Counter) Reset() (val, low, high int64) {
	c.mtx.Lock()
	val, low, high = c.val, c.low, c.high
	c.val, c.low, c.high, c.nonempty = 0, 0, 0, false
	c.mtx.Unlock()
	return val, low, high
}

// Stats implements the StatSource interface
func (c *Counter) Stats(cb func(key SeriesKey, field string, val float64)) {
	c.mtx.Lock()
	val, low, high, nonempty := c.val, c.low, c.high, c.nonempty
	c.mtx.Unlock()
	if nonempty {
		cb(c.key, "high", float64(high))
		cb(c.key, "low", float64(low))
	} else {
		cb(c.key, "high", math.NaN())
		cb(c.key, "low", math.NaN())
	}
	cb(c.key, "value", float64(val))
}
