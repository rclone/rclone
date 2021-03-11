package putio

import (
	"context"
	"fmt"
	"net/http"

	"github.com/putdotio/go-putio/putio"
	"github.com/rclone/rclone/fs/fserrors"
)

func checkStatusCode(resp *http.Response, expected int) error {
	if resp.StatusCode != expected {
		return &statusCodeError{response: resp}
	}
	return nil
}

type statusCodeError struct {
	response *http.Response
}

func (e *statusCodeError) Error() string {
	return fmt.Sprintf("unexpected status code (%d) response while doing %s to %s", e.response.StatusCode, e.response.Request.Method, e.response.Request.URL.String())
}

func (e *statusCodeError) Temporary() bool {
	return e.response.StatusCode == 429 || e.response.StatusCode >= 500
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err == nil {
		return false, nil
	}
	if perr, ok := err.(*putio.ErrorResponse); ok {
		err = &statusCodeError{response: perr.Response}
	}
	if fserrors.ShouldRetry(err) {
		return true, err
	}
	return false, err
}
