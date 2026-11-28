package linkbox

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPrefix(t *testing.T) {
	for _, size := range []int64{1, 1024, prefixSize - 1, prefixSize, prefixSize + 1, 3 * prefixSize} {
		t.Run(fmt.Sprintf("%d", size), func(t *testing.T) {
			inUse := pool.Global().InUse()
			content := bytes.Repeat([]byte("x"), int(size))
			for i := range content {
				content[i] = byte(i)
			}
			wantPrefix := content[:min(size, prefixSize)]
			wantMD5 := fmt.Sprintf("%x", md5.Sum(wantPrefix))

			in := bytes.NewReader(content)
			rw, gotMD5, err := readPrefix(in)
			require.NoError(t, err)
			assert.Equal(t, wantMD5, gotMD5)
			assert.Equal(t, int64(len(wantPrefix)), rw.Size())

			// The prefix followed by the rest of in must reproduce the content
			got, err := io.ReadAll(io.MultiReader(rw, in))
			require.NoError(t, err)
			assert.Equal(t, content, got)

			require.NoError(t, rw.Close())
			assert.Equal(t, inUse, pool.Global().InUse(), "pool buffers leaked")
		})
	}
}

func TestUpdateNoPrematureDelete(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var failUpload bool
	var failDelete bool

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.URL.Path)
		failUp := failUpload
		failDel := failDelete
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/get_upload_url":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 1,
				"msg":    "success",
				"data": map[string]any{
					"signUrl": "http://" + r.Host + "/put_upload",
				},
			})
		case "/put_upload":
			if r.Method == "HEAD" {
				w.WriteHeader(http.StatusOK)
				return
			}
			if failUp {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		case "/file_del":
			if failDel {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status": 0,
					"msg":    "delete failed",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 1,
				"msg":    "success",
			})
		case "/folder_upload_file":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 1,
				"msg":    "success",
			})
		case "/file/my_file_list/web":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 1,
				"msg":    "success",
				"data": map[string]any{
					"list": []map[string]any{
						{
							"type":    "file",
							"name":    "a.txt",
							"url":     "http://example.com/a.txt",
							"ctime":   time.Now().Unix(),
							"size":    4,
							"id":      100,
							"pid":     0,
							"item_id": "new_item_id",
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 1,
			})
		}
	}))
	defer ts.Close()

	oldAPIURL := linkboxAPIURL
	oldWebURL := linkboxWebAPIURL
	linkboxAPIURL = ts.URL + "/"
	linkboxWebAPIURL = ts.URL + "/"
	defer func() {
		linkboxAPIURL = oldAPIURL
		linkboxWebAPIURL = oldWebURL
	}()

	f := &Fs{
		name:     "linkbox",
		root:     "",
		opt:      Options{Token: "test_token"},
		srv:      rest.NewClient(ts.Client()),
		pacer:    fs.NewPacer(context.Background(), pacer.NewDefault()),
		webToken: "test_web_token",
	}
	f.dirCache = dircache.New("", rootID, f)

	// Test 1: Upload fails -> file_del MUST NOT be called!
	t.Run("UploadFailureDoesNotDeleteOldFile", func(t *testing.T) {
		mu.Lock()
		calls = nil
		failUpload = true
		failDelete = false
		mu.Unlock()

		o := &Object{
			fs:     f,
			remote: "a.txt",
			itemID: "old_item_id",
		}
		src := object.NewStaticObjectInfo("a.txt", time.Now(), 4, true, nil, f)
		err := o.Update(context.Background(), bytes.NewReader([]byte("test")), src)
		require.Error(t, err)

		mu.Lock()
		defer mu.Unlock()
		assert.NotContains(t, calls, "/file_del", "file_del should not be called when upload fails")
		assert.Equal(t, "old_item_id", o.itemID, "itemID should remain unchanged on failed upload")
	})

	// Test 2: Upload succeeds, but delete fails -> Update MUST return error!
	t.Run("FailedDeleteReturnsError", func(t *testing.T) {
		mu.Lock()
		calls = nil
		failUpload = false
		failDelete = true
		mu.Unlock()

		o := &Object{
			fs:     f,
			remote: "a.txt",
			itemID: "old_item_id",
		}
		src := object.NewStaticObjectInfo("a.txt", time.Now(), 4, true, nil, f)
		err := o.Update(context.Background(), bytes.NewReader([]byte("test")), src)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to remove existing file")

		mu.Lock()
		defer mu.Unlock()
		assert.Contains(t, calls, "/file_del")
		assert.NotContains(t, calls, "/folder_upload_file", "folder_upload_file should not be called if delete failed")
	})

	// Test 3: Upload and delete both succeed -> file_del happens after put_upload and before folder_upload_file
	t.Run("SuccessfulUpdateOrder", func(t *testing.T) {
		mu.Lock()
		calls = nil
		failUpload = false
		failDelete = false
		mu.Unlock()

		o := &Object{
			fs:     f,
			remote: "a.txt",
			itemID: "old_item_id",
		}
		src := object.NewStaticObjectInfo("a.txt", time.Now(), 4, true, nil, f)
		err := o.Update(context.Background(), bytes.NewReader([]byte("test")), src)
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()
		getUploadIdx, putIdx, delIdx, folderUploadIdx := -1, -1, -1, -1
		for i, call := range calls {
			switch call {
			case "/get_upload_url":
				getUploadIdx = i
			case "/put_upload":
				putIdx = i
			case "/file_del":
				delIdx = i
			case "/folder_upload_file":
				folderUploadIdx = i
			}
		}
		assert.True(t, getUploadIdx >= 0 && getUploadIdx < putIdx, "get_upload_url before put_upload")
		assert.True(t, putIdx < delIdx, "put_upload before file_del")
		assert.True(t, delIdx < folderUploadIdx, "file_del before folder_upload_file")
		assert.Equal(t, "new_item_id", o.itemID)
	})
}
