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
)

// IntVal is a convenience wrapper around an IntDist. Constructed using
// NewIntVal, though its expected usage is like:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     ...
//     mon.IntVal("size").Observe(val)
//     ...
//   }
//
type IntVal struct {
	mtx  sync.Mutex
	dist IntDist
}

// NewIntVal creates an IntVal
func NewIntVal(key SeriesKey) (v *IntVal) {
	v = &IntVal{}
	initIntDist(&v.dist, key)
	return v
}

// Observe observes an integer value
func (v *IntVal) Observe(val int64) {
	v.mtx.Lock()
	v.dist.Insert(val)
	v.mtx.Unlock()
}

// Stats implements the StatSource interface.
func (v *IntVal) Stats(cb func(key SeriesKey, field string, val float64)) {
	v.mtx.Lock()
	vd := v.dist.Copy()
	v.mtx.Unlock()

	vd.Stats(cb)
}

// Quantile returns an estimate of the requested quantile of observed values.
// 0 <= quantile <= 1
func (v *IntVal) Quantile(quantile float64) (rv int64) {
	v.mtx.Lock()
	rv = v.dist.Query(quantile)
	v.mtx.Unlock()
	return rv
}

// FloatVal is a convenience wrapper around an FloatDist. Constructed using
// NewFloatVal, though its expected usage is like:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     ...
//     mon.FloatVal("size").Observe(val)
//     ...
//   }
//
type FloatVal struct {
	mtx  sync.Mutex
	dist FloatDist
}

// NewFloatVal creates a FloatVal
func NewFloatVal(key SeriesKey) (v *FloatVal) {
	v = &FloatVal{}
	initFloatDist(&v.dist, key)
	return v
}

// Observe observes an floating point value
func (v *FloatVal) Observe(val float64) {
	v.mtx.Lock()
	v.dist.Insert(val)
	v.mtx.Unlock()
}

// Stats implements the StatSource interface.
func (v *FloatVal) Stats(cb func(key SeriesKey, field string, val float64)) {
	v.mtx.Lock()
	vd := v.dist.Copy()
	v.mtx.Unlock()

	vd.Stats(cb)
}

// Quantile returns an estimate of the requested quantile of observed values.
// 0 <= quantile <= 1
func (v *FloatVal) Quantile(quantile float64) (rv float64) {
	v.mtx.Lock()
	rv = v.dist.Query(quantile)
	v.mtx.Unlock()
	return rv
}

// BoolVal keeps statistics about boolean values. It keeps the number of trues,
// number of falses, and the disposition (number of trues minus number of
// falses). Constructed using NewBoolVal, though its expected usage is like:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     ...
//     mon.BoolVal("flipped").Observe(bool)
//     ...
//   }
//
type BoolVal struct {
	trues  int64
	falses int64
	recent int32
	key    SeriesKey
}

// NewBoolVal creates a BoolVal
func NewBoolVal(key SeriesKey) *BoolVal {
	return &BoolVal{key: key}
}

// Observe observes a boolean value
func (v *BoolVal) Observe(val bool) {
	if val {
		atomic.AddInt64(&v.trues, 1)
		atomic.StoreInt32(&v.recent, 1)
	} else {
		atomic.AddInt64(&v.falses, 1)
		atomic.StoreInt32(&v.recent, 0)
	}
}

// Stats implements the StatSource interface.
func (v *BoolVal) Stats(cb func(key SeriesKey, field string, val float64)) {
	trues := atomic.LoadInt64(&v.trues)
	falses := atomic.LoadInt64(&v.falses)
	recent := atomic.LoadInt32(&v.recent)
	cb(v.key, "disposition", float64(trues-falses))
	cb(v.key, "false", float64(falses))
	cb(v.key, "recent", float64(recent))
	cb(v.key, "true", float64(trues))
}

// StructVal keeps track of a structure of data. Constructed using
// NewStructVal, though its expected usage is like:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     ...
//     mon.StructVal("stats").Observe(stats)
//     ...
//   }
//
type StructVal struct {
	mtx    sync.Mutex
	recent interface{}
	key    SeriesKey
}

// NewStructVal creates a StructVal
func NewStructVal(key SeriesKey) *StructVal {
	return &StructVal{key: key}
}

// Observe observes a struct value. Only the fields convertable to float64 will
// be monitored. A reference to the most recently called Observe value is kept
// for reading when Stats is called.
func (v *StructVal) Observe(val interface{}) {
	v.mtx.Lock()
	v.recent = val
	v.mtx.Unlock()
}

// Stats implements the StatSource interface.
func (v *StructVal) Stats(cb func(key SeriesKey, field string, val float64)) {
	v.mtx.Lock()
	recent := v.recent
	v.mtx.Unlock()

	if recent != nil {
		StatSourceFromStruct(v.key, recent).Stats(cb)
	}
}
