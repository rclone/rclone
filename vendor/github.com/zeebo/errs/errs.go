// Package errs provides a simple error package with stack traces.
package errs

import (
	"fmt"
	"io"
	"runtime"
)

// Namer is implemented by all errors returned in this package. It returns a
// name for the class of error it is, and a boolean indicating if the name is
// valid.
type Namer interface{ Name() (string, bool) }

// Causer is implemented by all errors returned in this package. It returns
// the underlying cause of the error, or nil if there is no underlying cause.
type Causer interface{ Cause() error }

// unwrapper is implemented by all errors returned in this package. It returns
// the underlying cause of the error, or nil if there is no underlying error.
type unwrapper interface{ Unwrap() error }

// ungrouper is implemented by combinedError returned in this package. It
// returns all underlying errors, or nil if there is no underlying error.
type ungrouper interface{ Ungroup() []error }

// New returns an error not contained in any class. This is the same as calling
// fmt.Errorf(...) except it captures a stack trace on creation.
func New(format string, args ...interface{}) error {
	return (*Class).create(nil, 3, fmt.Errorf(format, args...))
}

// Wrap returns an error not contained in any class. It just associates a stack
// trace with the error. Wrap returns nil if err is nil.
func Wrap(err error) error {
	return (*Class).create(nil, 3, err)
}

// WrapP stores into the error pointer if it contains a non-nil error an error not
// contained in any class. It just associates a stack trace with the error. WrapP
// does nothing if the pointer or pointed at error is nil.
func WrapP(err *error) {
	if err != nil && *err != nil {
		*err = (*Class).create(nil, 3, *err)
	}
}

// Often, we call Cause as much as possible. Since comparing arbitrary
// interfaces with equality isn't panic safe, we only loop up to 100
// times to ensure that a poor implementation that causes a cycle does
// not run forever.
const maxCause = 100

// Unwrap returns the underlying error, if any, or just the error.
func Unwrap(err error) error {
	for i := 0; err != nil && i < maxCause; i++ {
		var nerr error

		switch e := err.(type) {
		case Causer:
			nerr = e.Cause()

		case unwrapper:
			nerr = e.Unwrap()
		}

		if nerr == nil {
			return err
		}
		err = nerr
	}

	return err
}

// Classes returns all the classes that have wrapped the error.
func Classes(err error) (classes []*Class) {
	causes := 0
	for {
		switch e := err.(type) {
		case *errorT:
			if e.class != nil {
				classes = append(classes, e.class)
			}
			err = e.err
			continue

		case Causer:
			err = e.Cause()

		case unwrapper:
			err = e.Unwrap()

		default:
			return classes
		}

		if causes >= maxCause {
			return classes
		}
		causes++
	}
}

// Is checks if any of the underlying errors matches target
func Is(err, target error) bool {
	return IsFunc(err, func(err error) bool {
		return err == target
	})
}

// IsFunc checks if any of the underlying errors matches the func
func IsFunc(err error, is func(err error) bool) bool {
	causes := 0
	errs := []error{err}

	for len(errs) > 0 {
		var next []error
		for _, err := range errs {
			if is(err) {
				return true
			}

			switch e := err.(type) {
			case ungrouper:
				ungrouped := e.Ungroup()
				for _, unerr := range ungrouped {
					if unerr != nil {
						next = append(next, unerr)
					}
				}
			case Causer:
				cause := e.Cause()
				if cause != nil {
					next = append(next, cause)
				}
			case unwrapper:
				unwrapped := e.Unwrap()
				if unwrapped != nil {
					next = append(next, unwrapped)
				}
			}

			if causes >= maxCause {
				return false
			}
			causes++
		}
		errs = next
	}

	return false
}

//
// error classes
//

// Class represents a class of errors. You can construct errors, and check if
// errors are part of the class.
type Class string

// Has returns true if the passed in error was wrapped by this class.
func (c *Class) Has(err error) bool {
	return IsFunc(err, func(err error) bool {
		errt, ok := err.(*errorT)
		return ok && errt.class == c
	})
}

// New constructs an error with the format string that will be contained by
// this class. This is the same as calling Wrap(fmt.Errorf(...)).
func (c *Class) New(format string, args ...interface{}) error {
	return c.create(3, fmt.Errorf(format, args...))
}

// Wrap returns a new error based on the passed in error that is contained in
// this class. Wrap returns nil if err is nil.
func (c *Class) Wrap(err error) error {
	return c.create(3, err)
}

// WrapP stores into the error pointer if it contains a non-nil error an error contained
// in this class. WrapP does nothing if the pointer or pointed at error is nil.
func (c *Class) WrapP(err *error) {
	if err != nil && *err != nil {
		*err = c.create(3, *err)
	}
}

// create constructs the error, or just adds the class to the error, keeping
// track of the stack if it needs to construct it.
func (c *Class) create(depth int, err error) error {
	if err == nil {
		return nil
	}

	var pcs []uintptr
	if err, ok := err.(*errorT); ok {
		if c == nil || err.class == c {
			return err
		}
		pcs = err.pcs
	}

	errt := &errorT{
		class: c,
		err:   err,
		pcs:   pcs,
	}

	if errt.pcs == nil {
		errt.pcs = make([]uintptr, 64)
		n := runtime.Callers(depth, errt.pcs)
		errt.pcs = errt.pcs[:n:n]
	}

	return errt
}

//
// errors
//

// errorT is the type of errors returned from this package.
type errorT struct {
	class *Class
	err   error
	pcs   []uintptr
}

var ( // ensure *errorT implements the helper interfaces.
	_ Namer  = (*errorT)(nil)
	_ Causer = (*errorT)(nil)
	_ error  = (*errorT)(nil)
)

// Stack returns the pcs for the stack trace associated with the error.
func (e *errorT) Stack() []uintptr { return e.pcs }

// errorT implements the error interface.
func (e *errorT) Error() string {
	return fmt.Sprintf("%v", e)
}

// Format handles the formatting of the error. Using a "+" on the format string
// specifier will also write the stack trace.
func (e *errorT) Format(f fmt.State, c rune) {
	sep := ""
	if e.class != nil && *e.class != "" {
		fmt.Fprintf(f, "%s", string(*e.class))
		sep = ": "
	}
	if text := e.err.Error(); len(text) > 0 {
		fmt.Fprintf(f, "%s%v", sep, text)
	}
	if f.Flag(int('+')) {
		summarizeStack(f, e.pcs)
	}
}

// Cause implements the interface wrapping errors are expected to implement
// to allow getting at underlying causes.
func (e *errorT) Cause() error {
	return e.err
}

// Unwrap implements the draft design for error inspection. Since this is
// on an unexported type, it should not be hard to maintain going forward
// given that it also is the exact same semantics as Cause.
func (e *errorT) Unwrap() error {
	return e.err
}

// Name returns the name for the error, which is the first wrapping class.
func (e *errorT) Name() (string, bool) {
	if e.class == nil {
		return "", false
	}
	return string(*e.class), true
}

// summarizeStack writes stack line entries to the writer.
func summarizeStack(w io.Writer, pcs []uintptr) {
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		if !more {
			return
		}
		fmt.Fprintf(w, "\n\t%s:%d", frame.Function, frame.Line)
	}
}
