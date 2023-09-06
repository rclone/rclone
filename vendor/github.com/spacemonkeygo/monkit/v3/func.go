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
)

// Func represents a FuncStats bound to a particular function id, scope, and
// name. You should create a Func using the Func creation methods
// (Func/FuncNamed) on a Scope. If you want to manage installation bookkeeping
// yourself, create a FuncStats directly. Expected Func creation like:
//
//   var mon = monkit.Package()
//
//   func MyFunc() {
//     f := mon.Func()
//     ...
//   }
//
type Func struct {
	// sync/atomic things
	FuncStats

	// constructor things
	id    int64
	scope *Scope
	key   SeriesKey
}

func newFunc(s *Scope, key SeriesKey) (f *Func) {
	f = &Func{
		id:    NewId(),
		scope: s,
		key:   key,
	}
	initFuncStats(&f.FuncStats, key)
	return f
}

// ShortName returns the name of the function within the package
func (f *Func) ShortName() string { return f.key.Tags.Get("name") }

// FullName returns the name of the function including the package
func (f *Func) FullName() string {
	return fmt.Sprintf("%s.%s", f.scope.name, f.key.Tags.Get("name"))
}

// Id returns a unique integer referencing this function
func (f *Func) Id() int64 { return f.id }

// Scope references the Scope this Func is bound to
func (f *Func) Scope() *Scope { return f.scope }

// Parents will call the given cb with all of the unique Funcs that so far
// have called this Func.
func (f *Func) Parents(cb func(f *Func)) {
	f.FuncStats.parents(cb)
}
