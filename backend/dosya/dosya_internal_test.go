package dosya

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/dosya/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const concurrentLimitBody = `{"ok":false,"error":"You have 5 uploads in progress. Wait for them to finish before starting more.","error_code":"concurrent_upload_limit","max_concurrent_uploads":5}`

func TestErrorHandlerDecodesAPIError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(strings.NewReader(concurrentLimitBody)),
	}
	err := errorHandler(resp)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "concurrent_upload_limit", apiErr.ErrorCode)
	assert.Equal(t, 5, apiErr.MaxConcurrentUploads)
	assert.Contains(t, err.Error(), "uploads in progress")
	assert.Contains(t, err.Error(), "concurrent_upload_limit")
}

func TestErrorHandlerNonJSONBody(t *testing.T) {
	// e.g. an edge (Cloudflare) response that never reached the API
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Status:     "429 Too Many Requests",
		Body:       io.NopCloser(strings.NewReader("error code: 1015\n")),
	}
	err := errorHandler(resp)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
	assert.Equal(t, "", apiErr.ErrorCode)
	assert.Contains(t, err.Error(), "429")
	assert.Contains(t, err.Error(), "error code: 1015")
}

func TestShouldRetryAPIErrors(t *testing.T) {
	ctx := context.Background()

	// The concurrent upload limit is handled by initUpload's slot wait,
	// not by the pacer's low level retries.
	limitErr := &api.Error{StatusCode: http.StatusBadRequest, ErrorCode: "concurrent_upload_limit"}
	retry, err := shouldRetry(ctx, &http.Response{StatusCode: http.StatusBadRequest}, limitErr)
	assert.False(t, retry)
	assert.Equal(t, limitErr, err)

	retry, _ = shouldRetry(ctx, &http.Response{StatusCode: http.StatusTooManyRequests}, &api.Error{StatusCode: http.StatusTooManyRequests})
	assert.True(t, retry)

	retry, _ = shouldRetry(ctx, &http.Response{StatusCode: http.StatusBadRequest}, &api.Error{StatusCode: http.StatusBadRequest, ErrorCode: "something_else"})
	assert.False(t, retry)
}

// swapWaitTimings shortens the slot wait timings for a test and returns a
// function which restores them
func swapWaitTimings(delay, max time.Duration) func() {
	oldDelay, oldMax := concurrentUploadRetryDelay, concurrentUploadWaitMax
	concurrentUploadRetryDelay, concurrentUploadWaitMax = delay, max
	return func() { concurrentUploadRetryDelay, concurrentUploadWaitMax = oldDelay, oldMax }
}

// newTestFs makes an Fs which talks to srv
func newTestFs(srv *httptest.Server) *Fs {
	ctx := context.Background()
	f := &Fs{
		name: "test",
		opt:  Options{WorkspaceID: "ws_test"},
		pacer: fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(time.Millisecond),
			pacer.MaxSleep(time.Millisecond),
			pacer.DecayConstant(decayConstant),
			pacer.AttackConstant(attackConstant),
		)),
	}
	f.rest = rest.NewClient(srv.Client()).SetRoot(srv.URL).SetErrorHandler(errorHandler)
	return f
}

// limitServer answers the first n init calls with the concurrent upload
// limit error and the rest with success
func limitServer(t *testing.T, n int32, calls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/upload/init", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) <= n {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(concurrentLimitBody))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
}

func TestInitUploadWaitsForFreeSlot(t *testing.T) {
	defer swapWaitTimings(5*time.Millisecond, 10*time.Second)()
	var calls atomic.Int32
	srv := limitServer(t, 2, &calls)
	defer srv.Close()
	f := newTestFs(srv)

	resp, err := f.initUpload(context.Background(), "file.txt", 10, "text/plain", "", nil)
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.Equal(t, int32(3), calls.Load())
}

func TestInitUploadGivesUpWhenSlotsStayFull(t *testing.T) {
	defer swapWaitTimings(5*time.Millisecond, 40*time.Millisecond)()
	var calls atomic.Int32
	srv := limitServer(t, 1<<30, &calls)
	defer srv.Close()
	f := newTestFs(srv)

	_, err := f.initUpload(context.Background(), "file.txt", 10, "text/plain", "", nil)
	require.Error(t, err)
	var apiErr *api.Error
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "concurrent_upload_limit", apiErr.ErrorCode)
	assert.Contains(t, err.Error(), "couldn't init upload")
	assert.GreaterOrEqual(t, calls.Load(), int32(2), "should have retried at least once")
}

func TestInitUploadStopsWaitingWhenContextCancelled(t *testing.T) {
	defer swapWaitTimings(time.Minute, time.Hour)()
	var calls atomic.Int32
	srv := limitServer(t, 1<<30, &calls)
	defer srv.Close()
	f := newTestFs(srv)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := f.initUpload(ctx, "file.txt", 10, "text/plain", "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
}
