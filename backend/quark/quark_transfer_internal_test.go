package quark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadMultipart(t *testing.T) {
	var (
		mu          sync.Mutex
		parts       []string
		commitBody  string
		authMetas   []string
		finishCalls atomic.Int32
		firstPart   atomic.Int32
		inFlight    atomic.Int32
		maxInFlight atomic.Int32
		twoStarted  = make(chan struct{})
		startOnce   sync.Once
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/upload/pre":
			var body uploadPreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "hello.txt", body.FileName)
			assert.EqualValues(t, 7, body.Size)
			assert.Equal(t, rootID, body.ParentID)
			assert.False(t, body.ParallelUpload)
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"task_id":"task-id","bucket":"","obj_key":"object-key","upload_id":"upload-id","upload_url":%q,"auth_info":{},"callback":{}},"metadata":{"part_size":3,"part_thread":2}}`, serverURL(r))
		case "/1/clouddrive/file/update/hash":
			var body updateHashRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "7ac66c0f148de9519b8bd264312c4d64", body.MD5)
			assert.Equal(t, "2fb5e13419fc89246865e7a324f476ec624e8740", body.SHA1)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"finish":false}}`)
		case "/1/clouddrive/file/upload/auth":
			var body uploadAuthRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			mu.Lock()
			authMetas = append(authMetas, body.AuthMeta)
			mu.Unlock()
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"auth_key":"oss-auth"}}`)
		case "/object-key":
			assert.Equal(t, "oss-auth", r.Header.Get("Authorization"))
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			if r.Method == http.MethodPut {
				current := inFlight.Add(1)
				defer inFlight.Add(-1)
				for {
					maximum := maxInFlight.Load()
					if current <= maximum || maxInFlight.CompareAndSwap(maximum, current) {
						break
					}
				}
				if current >= 2 {
					startOnce.Do(func() { close(twoStarted) })
				}
				select {
				case <-twoStarted:
				case <-time.After(time.Second):
				}
				partNumber := r.URL.Query().Get("partNumber")
				if partNumber == "1" && firstPart.Add(1) == 1 {
					http.Error(w, "retry me", http.StatusServiceUnavailable)
					return
				}
				mu.Lock()
				parts = append(parts, string(body))
				mu.Unlock()
				w.Header().Set("ETag", `"etag-`+partNumber+`"`)
				w.WriteHeader(http.StatusOK)
				return
			}
			commitBody = string(body)
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, `<CompleteMultipartUploadResult/>`)
		case "/1/clouddrive/file/upload/finish":
			finishCalls.Add(1)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"uploaded-id","size":7,"md5":"7ac66c0f148de9519b8bd264312c4d64"}}`)
		case "/1/clouddrive/file/sort":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"uploaded-id","pdir_fid":"0","file_name":"hello.txt","file":true,"size":7}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	src := object.NewStaticObjectInfo("hello.txt", time.Unix(1710000000, 0), 7, true, nil, f)
	obj, err := f.upload(context.Background(), bytes.NewBufferString("abcdefg"), src, "hello.txt", rootID)
	require.NoError(t, err)
	assert.Equal(t, "uploaded-id", obj.id)
	assert.ElementsMatch(t, []string{"abc", "def", "g"}, parts)
	assert.Contains(t, commitBody, `<PartNumber>1</PartNumber><ETag>"etag-1"</ETag>`)
	assert.Contains(t, commitBody, `<PartNumber>3</PartNumber><ETag>"etag-3"</ETag>`)
	assert.NotEmpty(t, authMetas)
	assert.EqualValues(t, 2, firstPart.Load())
	assert.EqualValues(t, 2, maxInFlight.Load())
	assert.EqualValues(t, 1, finishCalls.Load())
}

