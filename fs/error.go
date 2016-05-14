// Errors and error handling

package fs

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Retry is an optional interface for error as to whether the
// operation should be retried at a high level.
//
// This should be returned from Update or Put methods as required
type Retry interface {
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
var _ Retry = retryError("")

// RetryErrorf makes an error which indicates it would like to be retried
func RetryErrorf(format string, a ...interface{}) error {
	return retryError(fmt.Sprintf(format, a...))
}

// PlainRetryError is an error wrapped so it will retry
type plainRetryError struct {
	error
}

// Retry interface
func (err plainRetryError) Retry() bool {
	return true
}

// Check interface
var _ Retry = plainRetryError{(error)(nil)}

// RetryError makes an error which indicates it would like to be retried
func RetryError(err error) error {
	return plainRetryError{err}
}

// IsRetryError returns true if err conforms to the Retry interface
// and calling the Retry method returns true.
func IsRetryError(err error) bool {
	if r, ok := err.(Retry); ok {
		return r.Retry()
	}
	return false
}

// isClosedConnError reports whether err is an error from use of a closed
// network connection.
//
// Code adapted from net/http
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	// Note that this error isn't exported so we have to do a
	// string comparison :-(
	str := err.Error()
	if strings.Contains(str, "use of closed network connection") {
		return true
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

	// Look for premature closing of connection
	if err == io.EOF || err == io.ErrUnexpectedEOF || isClosedConnError(err) {
		return true
	}

	// Unwrap url.Error
	if urlErr, ok := err.(*url.Error); ok {
		err = urlErr.Err
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
