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
	"path"
	"strings"
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

// genByte is the content of the synthetic file served by TestOpenRangeBeyondServerLimit.
func genByte(i int64) byte { return byte(i % 251) }

// The download server answers a Range starting at 2,000,000,000 or later with
// two zero bytes.  Open must not send such a range, and must still return the
// bytes that were asked for.
func TestOpenRangeBeyondServerLimit(t *testing.T) {
	const size = int64(2000000300)
	var badRanges atomic.Int32
	var srvURL string
	f, srv := newSAPIServer(t, map[string]http.HandlerFunc{
		"/dl/7": func(w http.ResponseWriter, r *http.Request) {
			ro, err := fs.ParseRangeOption(r.Header.Get("Range"))
			require.NoError(t, err)
			if ro.Start >= 2000000000 { // the real server's limit, independent of maxRangeStart
				badRanges.Add(1)
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 1-2/%d", size))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte{0, 0})
				return
			}
			end := size - 1
			if ro.End >= 0 && ro.End < end {
				end = ro.End
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", ro.Start, end, size))
			w.WriteHeader(http.StatusPartialContent)
			buf := make([]byte, 0, end-ro.Start+1)
			for i := ro.Start; i <= end; i++ {
				buf = append(buf, genByte(i))
			}
			_, _ = w.Write(buf)
		},
	})
	srvURL = srv.URL
	o := &Object{fs: f, remote: "big", id: "7", hasMetaData: true, size: size, url: srvURL + "/dl/7?x=1"}

	want := func(start, end int64) []byte {
		b := make([]byte, 0, end-start+1)
		for i := start; i <= end; i++ {
			b = append(b, genByte(i))
		}
		return b
	}
	for _, tc := range []struct {
		name       string
		opt        fs.OpenOption
		start, end int64
	}{
		{"below the limit", &fs.RangeOption{Start: 1999999900, End: 1999999999}, 1999999900, 1999999999},
		{"crossing the limit", &fs.RangeOption{Start: 1999999900, End: 2000000099}, 1999999900, 2000000099},
		{"exactly at the limit", &fs.RangeOption{Start: 2000000000, End: 2000000099}, 2000000000, 2000000099},
		{"beyond the limit", &fs.RangeOption{Start: 2000000100, End: 2000000199}, 2000000100, 2000000199},
		{"open ended", &fs.RangeOption{Start: 2000000100, End: -1}, 2000000100, size - 1},
		{"last bytes", &fs.RangeOption{Start: -1, End: 50}, size - 50, size - 1},
		{"seek", &fs.SeekOption{Offset: 2000000250}, 2000000250, size - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc, err := o.Open(context.Background(), tc.opt)
			require.NoError(t, err)
			got, err := io.ReadAll(rc)
			require.NoError(t, rc.Close())
			require.NoError(t, err)
			assert.Equal(t, want(tc.start, tc.end), got)
		})
	}
	assert.Zero(t, badRanges.Load(), "a range beyond the server limit was sent")
}

func TestClampRangeStart(t *testing.T) {
	in := []fs.OpenOption{&fs.HTTPOption{Key: "k", Value: "v"}, &fs.RangeOption{Start: 2500000000, End: 2500000099}}
	out, skip := clampRangeStart(in)
	assert.Equal(t, int64(2500000000-maxRangeStart), skip)
	assert.Equal(t, &fs.RangeOption{Start: maxRangeStart, End: 2500000099}, out[1])
	assert.Equal(t, &fs.RangeOption{Start: 2500000000, End: 2500000099}, in[1], "input must not be modified")

	same := []fs.OpenOption{&fs.RangeOption{Start: maxRangeStart, End: -1}}
	out, skip = clampRangeStart(same)
	assert.Zero(t, skip)
	assert.Equal(t, same, out)

	out, skip = clampRangeStart(nil)
	assert.Zero(t, skip)
	assert.Empty(t, out)
}

