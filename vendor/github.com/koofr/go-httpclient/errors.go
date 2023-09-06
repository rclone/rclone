package httpclient

import (
	"errors"
	"fmt"
	"net/http"
)

type InvalidStatusError struct {
	Expected []int
	Got      int
	Headers  http.Header
	Content  string
}

func (e InvalidStatusError) Error() string {
	return fmt.Sprintf("Invalid response status! Got %d, expected %d; headers: %s, content: %s", e.Got, e.Expected, e.Headers, e.Content)
}

func IsInvalidStatusError(err error) (invalidStatusError *InvalidStatusError, ok bool) {
	if ise, ok := err.(InvalidStatusError); ok {
		return &ise, true
	} else if ise, ok := err.(*InvalidStatusError); ok {
		return ise, true
	} else {
		return nil, false
	}
}

func IsInvalidStatusCode(err error, statusCode int) bool {
	if ise, ok := IsInvalidStatusError(err); ok {
		return ise.Got == statusCode
	} else {
		return false
	}
}

var RateLimitTimeoutError = errors.New("HTTPClient rate limit timeout")
