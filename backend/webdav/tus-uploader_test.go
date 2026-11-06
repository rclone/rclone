package webdav

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"weak"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTusTestUploader(t *testing.T, server *httptest.Server, contents string) *Uploader {
	t.Helper()
	f := &Fs{
		opt:   Options{ChunkSize: 4},
		srv:   rest.NewClient(server.Client()),
		pacer: fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(0))),
	}
	// Wrap the reader so NewUpload buffers it, as it does for transfer readers.
	upload := NewUpload(io.LimitReader(strings.NewReader(contents), int64(len(contents))), int64(len(contents)), nil, "")
	require.NotNil(t, upload)
	return NewUploader(f, server.URL, upload, 0)
}

func tusTestHandler(t *testing.T, contents string, fail *atomic.Bool) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "1.0.0", r.Header.Get("Tus-Resumable"))
		if fail != nil && fail.Load() {
			w.WriteHeader(http.StatusConflict)
			return
		}
		offset, err := strconv.Atoi(r.Header.Get("Upload-Offset"))
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		end := min(offset+4, len(contents))
		assert.Equal(t, contents[offset:end], string(body))
		w.Header().Set("Upload-Offset", strconv.Itoa(end))
		w.WriteHeader(http.StatusNoContent)
	}
}

func TestTusUploadReleasesBuffer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contents string
		fail     bool
	}{
		{name: "Success", contents: "abcdefghij"},
		{name: "Error", contents: "abcdefghij", fail: true},
		{name: "Empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fail atomic.Bool
			fail.Store(tc.fail)
			server := httptest.NewServer(tusTestHandler(t, tc.contents, &fail))
			defer server.Close()

			released := func() weak.Pointer[Upload] {
				uploader := newTusTestUploader(t, server, tc.contents)
				err := uploader.Upload(context.Background())
				if tc.fail {
					require.ErrorIs(t, err, ErrOffsetMismatch)
				} else {
					require.NoError(t, err)
					assert.True(t, uploader.upload.Finished())
					require.NoError(t, uploader.Upload(context.Background()))
				}
				ref := weak.Make(uploader.upload)
				runtime.KeepAlive(uploader)
				return ref
			}()

			require.Eventually(t, func() bool {
				runtime.GC()
				return released.Value() == nil
			}, 5*time.Second, 10*time.Millisecond, "completed upload still retains its buffered contents")
		})
	}
}

func TestTusUploadRetry(t *testing.T) {
	const contents = "abcdefghij"
	var fail atomic.Bool
	fail.Store(true)
	server := httptest.NewServer(tusTestHandler(t, contents, &fail))
	defer server.Close()

	uploader := newTusTestUploader(t, server, contents)
	require.ErrorIs(t, uploader.Upload(context.Background()), ErrOffsetMismatch)
	assert.Zero(t, uploader.upload.Offset())
	fail.Store(false)
	require.NoError(t, uploader.Upload(context.Background()))
	assert.Equal(t, int64(len(contents)), uploader.upload.Offset())
	assert.True(t, uploader.upload.Finished())
	require.NoError(t, uploader.Upload(context.Background()))
}
