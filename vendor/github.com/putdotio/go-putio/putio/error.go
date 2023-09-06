package putio

import (
	"fmt"
	"net/http"
)

// ErrorResponse reports the error caused by an API request.
type ErrorResponse struct {
	// Original http.Response
	Response *http.Response `json:"-"`

	// Body read from Response
	Body []byte `json:"-"`

	// Error while parsing the response
	ParseError error

	// These fileds are parsed from response if JSON.
	Message string `json:"error_message"`
	Type    string `json:"error_type"`
}

func (e *ErrorResponse) Error() string {
	if e.ParseError != nil {
		return fmt.Errorf("cannot parse response. code:%d error:%q body:%q",
			e.Response.StatusCode, e.ParseError.Error(), string(e.Body[:250])).Error()
	}
	return fmt.Sprintf(
		"putio error. code:%d type:%q message:%q request:%v %v",
		e.Response.StatusCode,
		e.Type,
		e.Message,
		e.Response.Request.Method,
		e.Response.Request.URL,
	)
}