func TestResolvePartThreads(t *testing.T) {
	tests := []struct {
		name          string
		serverThreads int
		partCount     int
		want          int
	}{
		{name: "missing server value", partCount: 10, want: 1},
		{name: "server value", serverThreads: 9, partCount: 10, want: 9},
		{name: "fewer parts", serverThreads: 9, partCount: 3, want: 3},
		{name: "safety limit", serverThreads: 100, partCount: 100, want: maxUploadPartThreads},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, resolvePartThreads(test.serverThreads, test.partCount))
		})
	}
}

func TestUploadInstantAndUnknownSize(t *testing.T) {
	var preSize int64
	var ossCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/upload/pre":
			var body uploadPreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			preSize = body.Size
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"task_id":"task-id","bucket":"","obj_key":"object-key","upload_id":"upload-id","upload_url":%q,"auth_info":{},"callback":{}},"metadata":{"part_size":8}}`, serverURL(r))
		case "/1/clouddrive/file/update/hash":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"finish":true,"fid":"instant-id"}}`)
		case "/1/clouddrive/file/sort":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"instant-id","pdir_fid":"0","file_name":"unknown.bin","file":true,"size":12}]}}`)
		case "/object-key", "/1/clouddrive/file/upload/finish":
			ossCalls.Add(1)
			http.Error(w, "must not upload after instant match", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	src := object.NewStaticObjectInfo("unknown.bin", time.Time{}, -1, true, nil, f)
	obj, err := f.upload(context.Background(), bytes.NewBufferString("unknown-size"), src, "unknown.bin", rootID)
	require.NoError(t, err)
	assert.EqualValues(t, len("unknown-size"), preSize)
	assert.Equal(t, "instant-id", obj.id)
	assert.False(t, obj.ModTime(context.Background()).IsZero())
	assert.Zero(t, ossCalls.Load())
}

func TestUploadFallsBackWhenInstantUploadIsNotVisible(t *testing.T) {
	var (
		preCalls    atomic.Int32
		hashCalls   atomic.Int32
		partCalls   atomic.Int32
		fullUpload  atomic.Bool
		hashUpdates []bool
		mu          sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/upload/pre":
			var body uploadPreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			mu.Lock()
			hashUpdates = append(hashUpdates, body.HashUpdate)
			mu.Unlock()
			call := preCalls.Add(1)
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"task_id":"task-%d","bucket":"","obj_key":"object-key","upload_id":"upload-%d","upload_url":%q,"auth_info":{},"callback":{}},"metadata":{"part_size":8}}`, call, call, serverURL(r))
		case "/1/clouddrive/file/update/hash":
			hashCalls.Add(1)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"finish":true,"fid":"invisible-id"}}`)
		case "/1/clouddrive/file/upload/auth":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"auth_key":"oss-auth"}}`)
		case "/object-key":
			if r.Method == http.MethodPut {
				partCalls.Add(1)
				w.Header().Set("ETag", `"etag-1"`)
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = fmt.Fprint(w, `<CompleteMultipartUploadResult/>`)
		case "/1/clouddrive/file/upload/finish":
			fullUpload.Store(true)
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"uploaded-id","size":8,"file_name":"file.bin","pdir_fid":"0","file":true}}`)
		case "/1/clouddrive/file/sort":
			if fullUpload.Load() {
				_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"uploaded-id","pdir_fid":"0","file_name":"file.bin","file":true,"size":8}]}}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldInterval := taskPollInterval
	oldTimeout := taskPollTimeout
	taskPollInterval = 10 * time.Millisecond
	taskPollTimeout = time.Second
	t.Cleanup(func() {
		taskPollInterval = oldInterval
		taskPollTimeout = oldTimeout
	})

	f := newTestFs(t, server.URL)
	src := object.NewStaticObjectInfo("file.bin", time.Now(), 8, true, nil, f)
	obj, err := f.upload(context.Background(), bytes.NewBufferString("fallback"), src, "file.bin", rootID)
	require.NoError(t, err, "pre=%d hash=%d parts=%d full=%v hashUpdates=%v", preCalls.Load(), hashCalls.Load(), partCalls.Load(), fullUpload.Load(), hashUpdates)
	assert.Equal(t, "uploaded-id", obj.id)
	assert.EqualValues(t, 2, preCalls.Load())
	assert.EqualValues(t, 1, hashCalls.Load())
	assert.EqualValues(t, 1, partCalls.Load())
	assert.Equal(t, []bool{true, false}, hashUpdates)
}

func TestUploadAcceptsFinishedMultipartBeforeListingVisibility(t *testing.T) {
	var partCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/upload/pre":
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"task_id":"task-id","bucket":"","obj_key":"object-key","upload_id":"upload-id","upload_url":%q,"auth_info":{},"callback":{}},"metadata":{"part_size":8}}`, serverURL(r))
		case "/1/clouddrive/file/update/hash":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"finish":false}}`)
		case "/1/clouddrive/file/upload/auth":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"auth_key":"oss-auth"}}`)
		case "/object-key":
			if r.Method == http.MethodPut {
				partCalls.Add(1)
				w.Header().Set("ETag", `"etag-1"`)
				w.WriteHeader(http.StatusOK)
				return
			}
			_, _ = fmt.Fprint(w, `<CompleteMultipartUploadResult/>`)
		case "/1/clouddrive/file/upload/finish":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"fid":"uploaded-id","size":8,"file_name":"file.bin","pdir_fid":"0","file":true}}`)
		case "/1/clouddrive/file/sort":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldInterval := taskPollInterval
	oldTimeout := taskPollTimeout
	taskPollInterval = 10 * time.Millisecond
	taskPollTimeout = 100 * time.Millisecond
	t.Cleanup(func() {
		taskPollInterval = oldInterval
		taskPollTimeout = oldTimeout
	})

	f := newTestFs(t, server.URL)
	src := object.NewStaticObjectInfo("file.bin", time.Now(), 8, true, nil, f)
	obj, err := f.upload(context.Background(), bytes.NewBufferString("finished"), src, "file.bin", rootID)
	require.NoError(t, err)
	assert.Equal(t, "uploaded-id", obj.id)
	assert.EqualValues(t, 1, partCalls.Load())
}

func TestUploadZeroByte(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/upload/pre":
			var body uploadPreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Zero(t, body.Size)
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"task_id":"task-zero","bucket":"","obj_key":"empty","upload_id":"upload-zero","upload_url":%q,"auth_info":{},"callback":{}},"metadata":{"part_size":8}}`, serverURL(r))
		case "/1/clouddrive/file/update/hash":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"finish":true,"fid":"empty-id"}}`)
		case "/1/clouddrive/file/sort":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"list":[{"fid":"empty-id","pdir_fid":"0","file_name":"empty","file":true,"size":0}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	src := object.NewStaticObjectInfo("empty", time.Now(), 0, true, nil, f)
	obj, err := f.upload(context.Background(), bytes.NewReader(nil), src, "empty", rootID)
	require.NoError(t, err)
	assert.Equal(t, "empty-id", obj.id)
}

func TestUpdateUsesTemporaryNameThenReplaces(t *testing.T) {
	var events []string
	var mu sync.Mutex
	names := map[string]string{"old-id": "target.txt"}
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/upload/pre":
			var body uploadPreRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Contains(t, body.FileName, ".rclone-upload-")
			mu.Lock()
			names["new-id"] = body.FileName
			mu.Unlock()
			record("upload")
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"task_id":"task-id","bucket":"","obj_key":"object-key","upload_id":"upload-id","upload_url":%q,"auth_info":{},"callback":{}},"metadata":{"part_size":8}}`, serverURL(r))
		case "/1/clouddrive/file/update/hash":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"finish":true,"fid":"new-id"}}`)
		case "/1/clouddrive/file/delete":
			var body deleteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, []string{"old-id"}, body.FileList)
			mu.Lock()
			delete(names, "old-id")
			mu.Unlock()
			record("delete-old")
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{}}`)
		case "/1/clouddrive/file/rename":
			var body renameRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			switch body.ID {
			case "old-id":
				assert.Contains(t, body.FileName, ".rclone-old-")
				record("rename-old")
			case "new-id":
				assert.Equal(t, "target.txt", body.FileName)
				record("rename-new")
			default:
				t.Fatalf("unexpected rename ID %q", body.ID)
			}
			mu.Lock()
			names[body.ID] = body.FileName
			mu.Unlock()
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":{"fid":%q}}`, body.ID)
		case "/1/clouddrive/file/sort":
			mu.Lock()
			items := make([]map[string]any, 0, len(names))
			for id, name := range names {
				items = append(items, map[string]any{"fid": id, "pdir_fid": rootID, "file_name": name, "file": true, "size": 3})
			}
			mu.Unlock()
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": map[string]any{"list": items}}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	obj := &Object{fs: f, remote: "target.txt", id: "old-id", parentID: rootID, size: 3}
	src := object.NewStaticObjectInfo("target.txt", time.Now(), 3, true, nil, f)
	require.NoError(t, obj.Update(context.Background(), bytes.NewBufferString("new"), src))
	assert.Equal(t, []string{"upload", "rename-old", "rename-new", "delete-old"}, events)
	assert.Equal(t, "new-id", obj.id)
}

