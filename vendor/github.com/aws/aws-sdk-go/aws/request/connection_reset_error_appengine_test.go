// +build appengine

package request_test

import (
	"errors"
)

var stubConnectionResetError = errors.New("connection reset")
