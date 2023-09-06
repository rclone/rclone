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

type taskKey int

const taskGetFunc taskKey = 0

type taskSecretT struct{}

func (*taskSecretT) Value(key interface{}) interface{} { return nil }
func (*taskSecretT) Done() <-chan struct{}             { return nil }
func (*taskSecretT) Err() error                        { return nil }
func (*taskSecretT) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Func returns the Func associated with the Task
func (f Task) Func() (out *Func) {
	// we're doing crazy things to make a function have methods that do other
	// things with internal state. basically, we have a secret argument we can
	// pass to the function that is only checked if ctx is taskSecret (
	// which it should never be) that controls what other behavior we want.
	// in this case, if arg[0] is taskGetFunc, then f will place the func in the
	// out location.
	// since someone can cast any function of this signature to a lazy task,
	// let's make sure we got roughly expected behavior and panic otherwise
	if f(&taskSecret, taskGetFunc, &out) != nil || out == nil {
		panic("Func() called on a non-Task function")
	}
	return out
}

func taskArgs(f *Func, args []interface{}) bool {
	// this function essentially does method dispatch for Tasks. returns true
	// if a method got dispatched and normal behavior should be aborted
	if len(args) != 2 {
		return false
	}
	val, ok := args[0].(taskKey)
	if !ok {
		return false
	}
	switch val {
	case taskGetFunc:
		*(args[1].(**Func)) = f
		return true
	}
	return false
}

// TaskNamed is like Task except you can choose the name of the associated
// Func.
//
// You may also include any SeriesTags which should be included with the Task.
func (s *Scope) TaskNamed(name string, tags ...SeriesTag) Task {
	return s.FuncNamed(name, tags...).Task
}
