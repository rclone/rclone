package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalk(t *testing.T) {
	var (
		e1 = errors.New("e1")
		e2 = errors.New("e2")
		e3 = errors.New("e3")
	)

	for _, test := range []struct {
		err  error
		want []error
	}{
		{
			causerError{nil}, []error{
				causerError{nil},
			},
		}, {
			wrapperError{nil}, []error{
				wrapperError{nil},
			},
		}, {
			reflectError{nil}, []error{
				reflectError{nil},
			},
		}, {
			causerError{e1}, []error{
				causerError{e1}, e1,
			},
		}, {
			wrapperError{e1}, []error{
				wrapperError{e1}, e1,
			},
		}, {
			reflectError{e1}, []error{
				reflectError{e1}, e1,
			},
		}, {
			causerError{reflectError{e1}}, []error{
				causerError{reflectError{e1}},
				reflectError{e1},
				e1,
			},
		}, {
			wrapperError{causerError{e1}}, []error{
				wrapperError{causerError{e1}},
				causerError{e1},
				e1,
			},
		}, {
			reflectError{wrapperError{e1}}, []error{
				reflectError{wrapperError{e1}},
				wrapperError{e1},
				e1,
			},
		}, {
			causerError{reflectError{causerError{e1}}}, []error{
				causerError{reflectError{causerError{e1}}},
				reflectError{causerError{e1}},
				causerError{e1},
				e1,
			},
		}, {
			wrapperError{causerError{wrapperError{e1}}}, []error{
				wrapperError{causerError{wrapperError{e1}}},
				causerError{wrapperError{e1}},
				wrapperError{e1},
				e1,
			},
		}, {
			reflectError{wrapperError{reflectError{e1}}}, []error{
				reflectError{wrapperError{reflectError{e1}}},
				wrapperError{reflectError{e1}},
				reflectError{e1},
				e1,
			},
		}, {
			stopError{nil}, []error{
				stopError{nil},
			},
		}, {
			stopError{causerError{nil}}, []error{
				stopError{causerError{nil}},
			},
		}, {
			stopError{wrapperError{nil}}, []error{
				stopError{wrapperError{nil}},
			},
		}, {
			stopError{reflectError{nil}}, []error{
				stopError{reflectError{nil}},
			},
		}, {
			causerError{stopError{e1}}, []error{
				causerError{stopError{e1}},
				stopError{e1},
			},
		}, {
			wrapperError{stopError{e1}}, []error{
				wrapperError{stopError{e1}},
				stopError{e1},
			},
		}, {
			reflectError{stopError{e1}}, []error{
				reflectError{stopError{e1}},
				stopError{e1},
			},
		}, {
			causerError{reflectError{stopError{nil}}}, []error{
				causerError{reflectError{stopError{nil}}},
				reflectError{stopError{nil}},
				stopError{nil},
			},
		}, {
			wrapperError{causerError{stopError{nil}}}, []error{
				wrapperError{causerError{stopError{nil}}},
				causerError{stopError{nil}},
				stopError{nil},
			},
		}, {
			reflectError{wrapperError{stopError{nil}}}, []error{
				reflectError{wrapperError{stopError{nil}}},
				wrapperError{stopError{nil}},
				stopError{nil},
			},
		}, {
			multiWrapperError{[]error{e1}}, []error{
				multiWrapperError{[]error{e1}},
				e1,
			},
		}, {
			multiWrapperError{[]error{}}, []error{
				multiWrapperError{[]error{}},
			},
		}, {
			multiWrapperError{[]error{e1, e2, e3}}, []error{
				multiWrapperError{[]error{e1, e2, e3}},
				e1,
				e2,
				e3,
			},
		}, {
			multiWrapperError{[]error{reflectError{e1}, wrapperError{e2}, stopError{e3}}}, []error{
				multiWrapperError{[]error{reflectError{e1}, wrapperError{e2}, stopError{e3}}},
				reflectError{e1},
				e1,
				wrapperError{e2},
				e2,
				stopError{e3},
			},
		},
	} {
		var got []error
		Walk(test.err, func(err error) bool {
			got = append(got, err)
			_, stop := err.(stopError)
			return stop
		})
		assert.Equal(t, test.want, got, test.err)
	}
}

type causerError struct {
	err error
}

func (e causerError) Error() string {
	return fmt.Sprintf("causerError(%s)", e.err)
}

func (e causerError) Cause() error {
	return e.err
}

var (
	_ error  = causerError{nil}
	_ causer = causerError{nil}
)

type wrapperError struct {
	err error
}

func (e wrapperError) Unwrap() error {
	return e.err
}

func (e wrapperError) Error() string {
	return fmt.Sprintf("wrapperError(%s)", e.err)
}

var (
	_ error   = wrapperError{nil}
	_ wrapper = wrapperError{nil}
)

type multiWrapperError struct {
	errs []error
}

func (e multiWrapperError) Unwrap() []error {
	return e.errs
}

func (e multiWrapperError) Error() string {
	return fmt.Sprintf("multiWrapperError(%s)", e.errs)
}

var (
	_ error        = multiWrapperError{nil}
	_ multiWrapper = multiWrapperError{nil}
)

type reflectError struct {
	Err error
}

func (e reflectError) Error() string {
	return fmt.Sprintf("reflectError(%s)", e.Err)
}

var (
	_ error = reflectError{nil}
)

type stopError struct {
	err error
}

func (e stopError) Error() string {
	return fmt.Sprintf("stopError(%s)", e.err)
}

func (e stopError) Cause() error {
	return e.err
}

var (
	_ error  = stopError{nil}
	_ causer = stopError{nil}
)
