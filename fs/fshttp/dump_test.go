package fshttp

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRetryableResponse(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		err    error
		want   bool
	}{
		{name: "transport error", err: http.ErrHandlerTimeout, want: true},
		{name: "200 OK", status: 200, want: false},
		{name: "301 redirect", status: 301, want: false},
		{name: "404 not found", status: 404, want: false},
		{name: "429 too many requests", status: 429, want: true},
		{name: "500 server error", status: 500, want: true},
		{name: "503 unavailable", status: 503, want: true},
		{name: "509 bandwidth", status: 509, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var resp *http.Response
			if test.err == nil {
				resp = &http.Response{StatusCode: test.status}
			}
			got := isRetryableResponse(resp, test.err)
			assert.Equal(t, test.want, got)
		})
	}

	// nil response with nil error is not retryable
	assert.False(t, isRetryableResponse(nil, nil))
}

// captureBuf is a concurrency safe buffer to capture log output into
type captureBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *captureBuf) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}

func (c *captureBuf) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// TestDumpErrors checks that --dump errors only dumps the transactions
// which fail with a retryable error, and that the other dump flags
// control what is dumped.
func TestDumpErrors(t *testing.T) {
	// Server returns the status code asked for in the path
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
		case "/429":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/404":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write([]byte("response body\n"))
	}))
	defer srv.Close()

	// run makes a request to path with the dump flags set and returns
	// the captured log output.
	run := func(t *testing.T, dump fs.DumpFlags, path string) string {
		capture := &captureBuf{}
		fs.SetLogger(slog.NewTextHandler(capture, &slog.HandlerOptions{Level: slog.LevelDebug}))
		defer fs.SetLogger(slog.NewTextHandler(io.Discard, nil))

		// The dump logging is emitted at Debug level and filtered
		// against the global config, so set that up for the test.
		ci := fs.GetConfig(context.Background())
		oldDump, oldLevel := ci.Dump, ci.LogLevel
		ci.Dump = dump
		ci.LogLevel = fs.LogLevelDebug
		defer func() { ci.Dump = oldDump; ci.LogLevel = oldLevel }()

		client := NewClientCustom(context.Background(), nil)
		req, err := http.NewRequest("GET", srv.URL+path, strings.NewReader("request body\n"))
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return capture.String()
	}

	// Successful and non-retryable error requests should not be dumped
	for _, path := range []string{"/200", "/404"} {
		out := run(t, fs.DumpErrors|fs.DumpBodies, path)
		assert.NotContains(t, out, "HTTP REQUEST", path)
		assert.NotContains(t, out, "HTTP RESPONSE", path)
	}

	// Retryable errors should dump the request and response
	out500 := run(t, fs.DumpErrors|fs.DumpBodies, "/500")
	assert.Contains(t, out500, "HTTP REQUEST")
	assert.Contains(t, out500, "HTTP RESPONSE")
	assert.Contains(t, out500, "500 Internal Server Error")
	// bodies are included because of the bodies flag
	assert.Contains(t, out500, "request body")
	assert.Contains(t, out500, "response body")

	out429 := run(t, fs.DumpErrors|fs.DumpBodies, "/429")
	assert.Contains(t, out429, "429 Too Many Requests")

	// --dump errors on its own dumps the headers but not the bodies
	headersOnly := run(t, fs.DumpErrors, "/500")
	assert.Contains(t, headersOnly, "HTTP REQUEST")
	assert.Contains(t, headersOnly, "HTTP RESPONSE")
	assert.Contains(t, headersOnly, "500 Internal Server Error")
	assert.NotContains(t, headersOnly, "response body")

	// --dump curl is also gated by errors: the curl command is only
	// logged for failed transactions
	curlOK := run(t, fs.DumpErrors|fs.DumpCurl, "/200")
	assert.NotContains(t, curlOK, "curl")
	curl500 := run(t, fs.DumpErrors|fs.DumpCurl, "/500")
	assert.Contains(t, curl500, "curl")
}
