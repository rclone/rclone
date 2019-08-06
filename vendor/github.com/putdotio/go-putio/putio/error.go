package putio

import (
	"fmt"
	"net/http"
)

// ErrorResponse reports the error caused by an API request.
type ErrorResponse struct {
	Response *http.Response `json:"-"`

	Message string `json:"error_message"`
	Type    string `json:"error_type"`
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf(
		"Type: %v Message: %q. Original error: %v %v: %v",
		e.Type,
		e.Message,
		e.Response.Request.Method,
		e.Response.Request.URL,
		e.Response.Status,
	)
}
