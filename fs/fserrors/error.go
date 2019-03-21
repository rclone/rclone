// Package fserrors provides errors and error handling
package fserrors

import (
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Retrier is an optional interface for error as to whether the
// operation should be retried at a high level.
//
// This should be returned from Update or Put methods as required
type Retrier interface {
	error
	Retry() bool
}

// retryError is a type of error
type retryError string

// Error interface
func (r retryError) Error() string {
	return string(r)
}

// Retry interface
func (r retryError) Retry() bool {
	return true
}

// Check interface
var _ Retrier = retryError("")

// RetryErrorf makes an error which indicates it would like to be retried
func RetryErrorf(format string, a ...interface{}) error {
	return retryError(fmt.Sprintf(format, a...))
}

// wrappedRetryError is an error wrapped so it will satisfy the
// Retrier interface and return true
type wrappedRetryError struct {
	error
}

// Retry interface
func (err wrappedRetryError) Retry() bool {
	return true
}

// Check interface
var _ Retrier = wrappedRetryError{error(nil)}

// RetryError makes an error which indicates it would like to be retried
func RetryError(err error) error {
	if err == nil {
		err = errors.New("needs retry")
	}
	return wrappedRetryError{err}
}

// IsRetryError returns true if err conforms to the Retry interface
// and calling the Retry method returns true.
func IsRetryError(err error) bool {
	if err == nil {
		return false
	}
	_, err = Cause(err)
	if r, ok := err.(Retrier); ok {
		return r.Retry()
	}
	return false
}

// Fataler is an optional interface for error as to whether the
// operation should cause the entire operation to finish immediately.
//
// This should be returned from Update or Put methods as required
type Fataler interface {
	error
	Fatal() bool
}

// wrappedFatalError is an error wrapped so it will satisfy the
// Retrier interface and return true
type wrappedFatalError struct {
	error
}

// Fatal interface
func (err wrappedFatalError) Fatal() bool {
	return true
}

// Check interface
var _ Fataler = wrappedFatalError{error(nil)}

// FatalError makes an error which indicates it is a fatal error and
// the sync should stop.
func FatalError(err error) error {
	if err == nil {
		err = errors.New("fatal error")
	}
	return wrappedFatalError{err}
}

// IsFatalError returns true if err conforms to the Fatal interface
// and calling the Fatal method returns true.
func IsFatalError(err error) bool {
	if err == nil {
		return false
	}
	_, err = Cause(err)
	if r, ok := err.(Fataler); ok {
		return r.Fatal()
	}
	return false
}

// NoRetrier is an optional interface for error as to whether the
// operation should not be retried at a high level.
//
// If only NoRetry errors are returned in a sync then the sync won't
// be retried.
//
// This should be returned from Update or Put methods as required
type NoRetrier interface {
	error
	NoRetry() bool
}

// wrappedNoRetryError is an error wrapped so it will satisfy the
// Retrier interface and return true
type wrappedNoRetryError struct {
	error
}

// NoRetry interface
func (err wrappedNoRetryError) NoRetry() bool {
	return true
}

// Check interface
var _ NoRetrier = wrappedNoRetryError{error(nil)}

// NoRetryError makes an error which indicates the sync shouldn't be
// retried.
func NoRetryError(err error) error {
	return wrappedNoRetryError{err}
}

// IsNoRetryError returns true if err conforms to the NoRetry
// interface and calling the NoRetry method returns true.
func IsNoRetryError(err error) bool {
	if err == nil {
		return false
	}
	_, err = Cause(err)
	if r, ok := err.(NoRetrier); ok {
		return r.NoRetry()
	}
	return false
}

// RetryAfter is an optional interface for error as to whether the
// operation should be retried after a given delay
//
// This should be returned from Update or Put methods as required and
// will cause the entire sync to be retried after a delay.
type RetryAfter interface {
	error
	RetryAfter() time.Time
}

// ErrorRetryAfter is an error which expresses a time that should be
// waited for until trying again
type ErrorRetryAfter time.Time

// NewErrorRetryAfter returns an ErrorRetryAfter with the given
// duration as an endpoint
func NewErrorRetryAfter(d time.Duration) ErrorRetryAfter {
	return ErrorRetryAfter(time.Now().Add(d))
}

// Error returns the textual version of the error
func (e ErrorRetryAfter) Error() string {
	return fmt.Sprintf("try again after %v (%v)", time.Time(e).Format(time.RFC3339Nano), time.Time(e).Sub(time.Now()))
}

// RetryAfter returns the time the operation should be retried at or
// after
func (e ErrorRetryAfter) RetryAfter() time.Time {
	return time.Time(e)
}

// Check interface
var _ RetryAfter = ErrorRetryAfter{}

// RetryAfterErrorTime returns the time that the RetryAfter error
// indicates or a Zero time.Time
func RetryAfterErrorTime(err error) time.Time {
	if err == nil {
		return time.Time{}
	}
	_, err = Cause(err)
	if do, ok := err.(RetryAfter); ok {
		return do.RetryAfter()
	}
	return time.Time{}
}

// IsRetryAfterError returns true if err is an ErrorRetryAfter
func IsRetryAfterError(err error) bool {
	return !RetryAfterErrorTime(err).IsZero()
}

// Cause is a souped up errors.Cause which can unwrap some standard
// library errors too.  It returns true if any of the intermediate
// errors had a Timeout() or Temporary() method which returned true.
func Cause(cause error) (retriable bool, err error) {
	err = cause
	for prev := err; err != nil; prev = err {
		// Check for net error Timeout()
		if x, ok := err.(interface {
			Timeout() bool
		}); ok && x.Timeout() {
			retriable = true
		}

		// Check for net error Temporary()
		if x, ok := err.(interface {
			Temporary() bool
		}); ok && x.Temporary() {
			retriable = true
		}

		// Unwrap 1 level if possible
		err = errors.Cause(err)
		if err == nil {
			// errors.Cause can return nil which isn't
			// desirable so pick the previous error in
			// this case.
			err = prev
		}
		if reflect.DeepEqual(err, prev) {
			// Unpack any struct or *struct with a field
			// of name Err which satisfies the error
			// interface.  This includes *url.Error,
			// *net.OpError, *os.SyscallError and many
			// others in the stdlib
			errType := reflect.TypeOf(err)
			errValue := reflect.ValueOf(err)
			if errValue.IsValid() && errType.Kind() == reflect.Ptr {
				errType = errType.Elem()
				errValue = errValue.Elem()
			}
			if errValue.IsValid() && errType.Kind() == reflect.Struct {
				if errField := errValue.FieldByName("Err"); errField.IsValid() {
					errFieldValue := errField.Interface()
					if newErr, ok := errFieldValue.(error); ok {
						err = newErr
					}
				}
			}
		}
		if reflect.DeepEqual(err, prev) {
			break
		}
	}
	return retriable, err
}

// retriableErrorStrings is a list of phrases which when we find it
// in an an error, we know it is a networking error which should be
// retried.
//
// This is incredibly ugly - if only errors.Cause worked for all
// errors and all errors were exported from the stdlib.
var retriableErrorStrings = []string{
	"use of closed network connection", // internal/poll/fd.go
	"unexpected EOF reading trailer",   // net/http/transfer.go
	"transport connection broken",      // net/http/transport.go
	"http: ContentLength=",             // net/http/transfer.go
	"server closed idle connection",    // net/http/transport.go
}

// Errors which indicate networking errors which should be retried
//
// These are added to in retriable_errors*.go
var retriableErrors = []error{
	io.EOF,
	io.ErrUnexpectedEOF,
}

// ShouldRetry looks at an error and tries to work out if retrying the
// operation that caused it would be a good idea. It returns true if
// the error implements Timeout() or Temporary() or if the error
// indicates a premature closing of the connection.
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Find root cause if available
	retriable, err := Cause(err)
	if retriable {
		return true
	}

	// Check if it is a retriable error
	for _, retriableErr := range retriableErrors {
		if err == retriableErr {
			return true
		}
	}

	// Check error strings (yuch!) too
	errString := err.Error()
	for _, phrase := range retriableErrorStrings {
		if strings.Contains(errString, phrase) {
			return true
		}
	}

	return false
}

// ShouldRetryHTTP returns a boolean as to whether this resp deserves.
// It checks to see if the HTTP response code is in the slice
// retryErrorCodes.
func ShouldRetryHTTP(resp *http.Response, retryErrorCodes []int) bool {
	if resp == nil {
		return false
	}
	for _, e := range retryErrorCodes {
		if resp.StatusCode == e {
			return true
		}
	}
	return false
}
