package wire

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/thinkhy/go-adb/internal/errors"
)

// ErrorResponseDetails is an error message returned by the server for a particular request.
type ErrorResponseDetails struct {
	Request   string
	ServerMsg string
}

// deviceNotFoundMessagePattern matches all possible error messages returned by adb servers to
// report that a matching device was not found. Used to set the DeviceNotFound error code on
// error values.
//
// Old servers send "device not found", and newer ones "device 'serial' not found".
var deviceNotFoundMessagePattern = regexp.MustCompile(`device( '.*')? not found`)

func adbServerError(request string, serverMsg string) error {
	var msg string
	if request == "" {
		msg = fmt.Sprintf("server error: %s", serverMsg)
	} else {
		msg = fmt.Sprintf("server error for %s request: %s", request, serverMsg)
	}

	errCode := errors.AdbError
	if deviceNotFoundMessagePattern.MatchString(serverMsg) {
		errCode = errors.DeviceNotFound
	}

	return &errors.Err{
		Code:    errCode,
		Message: msg,
		Details: ErrorResponseDetails{
			Request:   request,
			ServerMsg: serverMsg,
		},
	}
}

// IsAdbServerErrorMatching returns true if err is an *Err with code AdbError and for which
// predicate returns true when passed Details.ServerMsg.
func IsAdbServerErrorMatching(err error, predicate func(string) bool) bool {
	if err, ok := err.(*errors.Err); ok && err.Code == errors.AdbError {
		return predicate(err.Details.(ErrorResponseDetails).ServerMsg)
	}
	return false
}

func errIncompleteMessage(description string, actual int, expected int) error {
	return &errors.Err{
		Code:    errors.ConnectionResetError,
		Message: fmt.Sprintf("incomplete %s: read %d bytes, expecting %d", description, actual, expected),
		Details: struct {
			ActualReadBytes int
			ExpectedBytes   int
		}{
			ActualReadBytes: actual,
			ExpectedBytes:   expected,
		},
	}
}

// writeFully writes all of data to w.
// Inverse of io.ReadFully().
func writeFully(w io.Writer, data []byte) error {
	offset := 0
	for offset < len(data) {
		n, err := w.Write(data[offset:])
		if err != nil {
			return errors.WrapErrorf(err, errors.NetworkError, "error writing %d bytes at offset %d", len(data), offset)
		}
		offset += n
	}
	return nil
}

// MultiCloseable wraps c in a ReadWriteCloser that can be safely closed multiple times.
func MultiCloseable(c io.ReadWriteCloser) io.ReadWriteCloser {
	return &multiCloseable{ReadWriteCloser: c}
}

type multiCloseable struct {
	io.ReadWriteCloser
	closeOnce sync.Once
	err       error
}

func (c *multiCloseable) Close() error {
	c.closeOnce.Do(func() {
		c.err = c.ReadWriteCloser.Close()
	})
	return c.err
}
