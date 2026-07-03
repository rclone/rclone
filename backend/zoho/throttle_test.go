package zoho

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestFs returns a bare *Fs so the 429/retry logic in shouldRetry can be
// exercised in isolation. No network or pacer is involved.
func newTestFs() *Fs {
	return &Fs{}
}

func TestShouldRetry(t *testing.T) {
	ctx := context.Background()

	// A 429 with a numeric Retry-After is honoured, plus retryAfterMargin.
	t.Run("429 honours Retry-After", func(t *testing.T) {
		f := newTestFs()
		resp := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"5"}}}
		retry, err := f.shouldRetry(ctx, resp, assert.AnError)
		assert.True(t, retry)
		wait, ok := pacer.IsRetryAfter(err)
		require.True(t, ok)
		assert.Equal(t, 5*time.Second+retryAfterMargin, wait)
	})

	// A 429 without a Retry-After header falls back to 60s + margin.
	t.Run("429 without Retry-After falls back to 60s", func(t *testing.T) {
		f := newTestFs()
		resp := &http.Response{StatusCode: 429, Header: http.Header{}}
		retry, err := f.shouldRetry(ctx, resp, assert.AnError)
		assert.True(t, retry)
		wait, ok := pacer.IsRetryAfter(err)
		require.True(t, ok)
		assert.Equal(t, 60*time.Second+retryAfterMargin, wait)
	})

	// An unparseable Retry-After is ignored in favour of the 60s fallback.
	t.Run("429 with unparseable Retry-After falls back", func(t *testing.T) {
		f := newTestFs()
		resp := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"soon"}}}
		retry, err := f.shouldRetry(ctx, resp, assert.AnError)
		assert.True(t, retry)
		wait, ok := pacer.IsRetryAfter(err)
		require.True(t, ok)
		assert.Equal(t, 60*time.Second+retryAfterMargin, wait)
	})

	// A missing OAuth scope is fatal and must not be retried.
	t.Run("401 missing scope aborts", func(t *testing.T) {
		f := newTestFs()
		resp := &http.Response{StatusCode: 401, Status: "401 INVALID_OAUTHSCOPE"}
		retry, _ := f.shouldRetry(ctx, resp, assert.AnError)
		assert.False(t, retry)
	})

	// An expired OAuth token is retried so the token can refresh.
	t.Run("401 expired token retries", func(t *testing.T) {
		f := newTestFs()
		resp := &http.Response{StatusCode: 401, Header: http.Header{"Www-Authenticate": {`Bearer error="expired_token"`}}}
		retry, _ := f.shouldRetry(ctx, resp, assert.AnError)
		assert.True(t, retry)
	})

	// A cancelled context is never retried.
	t.Run("cancelled context aborts", func(t *testing.T) {
		f := newTestFs()
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		retry, _ := f.shouldRetry(cctx, nil, nil)
		assert.False(t, retry)
	})
}
