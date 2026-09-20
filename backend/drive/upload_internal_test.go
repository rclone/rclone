package drive

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResumableUploadRetry checks that a chunk which fails with a 5xx is
// retried successfully. The chunk buffer is pool-backed and implements
// io.Closer, so if it reaches the http transport unwrapped the transport
// closes it after the failed attempt, returning its pages to the pool, and
// the retry then reads a freed buffer.
func TestResumableUploadRetry(t *testing.T) {
	content := bytes.Repeat([]byte("resumable"), 512)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		assert.Equal(t, content, body, "retried chunk should re-send the full chunk")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fake-id","name":"remote"}`))
	}))
	defer server.Close()

	f := &Fs{
		pacer:  fs.NewPacer(context.Background(), pacer.NewGoogleDrive()),
		client: server.Client(),
	}
	f.opt.ChunkSize = fs.SizeSuffix(len(content))
	rx := &resumableUpload{
		f:             f,
		remote:        "remote",
		URI:           server.URL,
		Media:         bytes.NewReader(content),
		MediaType:     "application/octet-stream",
		ContentLength: int64(len(content)),
	}
	info, err := rx.Upload(context.Background())
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "fake-id", info.Id)
	assert.Equal(t, int32(2), attempts.Load(), "expected exactly one failed attempt and one retry")
}
