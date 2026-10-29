package shade

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestChunkWriter returns a chunk writer pointed at server, which must
// answer the part URL request with a URL back to itself and the part PUT.
func newTestChunkWriter(t *testing.T, server *httptest.Server) *shadeChunkWriter {
	ctx := context.Background()
	f := &Fs{
		srv:      rest.NewClient(server.Client()),
		endpoint: server.URL,
		drive:    "drive",
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(time.Millisecond))),
		token:    "token",
		tokenExp: time.Now().Add(time.Hour),
	}
	return &shadeChunkWriter{initToken: "init", f: f}
}

// partServer returns a test server which hands out a part URL pointing at
// itself and passes part PUTs to put.
func partServer(t *testing.T, put func(w http.ResponseWriter, body []byte)) *httptest.Server {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/upload/multipart/part/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"url":%q}`, server.URL+"/put")
			return
		}
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		put(w, body)
	}))
	return server
}

// TestWriteChunkError checks that a part which keeps failing reports the
// part number in the error.
func TestWriteChunkError(t *testing.T) {
	server := partServer(t, func(w http.ResponseWriter, body []byte) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	s := newTestChunkWriter(t, server)
	_, err := s.WriteChunk(context.Background(), 2, bytes.NewReader([]byte("shade")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload part 3:")
}

// TestOpenUsesConfiguredClient checks that the first download request, which
// must not follow the redirect to the presigned URL, is sent with the
// configured HTTP client rather than a bare http.Client.
func TestOpenUsesConfiguredClient(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())
	ci.UserAgent = "rclone-shade-test"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "rclone-shade-test", r.UserAgent())
		if strings.HasSuffix(r.URL.Path, "/fs/download") {
			assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))
			w.Header().Set("Location", server.URL+"/presigned")
			w.WriteHeader(http.StatusTemporaryRedirect)
			_, _ = io.WriteString(w, server.URL+"/presigned")
			return
		}
		assert.Equal(t, "/presigned", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"), "the presigned URL must not get the token")
		_, _ = io.WriteString(w, "hello")
	}))
	defer server.Close()

	f := &Fs{
		srv:      rest.NewClient(fshttp.NewClient(ctx)),
		noRedir:  newNoRedirectClient(ctx),
		endpoint: server.URL,
		drive:    "drive",
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(time.Millisecond))),
		token:    "token",
		tokenExp: time.Now().Add(time.Hour),
	}
	o := &Object{fs: f, remote: "file.txt", size: 5}

	in, err := o.Open(ctx)
	require.NoError(t, err)
	data, err := io.ReadAll(in)
	require.NoError(t, err)
	require.NoError(t, in.Close())
	assert.Equal(t, "hello", string(data))
}
