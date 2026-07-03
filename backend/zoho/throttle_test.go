package zoho

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestFs returns a bare *Fs carrying just the throttle state shouldRetry
// touches, armed exactly as NewFs arms it. No network or pacer is involved, so
// the 429/retry logic can be exercised in isolation.
func newTestFs() *Fs {
	f := &Fs{throttle: &throttleState{}}
	f.throttle.progress.Store(true)
	return f
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

// TestThrottleEpisode covers the once-per-episode logging state machine that
// logThrottle/shouldRetry drive through throttleState, without sleeping: the
// penalty window is moved by hand instead of waited out.
func TestThrottleEpisode(t *testing.T) {
	ctx := context.Background()
	f := newTestFs() // progress armed: the first 429 would log at NOTICE
	resp429 := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"1"}}}
	respOK := &http.Response{StatusCode: 200}

	// The first 429 consumes the armed flag (logs once) and opens a penalty window.
	_, _ = f.shouldRetry(ctx, resp429, assert.AnError)
	assert.False(t, f.throttle.progress.Load(), "first 429 disarms progress")

	// A success still inside the penalty window must not re-arm (would let a
	// burst of in-flight successes start a fresh episode too early).
	_, _ = f.shouldRetry(ctx, respOK, nil)
	assert.False(t, f.throttle.progress.Load(), "success during penalty window does not re-arm")

	// Once the penalty window has elapsed, a success re-arms for the next episode.
	f.throttle.penaltyUntilNano.Store(time.Now().Add(-time.Second).UnixNano())
	_, _ = f.shouldRetry(ctx, respOK, nil)
	assert.True(t, f.throttle.progress.Load(), "success after penalty window re-arms")
}

// TestThrottleStateConcurrent drives the lock-free throttleState (shared across
// the shallow Fs copy) from many goroutines. Like the registry test, `go test
// -race` is the real assertion: it would flag any non-atomic access. The final
// flag is interleaving-dependent, so it is deliberately not asserted.
func TestThrottleStateConcurrent(t *testing.T) {
	ctx := context.Background()
	f := newTestFs()
	resp429 := &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": {"1"}}}
	respOK := &http.Response{StatusCode: 200}

	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				_, _ = f.shouldRetry(ctx, resp429, assert.AnError)
			} else {
				_, _ = f.shouldRetry(ctx, respOK, nil)
			}
		}(i)
	}
	wg.Wait()
}
