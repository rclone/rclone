// Errors and error handling

package fs

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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
var _ Retrier = wrappedRetryError{(error)(nil)}

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
	err = errors.Cause(err)
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
var _ Fataler = wrappedFatalError{(error)(nil)}

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
	err = errors.Cause(err)
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
var _ NoRetrier = wrappedNoRetryError{(error)(nil)}

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
	err = errors.Cause(err)
	if r, ok := err.(NoRetrier); ok {
		return r.NoRetry()
	}
	return false
}

// closedConnErrorStrings is a list of phrases which when we find it
// in an an error, we know it is a networking error which should be
// retried.
//
// This is incredibly ugly - if only errors.Cause worked for all
// errors and all errors were exported from the stdlib.
var closedConnErrorStrings = []string{
	"use of closed network connection", // not exported :-(
}

// isClosedConnError reports whether err is an error from use of a closed
// network connection or prematurely closed connection
//
// Code adapted from net/http
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	errString := err.Error()

	for _, phrase := range closedConnErrorStrings {
		if strings.Contains(errString, phrase) {
			return true
		}
	}

	return isClosedConnErrorPlatform(err)
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
	err = errors.Cause(err)

	// Unwrap url.Error
	if urlErr, ok := err.(*url.Error); ok {
		err = urlErr.Err
	}

	// Look for premature closing of connection
	if err == io.EOF || err == io.ErrUnexpectedEOF || isClosedConnError(err) {
		return true
	}

	// Check for net error Timeout()
	if x, ok := err.(interface {
		Timeout() bool
	}); ok && x.Timeout() {
		return true
	}

	// Check for net error Temporary()
	if x, ok := err.(interface {
		Temporary() bool
	}); ok && x.Temporary() {
		return true
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
