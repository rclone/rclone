// Copyright (C) 2021 Storj Labs, Inc.
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
)

// CallbackTransformer will take a provided callback and return a transformed one.
type CallbackTransformer interface {
	Transform(func(SeriesKey, string, float64)) func(SeriesKey, string, float64)
}

// CallbackTransformerFunc is a single function that implements
// CallbackTransformer's Transform.
type CallbackTransformerFunc func(func(SeriesKey, string, float64)) func(SeriesKey, string, float64)

// Transform implements CallbackTransformer.
func (f CallbackTransformerFunc) Transform(cb func(SeriesKey, string, float64)) func(SeriesKey, string, float64) {
	return f(cb)
}

// TransformStatSource will make sure that a StatSource has the provided
// CallbackTransformers applied to callbacks given to the StatSource.
func TransformStatSource(s StatSource, transformers ...CallbackTransformer) StatSource {
	return StatSourceFunc(func(cb func(key SeriesKey, field string, val float64)) {
		for _, t := range transformers {
			cb = t.Transform(cb)
		}
		s.Stats(cb)
	})
}

// DeltaTransformer calculates deltas from any total fields. It keeps internal
// state to keep track of the previous totals, so care should be taken to use
// a different DeltaTransformer per output.
type DeltaTransformer struct {
	mtx        sync.Mutex
	lastTotals map[string]float64
}

// NewDeltaTransformer creates a new DeltaTransformer with its own idea of the
// last totals seen.
func NewDeltaTransformer() *DeltaTransformer {
	return &DeltaTransformer{lastTotals: map[string]float64{}}
}

// Transform implements CallbackTransformer.
func (dt *DeltaTransformer) Transform(cb func(SeriesKey, string, float64)) func(SeriesKey, string, float64) {
	return func(key SeriesKey, field string, val float64) {
		if field != "total" {
			cb(key, field, val)
			return
		}

		mapIndex := key.WithField(field)

		dt.mtx.Lock()
		lastTotal, found := dt.lastTotals[mapIndex]
		dt.lastTotals[mapIndex] = val
		dt.mtx.Unlock()

		cb(key, field, val)
		if found {
			cb(key, "delta", val-lastTotal)
		}
	}
}
