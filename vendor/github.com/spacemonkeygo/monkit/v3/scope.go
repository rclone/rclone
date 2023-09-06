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
	"strings"
	"sync"
)

// Scope represents a named collection of StatSources. Scopes are constructed
// through Registries.
type Scope struct {
	r       *Registry
	name    string
	mtx     sync.RWMutex
	sources map[string]StatSource
	chains  []StatSource
}

func newScope(r *Registry, name string) *Scope {
	return &Scope{
		r:       r,
		name:    name,
		sources: map[string]StatSource{}}
}

// Func retrieves or creates a Func named after the currently executing
// function name (via runtime.Caller. See FuncNamed to choose your own name.
func (s *Scope) Func() *Func {
	return s.FuncNamed(callerFunc(0))
}

func (s *Scope) newSource(name string, constructor func() StatSource) (
	rv StatSource) {

	s.mtx.RLock()
	source, exists := s.sources[name]
	s.mtx.RUnlock()

	if exists {
		return source
	}

	s.mtx.Lock()
	if source, exists := s.sources[name]; exists {
		s.mtx.Unlock()
		return source
	}

	ss := constructor()
	s.sources[name] = ss
	s.mtx.Unlock()

	return ss
}

func sourceName(namespace, name string, tags []SeriesTag) string {
	var sourceNameSize int
	sourceNameSize += len(namespace) + len(name) + len(tags)*2
	for _, tag := range tags {
		sourceNameSize += len(tag.Key) + len(tag.Val)
	}
	var sourceName strings.Builder
	sourceName.Grow(sourceNameSize)
	sourceName.WriteString(namespace)
	sourceName.WriteString(name)
	for _, tag := range tags {
		sourceName.WriteByte(',')
		sourceName.WriteString(tag.Key)
		sourceName.WriteByte('=')
		sourceName.WriteString(tag.Val)
	}
	return sourceName.String()
}

// FuncNamed retrieves or creates a Func named using the given name and
// SeriesTags. See Func() for automatic name determination.
//
// Each unique combination of keys/values in each SeriesTag will result in a
// unique Func. SeriesTags are not sorted, so keep the order consistent to avoid
// unintentionally creating new unique Funcs.
func (s *Scope) FuncNamed(name string, tags ...SeriesTag) *Func {
	source := s.newSource(sourceName("func:", name, tags), func() StatSource {
		return newFunc(s, NewSeriesKey("function").WithTag("name", name).WithTags(tags...))
	})
	f, ok := source.(*Func)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return f
}

// Funcs calls 'cb' for all Funcs registered on this Scope.
func (s *Scope) Funcs(cb func(f *Func)) {
	s.mtx.Lock()
	funcs := make(map[*Func]struct{}, len(s.sources))
	for _, source := range s.sources {
		if f, ok := source.(*Func); ok {
			funcs[f] = struct{}{}
		}
	}
	s.mtx.Unlock()
	for f := range funcs {
		cb(f)
	}
}

// Meter retrieves or creates a Meter named after the given name. See Event.
func (s *Scope) Meter(name string, tags ...SeriesTag) *Meter {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewMeter(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*Meter)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// Event retrieves or creates a Meter named after the given name and then
// calls Mark(1) on that meter.
func (s *Scope) Event(name string, tags ...SeriesTag) {
	s.Meter(name, tags...).Mark(1)
}

// DiffMeter retrieves or creates a DiffMeter after the given name and two
// submeters.
func (s *Scope) DiffMeter(name string, m1, m2 *Meter, tags ...SeriesTag) {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewDiffMeter(NewSeriesKey(name).WithTags(tags...), m1, m2)
	})
	if _, ok := source.(*DiffMeter); !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
}

