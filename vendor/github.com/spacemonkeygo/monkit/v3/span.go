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
	"fmt"
	"sort"
	"time"
)

type ctxKey int

const (
	spanKey ctxKey = iota
)

// Annotation represents an arbitrary name and value string pair
type Annotation struct {
	Name  string
	Value string
}

func (s *Span) addChild(child *Span) {
	s.mtx.Lock()
	s.children.Add(child)
	done := s.done
	s.mtx.Unlock()
	if done {
		child.orphan()
	}
}

func (s *Span) removeChild(child *Span) {
	s.mtx.Lock()
	s.children.Remove(child)
	s.mtx.Unlock()
}

func (s *Span) orphan() {
	s.mtx.Lock()
	if !s.done && !s.orphaned {
		s.orphaned = true
		s.f.scope.r.orphanedSpan(s)
	}
	s.mtx.Unlock()
}

// Duration returns the current amount of time the Span has been running
func (s *Span) Duration() time.Duration {
	return time.Since(s.start)
}

// Start returns the time the Span started.
func (s *Span) Start() time.Time {
	return s.start
}

// Value implements context.Context
func (s *Span) Value(key interface{}) interface{} {
	if key == spanKey {
		return s
	}
	return s.Context.Value(key)
}

// String implements context.Context
func (s *Span) String() string {
	// TODO: for working with Contexts
	return fmt.Sprintf("%v.WithSpan()", s.Context)
}

// Children returns all known running child Spans.
func (s *Span) Children(cb func(s *Span)) {
	found := map[*Span]bool{}
	var sorter []*Span
	s.mtx.Lock()
	s.children.Iterate(func(s *Span) {
		if !found[s] {
			found[s] = true
			sorter = append(sorter, s)
		}
	})
	s.mtx.Unlock()
	sort.Sort(spanSorter(sorter))
	for _, s := range sorter {
		cb(s)
	}
}

// Args returns the list of strings associated with the args given to the
// Task that created this Span.
func (s *Span) Args() (rv []string) {
	rv = make([]string, 0, len(s.args))
	for _, arg := range s.args {
		rv = append(rv, fmt.Sprintf("%#v", arg))
	}
	return rv
}

// Id returns the Span id.
func (s *Span) Id() int64 { return s.id }

// Func returns the Func that kicked off this Span.
func (s *Span) Func() *Func { return s.f }

// Trace returns the Trace this Span is associated with.
func (s *Span) Trace() *Trace { return s.trace }

// Parent returns the Parent Span.
func (s *Span) Parent() *Span { return s.parent }

// Annotations returns any added annotations created through the Span Annotate
// method
func (s *Span) Annotations() []Annotation {
	s.mtx.Lock()
	annotations := s.annotations // okay cause we only ever append to this slice
	s.mtx.Unlock()
	return append([]Annotation(nil), annotations...)
}

// Annotate adds an annotation to the existing Span.
func (s *Span) Annotate(name, val string) {
	s.mtx.Lock()
	s.annotations = append(s.annotations, Annotation{Name: name, Value: val})
	s.mtx.Unlock()
}

// Orphaned returns true if the Parent span ended before this Span did.
func (s *Span) Orphaned() (rv bool) {
	s.mtx.Lock()
	rv = s.orphaned
	s.mtx.Unlock()
	return rv
}
