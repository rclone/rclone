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
)

// funcSet is a set data structure (keeps track of unique functions). funcSet
// has a fast path for dealing with cases where the set only has one element.
//
// to reduce memory usage for functions, funcSet exposes its mutex for use in
// other contexts
type funcSet struct {
	// sync/atomic things
	first *Func

	// protected by mtx
	sync.Mutex
	rest map[*Func]struct{}
}

var (
	// used to signify that we've specifically added a nil function, since nil is
	// used internally to specify an empty set.
	nilFunc = &Func{}
)

func (s *funcSet) Add(f *Func) {
	if f == nil {
		f = nilFunc
	}
	if loadFunc(&s.first) == f {
		return
	}
	if compareAndSwapFunc(&s.first, nil, f) {
		return
	}
	s.Mutex.Lock()
	if s.rest == nil {
		s.rest = map[*Func]struct{}{}
	}
	s.rest[f] = struct{}{}
	s.Mutex.Unlock()
}

// Iterate loops over all unique elements of the set.
func (s *funcSet) Iterate(cb func(f *Func)) {
	s.Mutex.Lock()
	uniq := make(map[*Func]struct{}, len(s.rest)+1)
	for f := range s.rest {
		uniq[f] = struct{}{}
	}
	s.Mutex.Unlock()
	f := loadFunc(&s.first)
	if f != nil {
		uniq[f] = struct{}{}
	}
	for f := range uniq {
		if f == nilFunc {
			cb(nil)
		} else {
			cb(f)
		}
	}
}