// IntVal retrieves or creates an IntVal after the given name.
func (s *Scope) IntVal(name string, tags ...SeriesTag) *IntVal {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewIntVal(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*IntVal)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// IntValf retrieves or creates an IntVal after the given printf-formatted
// name.
func (s *Scope) IntValf(template string, args ...interface{}) *IntVal {
	return s.IntVal(fmt.Sprintf(template, args...))
}

// FloatVal retrieves or creates a FloatVal after the given name.
func (s *Scope) FloatVal(name string, tags ...SeriesTag) *FloatVal {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewFloatVal(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*FloatVal)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// FloatValf retrieves or creates a FloatVal after the given printf-formatted
// name.
func (s *Scope) FloatValf(template string, args ...interface{}) *FloatVal {
	return s.FloatVal(fmt.Sprintf(template, args...))
}

// BoolVal retrieves or creates a BoolVal after the given name.
func (s *Scope) BoolVal(name string, tags ...SeriesTag) *BoolVal {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewBoolVal(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*BoolVal)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// BoolValf retrieves or creates a BoolVal after the given printf-formatted
// name.
func (s *Scope) BoolValf(template string, args ...interface{}) *BoolVal {
	return s.BoolVal(fmt.Sprintf(template, args...))
}

// StructVal retrieves or creates a StructVal after the given name.
func (s *Scope) StructVal(name string, tags ...SeriesTag) *StructVal {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewStructVal(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*StructVal)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// DurationVal retrieves or creates a DurationVal after the given name.
func (s *Scope) DurationVal(name string, tags ...SeriesTag) *DurationVal {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewDurationVal(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*DurationVal)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// Timer retrieves or creates a Timer after the given name.
func (s *Scope) Timer(name string, tags ...SeriesTag) *Timer {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewTimer(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*Timer)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// Counter retrieves or creates a Counter after the given name.
func (s *Scope) Counter(name string, tags ...SeriesTag) *Counter {
	source := s.newSource(sourceName("", name, tags), func() StatSource {
		return NewCounter(NewSeriesKey(name).WithTags(tags...))
	})
	m, ok := source.(*Counter)
	if !ok {
		panic(fmt.Sprintf("%s already used for another stats source: %#v",
			name, source))
	}
	return m
}

// Gauge registers a callback that returns a float as the given name in the
// Scope's StatSource table.
func (s *Scope) Gauge(name string, cb func() float64) {
	type gauge struct{ StatSource }

	// gauges allow overwriting
	s.mtx.Lock()
	defer s.mtx.Unlock()

	if source, exists := s.sources[name]; exists {
		if _, ok := source.(gauge); !ok {
			panic(fmt.Sprintf("%s already used for another stats source: %#v",
				name, source))
		}
	}

	s.sources[name] = gauge{StatSource: StatSourceFunc(
		func(scb func(key SeriesKey, field string, value float64)) {
			scb(NewSeriesKey(name), "value", cb())
		}),
	}
}

// Chain registers a full StatSource as the given name in the Scope's
// StatSource table.
func (s *Scope) Chain(source StatSource) {
	// chains allow overwriting
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.chains = append(s.chains, source)
}

func (s *Scope) allNamedSources() (sources []namedSource) {
	s.mtx.Lock()
	sources = make([]namedSource, 0, len(s.sources))
	for name, source := range s.sources {
		sources = append(sources, namedSource{name: name, source: source})
	}
	s.mtx.Unlock()
	return sources
}

// Stats implements the StatSource interface.
func (s *Scope) Stats(cb func(key SeriesKey, field string, val float64)) {
	cbWithScope := func(key SeriesKey, field string, val float64) {
		cb(key.WithTag("scope", s.name), field, val)
	}

	for _, namedSource := range s.allNamedSources() {
		namedSource.source.Stats(cbWithScope)
	}

	s.mtx.Lock()
	chains := append([]StatSource(nil), s.chains...)
	s.mtx.Unlock()

	for _, source := range chains {
		source.Stats(cbWithScope)
	}
}

// Name returns the name of the Scope, often the Package name.
func (s *Scope) Name() string { return s.name }

var _ StatSource = (*Scope)(nil)

type namedSource struct {
	name   string
	source StatSource
}

type namedSourceList []namedSource

func (l namedSourceList) Len() int           { return len(l) }
func (l namedSourceList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l namedSourceList) Less(i, j int) bool { return l[i].name < l[j].name }