// Multi-thread downloads would read ranges beyond the server limit.
func TestNoMultiThreading(t *testing.T) {
	f, _ := newSAPIServer(t, nil)
	assert.True(t, f.Features().NoMultiThreading)
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

// A SAPI call is retried while the CDN answers 403 with an HTML page.
func TestCallJSONRetriesHTMLForbidden(t *testing.T) {
	var calls atomic.Int32
	f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media/folder": func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(http.StatusForbidden)
				_, _ = io.WriteString(w, `<!DOCTYPE html><html></html>`)
				return
			}
			_, _ = io.WriteString(w, `{"data":{"folders":[]}}`)
		},
	})
	_, err := f.listFolders(context.Background(), "1")
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
}

// A file the server is still validating is deleted once it is ready, and one
// already in the trash counts as deleted, also within a batch.
func TestDeleteMedia(t *testing.T) {
	for name, tc := range map[string]struct {
		ids     []string
		replies []string // error code of each delete call, "" for success
		want    [][]string
		err     bool
	}{
		"validating":      {ids: []string{"1"}, replies: []string{"MED-1017"}, want: [][]string{{"1"}, {"1"}}},
		"already deleted": {ids: []string{"1"}, replies: []string{"MED-1022"}, want: [][]string{{"1"}}},
		"batch with an already deleted item": {ids: []string{"1", "2"}, replies: []string{"MED-1022", "MED-1022"},
			want: [][]string{{"1", "2"}, {"1"}, {"2"}}},
		"other error": {ids: []string{"1"}, replies: []string{"MED-1001"}, want: [][]string{{"1"}}, err: true},
	} {
		t.Run(name, func(t *testing.T) {
			var calls [][]string
			replies := tc.replies
			f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
				"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "delete", r.URL.Query().Get("action"))
					var req api.IDsRequest
					require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
					var ids []string
					for _, id := range req.Data.IDs {
						ids = append(ids, id.String())
					}
					calls = append(calls, ids)
					if len(replies) > 0 {
						code := replies[0]
						replies = replies[1:]
						if code != "" {
							_, _ = fmt.Fprintf(w, `{"error":{"code":%q,"message":"refused"}}`, code)
							return
						}
					}
					_, _ = io.WriteString(w, `{"success":"Media deleted"}`)
				},
			})
			err := f.deleteMedia(context.Background(), tc.ids...)
			if tc.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, calls)
		})
	}
}

// The live files are the new and updated generic files that are not deleted.
func TestListFileIDs(t *testing.T) {
	f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/profile/changes": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"data":{"file":{"N":[1,2],"U":[2,3,4],"D":[4]},"picture":{"U":[5]}}}`)
		},
	})
	ids, err := f.listFileIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2", "3"}, ids)
}

func TestUploadTMPNames(t *testing.T) {
	for _, name := range []string{"probe.tmp", "probe.TMP", "dir/probe.TmP", "probe.tmp.bin", "probe.tmpx"} {
		t.Run(name, func(t *testing.T) {
			var writes atomic.Int32
			f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
				"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
					writes.Add(1)
					_, _ = io.WriteString(w, `{"data":{"media":[]}}`)
				},
				"/sapi/system/information": func(w http.ResponseWriter, r *http.Request) {
					writes.Add(1)
					_, _ = io.WriteString(w, `{}`)
				},
				"/sapi/upload/file": func(w http.ResponseWriter, r *http.Request) {
					writes.Add(1)
					if r.URL.Query().Get("action") == "save-metadata" {
						_, _ = io.WriteString(w, `{"id":"guid"}`)
					} else {
						_, _ = io.WriteString(w, `{"id":42}`)
					}
				},
			})
			src := object.NewStaticObjectInfo(name, time.Now(), 4, true, nil, nil)
			_, err := f.Put(context.Background(), bytes.NewReader([]byte("data")), src)
			if strings.EqualFold(path.Ext(name), ".tmp") {
				require.Error(t, err)
				assert.True(t, fserrors.IsNoRetryError(err), err)
				assert.Zero(t, writes.Load(), "refuse before any API calls or deletions")
			} else {
				require.NoError(t, err)
				assert.Positive(t, writes.Load())
			}
		})
	}
}

