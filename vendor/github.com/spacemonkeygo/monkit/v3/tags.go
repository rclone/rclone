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
	"sort"
	"strings"
)

// SeriesTag is a key/value pair. When used with a measurement name, each set
// of unique key/value pairs represents a new unique series.
type SeriesTag struct {
	Key, Val string
}

// NewTag creates a new tag
func NewSeriesTag(key, val string) SeriesTag {
	return SeriesTag{key, val}
}

// TagSet is an immutible collection of tag, value pairs.
type TagSet struct {
	all map[string]string
	str string // cached string form
}

// Get returns the value associated with the key.
func (t *TagSet) Get(key string) string {
	if t == nil || t.all == nil {
		return ""
	}
	return t.all[key]
}

// All returns a map of all the key/value pairs in the tag set. It
// should not be modified.
func (t *TagSet) All() map[string]string {
	if t == nil {
		return nil
	}
	return t.all
}

// Len returns the number of tags in the tag set.
func (t *TagSet) Len() int {
	if t == nil {
		return 0
	}
	return len(t.all)
}

// Set returns a new tag set with the key associated to the value.
func (t *TagSet) Set(key, value string) *TagSet {
	return t.SetAll(map[string]string{key: value})
}

// SetAll returns a new tag set with the key value pairs in the map all set.
func (t *TagSet) SetAll(kvs map[string]string) *TagSet {
	all := make(map[string]string)
	if t != nil {
		for key, value := range t.all {
			all[key] = value
		}
	}
	for key, value := range kvs {
		all[key] = value
	}
	return &TagSet{all: all}
}

// String returns a string form of the tag set suitable for sending to influxdb.
func (t *TagSet) String() string {
	if t == nil {
		return ""
	}
	if t.str == "" {
		var builder strings.Builder
		t.writeTags(&builder)
		t.str = builder.String()
	}
	return t.str
}

// writeTags writes the tags in the tag set to the builder.
func (t *TagSet) writeTags(builder *strings.Builder) {
	type kv struct {
		key   string
		value string
	}
	var kvs []kv

	for key, value := range t.all {
		kvs = append(kvs, kv{key, value})
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].key < kvs[j].key
	})

	for i, kv := range kvs {
		if i > 0 {
			builder.WriteByte(',')
		}
		writeTag(builder, kv.key)
		builder.WriteByte('=')
		writeTag(builder, kv.value)
	}
}

// writeMeasurement writes a measurement to the builder.
func writeMeasurement(builder *strings.Builder, measurement string) {
	if strings.IndexByte(measurement, ',') == -1 &&
		strings.IndexByte(measurement, ' ') == -1 {

		builder.WriteString(measurement)
		return
	}

	for i := 0; i < len(measurement); i++ {
		if measurement[i] == ',' ||
			measurement[i] == ' ' {
			builder.WriteByte('\\')
		}
		builder.WriteByte(measurement[i])
	}
}

// writeTag writes a tag key, value, or field key to the builder.
func writeTag(builder *strings.Builder, tag string) {
	if strings.IndexByte(tag, ',') == -1 &&
		strings.IndexByte(tag, '=') == -1 &&
		strings.IndexByte(tag, ' ') == -1 {

		builder.WriteString(tag)
		return
	}

	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' ||
			tag[i] == '=' ||
			tag[i] == ' ' {
			builder.WriteByte('\\')
		}
		builder.WriteByte(tag[i])
	}
}
