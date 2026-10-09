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
