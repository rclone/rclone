package filelu

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetUploadChunkSize changes the configured chunk size, returning the old value.
//
// It is only called by the integration tests while no transfer is in progress.
func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	return old, nil
}

// SetUploadCutoff changes the configured upload cutoff, returning the old value.
//
// It is only called by the integration tests while no transfer is in progress.
func (f *Fs) SetUploadCutoff(cutoff fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.UploadCutoff = f.opt.UploadCutoff, cutoff
	return old, nil
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)

func TestUploadFileNoPrematureDelete(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var ts *httptest.Server

	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.URL.Path)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/upload/server":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  200,
				"sess_id": "test_sess",
				"result":  ts.URL + "/upload_dest",
			})
		case "/upload_dest":
			// simulate upload failure (e.g. quota exceeded)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"file_code":   "",
					"file_status": "quota exceeded",
				},
			})
		case "/file/remove":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200,
				"msg":    "OK",
			})
		default:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200,
			})
		}
	}))
	defer ts.Close()

	client := ts.Client()
	f := &Fs{
		endpoint: ts.URL,
		client:   client,
		srv:      rest.NewClient(client).SetRoot(ts.URL),
		pacer:    pacer.New(),
		opt:      Options{Key: "testkey"},
	}

	err := f.uploadFile(context.Background(), bytes.NewReader([]byte("test data")), "a.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")

	mu.Lock()
	defer mu.Unlock()
	assert.NotContains(t, calls, "/file/remove", "existing file should not be deleted before/on failed upload")
}
