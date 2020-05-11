// Copyright (C) 2014 Space Monkey, Inc.
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

package errors

import (
	"fmt"
	"log"
	"runtime"
	"strings"
)

var (
	// Change this method if you want errors to log somehow else
	LogMethod = log.Printf

	ErrorGroupError = NewClass("Error Group Error")
)

// LogWithStack will log the given messages with the current stack
func LogWithStack(messages ...interface{}) {
	buf := make([]byte, Config.Stacklogsize)
	buf = buf[:runtime.Stack(buf, false)]
	LogMethod("%s\n%s", fmt.Sprintln(messages...), buf)
}

// CatchPanic can be used to catch panics and turn them into errors. See the
// example.
func CatchPanic(err_ref *error) {
	r := recover()
	if r == nil {
		return
	}
	err, ok := r.(error)
	if ok {
		*err_ref = PanicError.Wrap(err)
		return
	}
	*err_ref = PanicError.New("%v", r)
}

// ErrorGroup is a type for collecting errors from a bunch of independent
// tasks. ErrorGroups are not threadsafe. See the example for usage.
type ErrorGroup struct {
	Errors []error
	limit  int
	excess int
}

// NewErrorGroup makes a new ErrorGroup
func NewErrorGroup() *ErrorGroup { return &ErrorGroup{} }

// NewBoundedErrorGroup makes a new ErrorGroup that will not track more than
// limit errors. Once the limit is reached, the ErrorGroup will track
// additional errors as excess.
func NewBoundedErrorGroup(limit int) *ErrorGroup {
	return &ErrorGroup{
		limit: limit,
	}
}

// Add is called with errors. nil errors are ignored.
func (e *ErrorGroup) Add(err error) {
	if err == nil {
		return
	}
	if e.limit > 0 && len(e.Errors) == e.limit {
		e.excess++
	} else {
		e.Errors = append(e.Errors, err)
	}
}

// Finalize will collate all the found errors. If no errors were found, it will
// return nil. If one error was found, it will be returned directly. Otherwise
// an ErrorGroupError will be returned.
func (e *ErrorGroup) Finalize() error {
	if len(e.Errors) == 0 {
		return nil
	}
	if len(e.Errors) == 1 && e.excess == 0 {
		return e.Errors[0]
	}
	msgs := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	if e.excess > 0 {
		msgs = append(msgs, fmt.Sprintf("... and %d more.", e.excess))
		e.excess = 0
	}
	e.Errors = nil
	return ErrorGroupError.New(strings.Join(msgs, "\n"))
}

// LoggingErrorGroup is similar to ErrorGroup except that instead of collecting
// all of the errors, it logs the errors immediately and just counts how many
// non-nil errors have been seen. See the ErrorGroup example for usage.
type LoggingErrorGroup struct {
	name   string
	total  int
	failed int
}

// NewLoggingErrorGroup returns a new LoggingErrorGroup with the given name.
func NewLoggingErrorGroup(name string) *LoggingErrorGroup {
	return &LoggingErrorGroup{name: name}
}

// Add will handle a given error. If the error is non-nil, total and failed
// are both incremented and the error is logged. If the error is nil, only
// total is incremented.
func (e *LoggingErrorGroup) Add(err error) {
	e.total++
	if err != nil {
		LogMethod("%s: %s", e.name, err)
		e.failed++
	}
}

// Finalize returns no error if no failures were observed, otherwise it will
// return an ErrorGroupError with statistics about the observed errors.
func (e *LoggingErrorGroup) Finalize() (err error) {
	if e.failed > 0 {
		err = ErrorGroupError.New("%s: %d of %d failed.", e.name, e.failed,
			e.total)
	}
	e.total = 0
	e.failed = 0
	return err
}

type Finalizer interface {
	Finalize() error
}

// Finalize takes a group of ErrorGroups and joins them together into one error
func Finalize(finalizers ...Finalizer) error {
	var errs ErrorGroup
	for _, finalizer := range finalizers {
		errs.Add(finalizer.Finalize())
	}
	return errs.Finalize()
}