func TestDownloadURLSyncAndRange(t *testing.T) {
	contentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=2-4", r.Header.Get("Range"))
		assert.Empty(t, r.Header.Get("Cookie"))
		w.Header().Set("Content-Range", "bytes 2-4/5")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = fmt.Fprint(w, "llo")
	}))
	defer contentServer.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1/clouddrive/file/download":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"status":200,"code":0,"data":[{"fid":"file-id","download_url":%q}]}`, contentServer.URL)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := newTestFs(t, server.URL)
	obj := &Object{fs: f, remote: "hello.txt", id: "file-id", parentID: rootID, size: 5}
	rc, err := obj.Open(context.Background(), &fs.RangeOption{Start: 2, End: 1000})
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "llo", string(got))
}

func TestQuarkDownloadCookieHostScope(t *testing.T) {
	assert.True(t, isQuarkHost("https://dl-pc-zb.pds.quark.cn/file"))
	assert.True(t, isQuarkHost("https://quark.cn/file"))
	assert.False(t, isQuarkHost("https://quark.cn.example.com/file"))
	assert.False(t, isQuarkHost("https://downloads.example.com/file"))
}

func TestDownloadURLAsync(t *testing.T) {
	var taskCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/download":
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"task_id":"download-task","task_sync":false}}`)
		case "/1/clouddrive/task":
			assert.Equal(t, "download-task", r.URL.Query().Get("task_id"))
			if taskCalls.Add(1) == 1 {
				_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"status":1}}`)
				return
			}
			_, _ = fmt.Fprint(w, `{"status":200,"code":0,"data":{"status":2,"download_url":"https://download.example/file"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldInterval := taskPollInterval
	oldTimeout := taskPollTimeout
	taskPollInterval = time.Millisecond
	taskPollTimeout = time.Second
	t.Cleanup(func() {
		taskPollInterval = oldInterval
		taskPollTimeout = oldTimeout
	})

	f := newTestFs(t, server.URL)
	got, err := f.getDownloadURL(context.Background(), "file-id")
	require.NoError(t, err)
	assert.Equal(t, "https://download.example/file", got)
	assert.EqualValues(t, 2, taskCalls.Load())
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestCompleteMultipartXMLDoesNotEscapeQuotes(t *testing.T) {
	got := completeMultipartXML([]uploadedPart{{Number: 1, ETag: `"abc"`}})
	assert.True(t, strings.Contains(got, `<ETag>"abc"</ETag>`), got)
}
