package errors

import (
	"bytes"
	"fmt"
)

/*
Err is the implementation of error that all goadb functions return.

Best Practice

External errors should be wrapped using WrapErrorf, as soon as they are known about.

Intermediate code should pass *Errs up until they will be returned outside the library.
Errors should *not* be wrapped at every return site.

Just before returning an *Err outside the library, it can be wrapped again, preserving the
ErrCode (e.g. with WrapErrf).
*/
type Err struct {
	// Code is the high-level "type" of error.
	Code ErrCode
	// Message is a human-readable description of the error.
	Message string
	// Details is optional, and can be used to associate any auxiliary data with an error.
	Details interface{}
	// Cause is optional, and points to the more specific error that caused this one.
	Cause error
}

var _ error = &Err{}

// Keep this in sync with ../error.go.
//go:generate stringer -type=ErrCode
type ErrCode byte

const (
	AssertionError ErrCode = iota
	ParseError
	// The server was not available on the requested port.
	ServerNotAvailable
	// General network error communicating with the server.
	NetworkError
	// The connection to the server was reset in the middle of an operation. Server probably died.
	ConnectionResetError
	// The server returned an error message, but we couldn't parse it.
	AdbError
	// The server returned a "device not found" error.
	DeviceNotFound
	// Tried to perform an operation on a path that doesn't exist on the device.
	FileNoExistError
)

func Errorf(code ErrCode, format string, args ...interface{}) error {
	return &Err{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

/*
WrapErrf returns an *Err that wraps another *Err and has the same ErrCode.
Panics if cause is not an *Err.

To wrap generic errors, use WrapErrorf.
*/
func WrapErrf(cause error, format string, args ...interface{}) error {
	if cause == nil {
		return nil
	}

	err := cause.(*Err)
	return &Err{
		Code:    err.Code,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
	}
}

// CombineErrs returns an error that wraps all the non-nil errors passed to it.
// If all errors are nil, returns nil.
// If there's only one non-nil error, returns that error without wrapping.
// Else, returns an error with the message and code as passed, with the cause set to an error
// that contains all the non-nil errors and for which Error() returns the concatenation of all their messages.
func CombineErrs(msg string, code ErrCode, errs ...error) error {
	var nonNilErrs []error
	for _, err := range errs {
		if err != nil {
			nonNilErrs = append(nonNilErrs, err)
		}
	}

	switch len(nonNilErrs) {
	case 0:
		return nil
	case 1:
		return nonNilErrs[0]
	default:
		return WrapErrorf(multiError(nonNilErrs), code, "%s", msg)
	}
}

type multiError []error

func (errs multiError) Error() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%d errors: [", len(errs))
	for i, err := range errs {
		buf.WriteString(err.Error())
		if i < len(errs)-1 {
			buf.WriteString(" ∪ ")
		}
	}
	buf.WriteRune(']')
	return buf.String()
}

/*
WrapErrorf returns an *Err that wraps another arbitrary error with an ErrCode and a message.

If cause is nil, returns nil, so you can use it like
	return util.WrapErrorf(DoSomethingDangerous(), util.NetworkError, "well that didn't work")

If cause is known to be of type *Err, use WrapErrf.
*/
func WrapErrorf(cause error, code ErrCode, format string, args ...interface{}) error {
	if cause == nil {
		return nil
	}

	return &Err{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

func AssertionErrorf(format string, args ...interface{}) error {
	return &Err{
		Code:    AssertionError,
		Message: fmt.Sprintf(format, args...),
	}
}

func (err *Err) Error() string {
	msg := fmt.Sprintf("%s: %s", err.Code, err.Message)
	if err.Details != nil {
		msg = fmt.Sprintf("%s (%+v)", msg, err.Details)
	}
	return msg
}

// HasErrCode returns true if err is an *Err and err.Code == code.
func HasErrCode(err error, code ErrCode) bool {
	switch err := err.(type) {
	case *Err:
		return err.Code == code
	default:
		return false
	}
}

/*
ErrorWithCauseChain formats err and all its causes if it's an *Err, else returns
err.Error().
*/
func ErrorWithCauseChain(err error) string {
	var buffer bytes.Buffer

	for {
		if wrappedErr, ok := err.(*Err); ok && wrappedErr.Cause != nil {
			fmt.Fprintln(&buffer, wrappedErr.Error())
			fmt.Fprint(&buffer, "caused by ")
			err = wrappedErr.Cause
		} else {
			break
		}
	}

	if err != nil {
		buffer.WriteString(err.Error())
	} else {
		buffer.WriteString("<err=nil>")
	}

	return buffer.String()
}
