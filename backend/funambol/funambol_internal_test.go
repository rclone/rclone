package funambol

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/funambol/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSAPIServer serves the root folder plus the given handlers keyed by path
// and returns an Fs logged in with testKey.
func newSAPIServer(t *testing.T, handlers map[string]http.HandlerFunc) (*Fs, *httptest.Server) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sapi/media/folder/root", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"folders":[{"id":1,"name":"root"}]}}`)
	})
	for p, h := range handlers {
		mux.HandleFunc(p, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cookies, err := json.Marshal([]cookiePair{{"validationKey", testKey}})
	require.NoError(t, err)
	m := configmap.Simple{
		"user":     testUser,
		"endpoint": srv.URL,
		"cookies":  base64.StdEncoding.EncodeToString(cookies),
	}
	f, err := NewFs(context.Background(), "test", "", m)
	require.NoError(t, err)
	return f.(*Fs), srv
}

// A file must be found straight after its upload, before the server lists it.
func TestNewObjectFindsRecentUpload(t *testing.T) {
	var deleted atomic.Value
	f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media/folder": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("action") == "save" {
				_, _ = io.WriteString(w, `{"data":{"folder":{"id":2,"name":"locks"}}}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":{"folders":[]}}`)
		},
		"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("action") == "delete" {
				body, _ := io.ReadAll(r.Body)
				deleted.Store(string(body))
			}
			_, _ = io.WriteString(w, `{"data":{"media":[],"more":false}}`)
		},
		"/sapi/upload/file": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("action") == "save-metadata" {
				_, _ = io.WriteString(w, `{"id":"guid"}`)
				return
			}
			_, _ = io.WriteString(w, `{"id":42}`)
		},
	})
	ctx := context.Background()
	data := []byte("lock")
	src := object.NewStaticObjectInfo("locks/a", time.Now(), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), src)
	require.NoError(t, err)

	o, err := f.NewObject(ctx, "locks/a")
	require.NoError(t, err)
	assert.Equal(t, "42", o.(*Object).ID())
	assert.Equal(t, int64(len(data)), o.Size())

	require.NoError(t, o.Remove(ctx))
	assert.Contains(t, deleted.Load(), "42")

	_, err = f.NewObject(ctx, "locks/a")
	assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
}

// Large uploads need asynchronous processing to avoid a gateway timeout
// after the server has received the entire file.
func TestUploadLargeFileAsynchronously(t *testing.T) {
	const cutoff = int64(200 * 1024 * 1024)
	for _, size := range []int64{8, cutoff, cutoff + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
				"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.WriteString(w, `{"data":{"media":[],"more":false}}`)
				},
				"/sapi/upload/file": func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("action") == "save-metadata" {
						_, _ = io.WriteString(w, `{"id":"guid"}`)
						return
					}
					n, err := io.Copy(io.Discard, r.Body)
					assert.NoError(t, err)
					assert.Equal(t, size, n)
					async := r.URL.Query().Get("acceptasynchronous")
					if size > cutoff {
						if !assert.Equal(t, "true", async) {
							w.WriteHeader(http.StatusGatewayTimeout)
							return
						}
					} else {
						assert.Empty(t, async)
					}
					_, _ = io.WriteString(w, `{"id":42,"status":"U"}`)
				},
			})
			src := object.NewStaticObjectInfo("test.bin", time.Now(), size, true, nil, nil)
			o, err := f.Put(context.Background(), readers.NewPatternReader(size), src)
			require.NoError(t, err)
			assert.Equal(t, "42", o.(*Object).ID())
			assert.Equal(t, size, o.Size())
		})
	}
}

// A file bigger than the limit the server publishes fails before the upload,
// with an error that is not retried.
func TestUploadRejectsFileOverServerLimit(t *testing.T) {
	const limit = int64(1024 * 1024)
	var infos, uploads atomic.Int32
	f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/system/information": func(w http.ResponseWriter, r *http.Request) {
			infos.Add(1)
			_, _ = io.WriteString(w, `{"sapi.upload.max-size-in-mb":"1"}`)
		},
		"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"data":{"media":[],"more":false}}`)
		},
		"/sapi/upload/file": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("action") == "save-metadata" {
				uploads.Add(1)
				_, _ = io.WriteString(w, `{"id":"guid"}`)
				return
			}
			_, _ = io.Copy(io.Discard, r.Body)
			_, _ = io.WriteString(w, `{"id":42,"status":"U"}`)
		},
	})
	ctx := context.Background()

	src := object.NewStaticObjectInfo("ok.bin", time.Now(), limit, true, nil, nil)
	_, err := f.Put(ctx, readers.NewPatternReader(limit), src)
	require.NoError(t, err)

	src = object.NewStaticObjectInfo("big.bin", time.Now(), limit+1, true, nil, nil)
	_, err = f.Put(ctx, readers.NewPatternReader(limit+1), src)
	require.Error(t, err)
	assert.True(t, fserrors.IsNoRetryError(err), err)
	assert.Contains(t, err.Error(), "chunker")

	assert.Equal(t, int32(1), uploads.Load())
	assert.Equal(t, int32(1), infos.Load(), "the limit is read once")
}

