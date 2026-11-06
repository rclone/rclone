package googlephotos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/googlephotos/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/lib/batcher"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAsyncBatchModePanic is a regression test for the nil-pointer panic that
// occurred when batch_mode=async was used. In async mode batcher.Commit
// returns immediately with a nil *api.MediaItem result; the old code passed
// that nil directly to o.setMetaData which dereferenced it unconditionally.
//
// This test should FAIL (panic) against the unfixed code and PASS after the fix.
func TestAsyncBatchModePanic(t *testing.T) {
	ctx := context.Background()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/albums":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.ListAlbums{
				Albums: []api.Album{
					{ID: "album1", Title: "my-album", IsWriteable: true},
				},
			})
		case r.Method == "POST" && r.URL.Path == "/uploads":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("upload-token-async"))
		default:
			// In async mode the batcher does not call batchCreate synchronously,
			// so we should never reach mediaItems:batchCreate during Put.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	f := &Fs{
		name:   "TestGphotos",
		root:   "album/my-album",
		unAuth: rest.NewClient(http.DefaultClient),
		srv:    rest.NewClient(http.DefaultClient).SetRoot(ts.URL),
		pacer:  fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(10*time.Millisecond))),
		albums: map[bool]*albums{},
		opt: Options{
			BatchMode: "async",
		},
	}
	f.srv.SetErrorHandler(errorHandler)

	var err error
	batcherOpts := defaultBatcherOptions
	batcherOpts.Mode = f.opt.BatchMode
	f.batcher, err = batcher.New(ctx, f, f.commitBatch, batcherOpts)
	require.NoError(t, err)
	defer f.batcher.Shutdown()

	_, err = f.listAlbums(ctx, false)
	require.NoError(t, err)

	// assert.NotPanics documents the expected post-fix behaviour.
	// Before the fix this panics with: "runtime error: invalid memory address
	// or nil pointer dereference" inside setMetaData(info) when info is nil.
	src := mockobject.New("photo.jpg").WithContent([]byte("content"), mockobject.SeekModeNone)
	assert.NotPanics(t, func() {
		_, _ = f.Put(ctx, strings.NewReader("content"), src)
	})
}
