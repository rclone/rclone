package pikpak

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeTracker counts Close calls on a response body.
type closeTracker struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (b closeTracker) Close() error {
	b.closed.Add(1)
	return b.ReadCloser.Close()
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// newDownloadTestObject returns an object whose downloads are served by srv.
// closed counts response bodies closed through the client.
func newDownloadTestObject(t *testing.T, srv *httptest.Server, closed *atomic.Int32) *Object {
	t.Helper()
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 3
	client := *srv.Client()
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		res, err := base.RoundTrip(req)
		if err == nil && res != nil && res.Body != nil {
			res.Body = closeTracker{ReadCloser: res.Body, closed: closed}
		}
		return res, err
	})
	return &Object{
		fs: &Fs{
			client: &client,
			pacer:  fs.NewPacer(ctx, &pacer.ZeroDelayCalculator{}),
		},
		size: 1,
	}
}

// TestOpenHTTPStatus checks that a non-2xx download response is an error
// (body closed, retriable statuses retried by the pacer) and that 200 and
// 206 responses are still returned as file content.
func TestOpenHTTPStatus(t *testing.T) {
	const htmlBody = "<html><head><title>502 Bad Gateway</title></head><body>502</body></html>"
	fileBody := []byte("pikpak-file-bytes-0123456789")

	t.Run("502", func(t *testing.T) {
		var closed atomic.Int32
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, htmlBody)
		}))
		t.Cleanup(srv.Close)
		o := newDownloadTestObject(t, srv, &closed)
		in, err := o.open(context.Background(), srv.URL)
		if err == nil {
			got, readErr := io.ReadAll(in)
			require.NoError(t, readErr)
			require.NoError(t, in.Close())
			t.Fatalf("open returned the error page as file data (%d bytes): %q", len(got), got)
		}
		assert.Nil(t, in)
		assert.ErrorContains(t, err, "502")
		assert.NotContains(t, err.Error(), htmlBody)
		assert.Equal(t, int32(3), hits.Load(), "502 should be retried by the pacer")
		assert.Equal(t, hits.Load(), closed.Load(), "error response bodies should be closed")
	})

	t.Run("404", func(t *testing.T) {
		var closed atomic.Int32
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			http.Error(w, "missing", http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)
		o := newDownloadTestObject(t, srv, &closed)
		in, err := o.open(context.Background(), srv.URL)
		require.Error(t, err)
		assert.Nil(t, in)
		assert.Equal(t, int32(1), hits.Load(), "404 is not a retry status")
		assert.Equal(t, hits.Load(), closed.Load())
	})

	t.Run("502 then 200", func(t *testing.T) {
		var closed atomic.Int32
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if hits.Add(1) == 1 {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = io.WriteString(w, htmlBody)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(fileBody)
		}))
		t.Cleanup(srv.Close)
		o := newDownloadTestObject(t, srv, &closed)
		in, err := o.open(context.Background(), srv.URL)
		require.NoError(t, err)
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		require.NoError(t, in.Close())
		assert.Equal(t, fileBody, got)
		assert.Equal(t, int32(2), hits.Load())
		assert.Equal(t, int32(2), closed.Load())
	})

	t.Run("200", func(t *testing.T) {
		var closed atomic.Int32
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.Header.Get("Range") != "" {
				t.Errorf("unexpected Range header %q", r.Header.Get("Range"))
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(fileBody)
		}))
		t.Cleanup(srv.Close)
		o := newDownloadTestObject(t, srv, &closed)
		o.size = int64(len(fileBody))
		in, err := o.open(context.Background(), srv.URL)
		require.NoError(t, err)
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		assert.Equal(t, int32(0), closed.Load(), "successful body stays open for the caller")
		require.NoError(t, in.Close())
		assert.Equal(t, fileBody, got)
		assert.Equal(t, int32(1), hits.Load())
		assert.Equal(t, int32(1), closed.Load())
	})

	t.Run("206", func(t *testing.T) {
		var closed atomic.Int32
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			assert.Equal(t, "bytes=0-3", r.Header.Get("Range"))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Range", "bytes 0-3/"+strconv.Itoa(len(fileBody)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(fileBody[:4])
		}))
		t.Cleanup(srv.Close)
		o := newDownloadTestObject(t, srv, &closed)
		o.size = int64(len(fileBody))
		in, err := o.open(context.Background(), srv.URL, &fs.RangeOption{Start: 0, End: 3})
		require.NoError(t, err)
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		require.NoError(t, in.Close())
		assert.Equal(t, fileBody[:4], got)
		assert.Equal(t, int32(1), hits.Load())
		assert.Equal(t, int32(1), closed.Load())
	})
}
