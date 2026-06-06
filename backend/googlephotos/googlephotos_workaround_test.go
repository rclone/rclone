package googlephotos

import (
	"context"
	"encoding/json"
	"io"
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

func TestRemoveTrashWorkaround(t *testing.T) {
	ctx := context.Background()

	// Track mock request count and URLs called
	calls := make(map[string]int)

	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.Method+" "+r.URL.Path]++

		switch {
		case r.Method == "GET" && r.URL.Path == "/albums":
			// Return list of albums (rclone_Trash does not exist yet)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.ListAlbums{
				Albums: []api.Album{
					{ID: "album1", Title: "my-album"},
				},
			})

		case r.Method == "POST" && r.URL.Path == "/albums":
			// Create rclone_Trash album
			var req api.CreateAlbum
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, "rclone_Trash", req.Album.Title)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.Album{
				ID:    "trash-album-123",
				Title: "rclone_Trash",
			})

		case r.Method == "POST" && r.URL.Path == "/albums/trash-album-123:batchAddMediaItems":
			// Add items to trash
			var req api.BatchAddItems
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, []string{"photo-123"}, req.MediaItemIDs)
			w.WriteHeader(http.StatusOK)

		case r.Method == "POST" && r.URL.Path == "/albums/album1:batchRemoveMediaItems":
			// Remove items from my-album
			var req api.BatchRemoveItems
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, []string{"photo-123"}, req.MediaItemIDs)
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Instantiate a test Fs manually
	f := &Fs{
		name:   "TestGphotos",
		root:   "album/my-album",
		unAuth: rest.NewClient(http.DefaultClient),
		srv:    rest.NewClient(http.DefaultClient).SetRoot(ts.URL),
		pacer:  fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(10*time.Millisecond))),
		albums: map[bool]*albums{},
	}
	f.srv.SetErrorHandler(errorHandler)

	// Build the local albums cache (mocking listAlbums result)
	_, err := f.listAlbums(ctx, false)
	require.NoError(t, err)

	// Create Object to remove (remote path is relative to the root "album/my-album")
	o := &Object{
		fs:     f,
		remote: "photo.jpg",
		id:     "photo-123",
	}

	// Remove item
	err = o.Remove(ctx)
	require.NoError(t, err)

	// Assertions
	assert.Equal(t, 1, calls["GET /albums"], "Should list albums to find rclone_Trash")
	assert.Equal(t, 1, calls["POST /albums"], "Should create rclone_Trash")
	assert.Equal(t, 1, calls["POST /albums/trash-album-123:batchAddMediaItems"], "Should add to trash album")
	assert.Equal(t, 1, calls["POST /albums/album1:batchRemoveMediaItems"], "Should remove from my-album")
}

func TestUpdateTrashWorkaround(t *testing.T) {
	ctx := context.Background()
	calls := make(map[string]int)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.Method+" "+r.URL.Path]++

		switch {
		case r.Method == "GET" && r.URL.Path == "/albums":
			// Return list containing rclone_Trash
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.ListAlbums{
				Albums: []api.Album{
					{ID: "album1", Title: "my-album", IsWriteable: true},
					{ID: "trash-album-123", Title: "rclone_Trash", IsWriteable: true},
				},
			})

		case r.Method == "POST" && r.URL.Path == "/uploads":
			// Upload media bytes
			body, _ := io.ReadAll(r.Body)
			assert.Equal(t, "new content", string(body))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("upload-token-abc"))

		case r.Method == "POST" && r.URL.Path == "/mediaItems:batchCreate":
			// Commit batch upload
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(api.BatchCreateResponse{
				NewMediaItemResults: []struct {
					UploadToken string `json:"uploadToken"`
					Status      struct {
						Message string `json:"message"`
						Code    int    `json:"code"`
					} `json:"status"`
					MediaItem api.MediaItem `json:"mediaItem"`
				}{
					{
						UploadToken: "upload-token-abc",
						MediaItem: api.MediaItem{
							ID:       "new-photo-456",
							Filename: "photo.jpg",
						},
					},
				},
			})

		case r.Method == "POST" && r.URL.Path == "/albums/trash-album-123:batchAddMediaItems":
			// Add old photo to trash
			var req api.BatchAddItems
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, []string{"old-photo-123"}, req.MediaItemIDs)
			w.WriteHeader(http.StatusOK)

		case r.Method == "POST" && r.URL.Path == "/albums/album1:batchRemoveMediaItems":
			// Remove old photo from my-album
			var req api.BatchRemoveItems
			_ = json.NewDecoder(r.Body).Decode(&req)
			assert.Equal(t, []string{"old-photo-123"}, req.MediaItemIDs)
			w.WriteHeader(http.StatusOK)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Instantiate a test Fs manually
	f := &Fs{
		name:   "TestGphotos",
		root:   "album/my-album",
		unAuth: rest.NewClient(http.DefaultClient),
		srv:    rest.NewClient(http.DefaultClient).SetRoot(ts.URL),
		pacer:  fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(10*time.Millisecond))),
		albums: map[bool]*albums{},
		opt: Options{
			BatchMode: "sync",
		},
	}
	f.srv.SetErrorHandler(errorHandler)

	var err error
	batcherOpts := defaultBatcherOptions
	batcherOpts.Mode = f.opt.BatchMode
	f.batcher, err = batcher.New(ctx, f, f.commitBatch, batcherOpts)
	require.NoError(t, err)

	// Build the local albums cache
	_, err = f.listAlbums(ctx, false)
	require.NoError(t, err)

	// Create Object to update (remote path is relative to the root "album/my-album")
	o := &Object{
		fs:     f,
		remote: "photo.jpg",
		id:     "old-photo-123",
	}

	// Update object with new contents
	err = o.Update(ctx, strings.NewReader("new content"), nil)
	require.NoError(t, err)

	// Assertions
	assert.Equal(t, "new-photo-456", o.id, "Object ID should be updated to new ID")
	assert.Equal(t, 1, calls["POST /uploads"], "Should upload new photo")
	assert.Equal(t, 1, calls["POST /mediaItems:batchCreate"], "Should commit upload batch")
	assert.Equal(t, 1, calls["POST /albums/trash-album-123:batchAddMediaItems"], "Should trash old duplicate ID")
	assert.Equal(t, 1, calls["POST /albums/album1:batchRemoveMediaItems"], "Should remove old duplicate ID from album")
}