// MED-1000 is a transient server fault and must be retried.
func TestCallJSONRetriesMED1000(t *testing.T) {
	var calls atomic.Int32
	f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media/folder": func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				_, _ = io.WriteString(w, `{"error":{"code":"MED-1000","message":"Unknown exception in media handling"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":{"folders":[{"id":2,"name":"sub"}]}}`)
		},
	})
	folders, err := f.listFolders(context.Background(), "1")
	require.NoError(t, err)
	assert.Len(t, folders, 1)
	assert.Equal(t, int32(2), calls.Load())
}

// A reply to a request sent with an old key must not replace a newer key.
func TestRefreshOnErrorKeepsNewerKey(t *testing.T) {
	f, _ := newSAPIServer(t, nil)
	stale := &api.Error{Code: "SEC-1003", Data: "from-old-request"}

	f.setValidationKey("newer")
	assert.True(t, f.refreshOnError(stale, "older"))
	assert.Equal(t, "newer", f.key())

	assert.True(t, f.refreshOnError(stale, "newer"))
	assert.Equal(t, "from-old-request", f.key())

	assert.False(t, f.refreshOnError(&api.Error{Code: "SEC-1003"}, f.key()))
}

// A 401 for a key that another call already replaced is retried with the
// current key instead of asking for a reconnect.
func TestCallJSONRetriesAfterConcurrentRotation(t *testing.T) {
	var f *Fs
	var calls atomic.Int32
	f, _ = newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media/folder": func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				f.setValidationKey("rotated") // a concurrent call renewed the session
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":{"code":"SEC-1004"}}`)
				return
			}
			assert.Equal(t, "rotated", r.URL.Query().Get("validationkey"))
			_, _ = io.WriteString(w, `{"data":{"folders":[]}}`)
		},
	})
	_, err := f.listFolders(context.Background(), "1")
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
}

// A refused download reads the metadata again and retries with the key.
func TestOpenRetriesAfterForbidden(t *testing.T) {
	var lists, downloads atomic.Int32
	var srvURL string
	f, srv := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
			lists.Add(1)
			_, _ = io.WriteString(w, `{"data":{"media":[{"id":7,"name":"config","size":5,"url":"`+srvURL+`/dl/7?x=1"}],"more":false}}`)
		},
		"/dl/7": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, testKey, r.URL.Query().Get("validationkey"))
			assert.Equal(t, "1", r.URL.Query().Get("x"))
			if downloads.Add(1) == 1 {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `<!DOCTYPE html><html lang="en"></html>`)
				return
			}
			_, _ = io.WriteString(w, "hello")
		},
	})
	srvURL = srv.URL
	ctx := context.Background()
	o, err := f.NewObject(ctx, "config")
	require.NoError(t, err)
	rc, err := o.Open(ctx)
	require.NoError(t, err)
	body, err := io.ReadAll(rc)
	require.NoError(t, rc.Close())
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))
	assert.Equal(t, int32(2), lists.Load())
	assert.Equal(t, int32(2), downloads.Load())
}

func TestDownloadURL(t *testing.T) {
	f, srv := newSAPIServer(t, nil)
	got, err := url.Parse(f.downloadURL(srv.URL + "/sapi/download/file?id=7"))
	require.NoError(t, err)
	assert.Equal(t, testKey, got.Query().Get("validationkey"))
	assert.Equal(t, "7", got.Query().Get("id"))

	other := "https://cdn.example.com/f?sig=abc"
	assert.Equal(t, other, f.downloadURL(other))
}

func TestRedactKey(t *testing.T) {
	err := redactKey(&url.Error{Op: "Post", URL: "https://example.com/sapi/upload/file?action=save&validationkey=secret", Err: context.Canceled})
	assert.NotContains(t, err.Error(), "secret")
	assert.Contains(t, err.Error(), "action=save")
	assert.Contains(t, err.Error(), "validationkey=REDACTED")
	assert.ErrorIs(t, err, context.Canceled)
}