func TestUploadOverwritesInPlace(t *testing.T) {
	for _, mode := range []string{"listed", "object", "cached", "duplicates", "validating", "metadata failure", "stream failure", "picture"} {
		t.Run(mode, func(t *testing.T) {
			var deleted []string
			var metadataIDs []string
			target := api.Item{ID: "42", Name: "target.bin", MediaType: "file", Size: 3}
			if mode == "picture" {
				target.MediaType = "picture"
			}
			f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
				"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
					var req api.IDsRequest
					if r.URL.Query().Get("action") == "delete" {
						require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
						for _, id := range req.Data.IDs {
							deleted = append(deleted, id.String())
						}
						_, _ = io.WriteString(w, `{}`)
						return
					}
					var items []api.Item
					byID := r.URL.Query().Get("folderid") == ""
					if byID || mode != "object" && mode != "cached" {
						items = append(items, target)
					}
					if mode == "duplicates" && !byID {
						items = append(items, api.Item{ID: "43", Name: "TARGET.bin", MediaType: "file"})
					}
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"media": items}}))
				},
				"/sapi/upload/file": func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("action") == "save-metadata" {
						var req struct {
							Data struct{ ID string } `json:"data"`
						}
						require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
						metadataIDs = append(metadataIDs, req.Data.ID)
						if mode == "metadata failure" {
							_, _ = io.WriteString(w, `{"error":{"code":"MED-1001"}}`)
						} else if mode == "validating" && len(metadataIDs) == 1 {
							_, _ = io.WriteString(w, `{"error":{"code":"MED-1017"}}`)
						} else {
							_, _ = io.WriteString(w, `{"id":"guid"}`)
						}
						return
					}
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.Equal(t, "replacement", string(data))
					if mode == "stream failure" {
						_, _ = io.WriteString(w, `{"error":{"code":"MED-1001"}}`)
					} else if metadataIDs[len(metadataIDs)-1] == "42" {
						_, _ = io.WriteString(w, `{"id":42}`)
					} else {
						_, _ = io.WriteString(w, `{"id":99}`)
					}
				},
			})
			o := &Object{fs: f, remote: "target.bin"}
			if mode == "object" || mode == "metadata failure" || mode == "stream failure" {
				o.id = target.ID.String()
			}
			if mode == "cached" || mode == "metadata failure" || mode == "stream failure" {
				f.uploads.put(o.remote, target)
			}
			src := object.NewStaticObjectInfo(o.remote, time.Now(), 11, true, nil, nil)
			err := o.Update(context.Background(), bytes.NewReader([]byte("replacement")), src)
			if mode == "metadata failure" || mode == "stream failure" {
				require.Error(t, err)
				assert.Equal(t, "42", o.id)
				cached, ok := f.uploads.get(o.remote)
				assert.True(t, ok, "the previous version stays in the cache on failure")
				assert.Equal(t, target, cached)
			} else {
				require.NoError(t, err)
				if mode == "picture" {
					assert.Equal(t, "99", o.id)
					assert.Equal(t, []string{""}, metadataIDs)
				} else {
					assert.Equal(t, "42", o.id)
					for _, id := range metadataIDs {
						assert.Equal(t, "42", id)
					}
				}
			}
			if mode == "duplicates" {
				assert.Equal(t, []string{"43"}, deleted)
			} else if mode == "picture" {
				assert.Equal(t, []string{"42"}, deleted)
			} else {
				assert.Empty(t, deleted, "the overwritten file must not be deleted")
			}
		})
	}
}
