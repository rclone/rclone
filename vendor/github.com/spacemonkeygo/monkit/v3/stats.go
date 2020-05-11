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
	"strings"
)

// SeriesKey represents an individual time series for monkit to output.
type SeriesKey struct {
	Measurement string
	Tags        *TagSet
}

// NewSeriesKey constructs a new series with the minimal fields.
func NewSeriesKey(measurement string) SeriesKey {
	return SeriesKey{Measurement: measurement}
}

// WithTag returns a copy of the SeriesKey with the tag set
func (s SeriesKey) WithTag(key, value string) SeriesKey {
	s.Tags = s.Tags.Set(key, value)
	return s
}

// String returns a string representation of the series. For example, it returns
// something like `measurement,tag0=val0,tag1=val1`.
func (s SeriesKey) String() string {
	var builder strings.Builder
	writeMeasurement(&builder, s.Measurement)
	if s.Tags.Len() > 0 {
		builder.WriteByte(',')
		builder.WriteString(s.Tags.String())
	}
	return builder.String()
}

func (s SeriesKey) WithField(field string) string {
	var builder strings.Builder
	builder.WriteString(s.String())
	builder.WriteByte(' ')
	writeTag(&builder, field)
	return builder.String()
}

// StatSource represents anything that can return named floating point values.
type StatSource interface {
	Stats(cb func(key SeriesKey, field string, val float64))
}

type StatSourceFunc func(cb func(key SeriesKey, field string, val float64))

func (f StatSourceFunc) Stats(cb func(key SeriesKey, field string, val float64)) {
	f(cb)
}

// Collect takes something that implements the StatSource interface and returns
// a key/value map.
func Collect(mon StatSource) map[string]float64 {
	rv := make(map[string]float64)
	mon.Stats(func(key SeriesKey, field string, val float64) {
		rv[key.WithField(field)] = val
	})
	return rv
}
