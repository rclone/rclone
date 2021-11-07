package errors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	liberrors "github.com/rclone/rclone/lib/errors"
)

func TestWalk(t *testing.T) {
	origin := errors.New("origin")

	for _, test := range []struct {
		err   error
		calls int
		last  error
	}{
		{causerError{nil}, 1, causerError{nil}},
		{wrapperError{nil}, 1, wrapperError{nil}},
		{reflectError{nil}, 1, reflectError{nil}},
		{causerError{origin}, 2, origin},
		{wrapperError{origin}, 2, origin},
		{reflectError{origin}, 2, origin},
		{causerError{reflectError{origin}}, 3, origin},
		{wrapperError{causerError{origin}}, 3, origin},
		{reflectError{wrapperError{origin}}, 3, origin},
		{causerError{reflectError{causerError{origin}}}, 4, origin},
		{wrapperError{causerError{wrapperError{origin}}}, 4, origin},
		{reflectError{wrapperError{reflectError{origin}}}, 4, origin},

		{stopError{nil}, 1, stopError{nil}},
		{stopError{causerError{nil}}, 1, stopError{causerError{nil}}},
		{stopError{wrapperError{nil}}, 1, stopError{wrapperError{nil}}},
		{stopError{reflectError{nil}}, 1, stopError{reflectError{nil}}},
		{causerError{stopError{origin}}, 2, stopError{origin}},
		{wrapperError{stopError{origin}}, 2, stopError{origin}},
		{reflectError{stopError{origin}}, 2, stopError{origin}},
		{causerError{reflectError{stopError{nil}}}, 3, stopError{nil}},
		{wrapperError{causerError{stopError{nil}}}, 3, stopError{nil}},
		{reflectError{wrapperError{stopError{nil}}}, 3, stopError{nil}},
	} {
		var last error
		calls := 0
		liberrors.Walk(test.err, func(err error) bool {
			calls++
			last = err
			_, stop := err.(stopError)
			return stop
		})
		assert.Equal(t, test.calls, calls)
		assert.Equal(t, test.last, last)
	}
}

type causerError struct {
	err error
}
type wrapperError struct {
	err error
}
type reflectError struct {
	Err error
}
type stopError struct {
	err error
}

func (e causerError) Error() string {
	return fmt.Sprintf("causerError(%s)", e.err)
}
func (e causerError) Cause() error {
	return e.err
}
func (e wrapperError) Unwrap() error {
	return e.err
}
func (e wrapperError) Error() string {
	return fmt.Sprintf("wrapperError(%s)", e.err)
}
func (e reflectError) Error() string {
	return fmt.Sprintf("reflectError(%s)", e.Err)
}
func (e stopError) Error() string {
	return fmt.Sprintf("stopError(%s)", e.err)
}
func (e stopError) Cause() error {
	return e.err
}
