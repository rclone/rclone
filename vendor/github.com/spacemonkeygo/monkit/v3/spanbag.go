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

// spanBag is a bag data structure (can add 0 or more references to a span,
// where every add needs to be matched with an equivalent remove). spanBag has
// a fast path for dealing with cases where the bag only has one element (the
// common case). spanBag is not threadsafe
type spanBag struct {
	first *Span
	rest  map[*Span]int32
}

func (b *spanBag) Add(s *Span) {
	if b.first == nil {
		b.first = s
		return
	}
	if b.rest == nil {
		b.rest = map[*Span]int32{}
	}
	b.rest[s] += 1
}

func (b *spanBag) Remove(s *Span) {
	if b.first == s {
		b.first = nil
		return
	}
	// okay it must be in b.rest
	count := b.rest[s]
	if count <= 1 {
		delete(b.rest, s)
	} else {
		b.rest[s] = count - 1
	}
}

// Iterate returns all elements
func (b *spanBag) Iterate(cb func(*Span)) {
	if b.first != nil {
		cb(b.first)
	}
	for s := range b.rest {
		cb(s)
	}
}
