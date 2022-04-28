package putio

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/putdotio/go-putio/putio"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
)

func checkStatusCode(resp *http.Response, expected ...int) error {
	for _, code := range expected {
		if resp.StatusCode == code {
			return nil
		}
	}
	return &statusCodeError{response: resp}
}

type statusCodeError struct {
	response *http.Response
}

func (e *statusCodeError) Error() string {
	return fmt.Sprintf("unexpected status code (%d) response while doing %s to %s", e.response.StatusCode, e.response.Request.Method, e.response.Request.URL.String())
}

// This method is called from fserrors.ShouldRetry() to determine if an error should be retried.
// Some errors (e.g. 429 Too Many Requests) are handled before this step, so they are not included here.
func (e *statusCodeError) Temporary() bool {
	return e.response.StatusCode >= 500
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
	if scerr, ok := err.(*statusCodeError); ok && scerr.response.StatusCode == 429 {
		delay := defaultRateLimitSleep
		header := scerr.response.Header.Get("x-ratelimit-reset")
		if header != "" {
			if resetTime, cerr := strconv.ParseInt(header, 10, 64); cerr == nil {
				delay = time.Until(time.Unix(resetTime+1, 0))
			}
		}
		return true, pacer.RetryAfterError(scerr, delay)
	}
	if fserrors.ShouldRetry(err) {
		return true, err
	}
	return false, err
}
