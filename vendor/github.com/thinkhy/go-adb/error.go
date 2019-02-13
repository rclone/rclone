package adb

import "github.com/thinkhy/go-adb/internal/errors"
import sysErrors "errors"

type ErrCode errors.ErrCode

var ErrPackageNotExist = sysErrors.New("package not exist")

const (
	AssertionError = ErrCode(errors.AssertionError)
	ParseError     = ErrCode(errors.ParseError)
	// The server was not available on the requested port.
	ServerNotAvailable = ErrCode(errors.ServerNotAvailable)
	// General network error communicating with the server.
	NetworkError = ErrCode(errors.NetworkError)
	// The connection to the server was reset in the middle of an operation. Server probably died.
	ConnectionResetError = ErrCode(errors.ConnectionResetError)
	// The server returned an error message, but we couldn't parse it.
	AdbError = ErrCode(errors.AdbError)
	// The server returned a "device not found" error.
	DeviceNotFound = ErrCode(errors.DeviceNotFound)
	// Tried to perform an operation on a path that doesn't exist on the device.
	FileNoExistError = ErrCode(errors.FileNoExistError)
)

// HasErrCode returns true if err is an *errors.Err and err.Code == code.
func HasErrCode(err error, code ErrCode) bool {
	return errors.HasErrCode(err, errors.ErrCode(code))
}

/*
ErrorWithCauseChain formats err and all its causes if it's an *errors.Err, else returns
err.Error().
*/
func ErrorWithCauseChain(err error) string {
	return errors.ErrorWithCauseChain(err)
}
