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
	"time"
)

const (
	ReservoirSize = 64
)

var (
	// If Window is > 0, the probability of replacing a datapoint will never
	// fall below ReservoirSize/Window instead of continuing to fall over time.
	// Window should be a multiple of ReservoirSize.
	Window int64 = 1024
)

// ObservedQuantiles is the set of quantiles the internal distribution
// measurement logic will try to optimize for, if applicable.
var ObservedQuantiles = []float64{0, .1, .25, .5, .75, .9, .95, 1}

type float32Slice []float32

func (p float32Slice) Len() int      { return len(p) }
func (p float32Slice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p float32Slice) Less(i, j int) bool {
	// N.B.: usually, float comparisons should check if either value is NaN, but
	// in this package's usage, they never are here.
	return p[i] < p[j]
}

//go:generate sh -c "m4 -D_IMPORT_='\"time\"' -D_NAME_=Duration -D_LOWER_NAME_=duration -D_TYPE_=time.Duration distgen.go.m4 > durdist.go"
//go:generate sh -c "m4 -D_IMPORT_= -D_NAME_=Float -D_LOWER_NAME_=float -D_TYPE_=float64 distgen.go.m4 > floatdist.go"
//go:generate sh -c "m4 -D_IMPORT_= -D_NAME_=Int -D_LOWER_NAME_=int -D_TYPE_=int64 distgen.go.m4 > intdist.go"
//go:generate gofmt -w -s durdist.go floatdist.go intdist.go

func (d *DurationDist) toFloat64(v time.Duration) float64 {
	return v.Seconds()
}

func (d *IntDist) toFloat64(v int64) float64 {
	return float64(v)
}

func (d *FloatDist) toFloat64(v float64) float64 {
	return v
}
