// Tests for the Databricks Unity Catalog backend.
//
// All tests use an in-process httptest.Server that simulates the Databricks
// Files REST API, so no real Databricks credentials are required.

//go:build !plan9

package databricks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSDKMetadata wraps a handler to return 404 for Databricks SDK internal
// host-metadata probes so they don't produce test noise or failures.
func withSDKMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/databricks-config" {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newTestFs creates an Fs backed by a fake httptest.Server.
// The caller is responsible for calling ts.Close() when done.
func newTestFs(t *testing.T, root string, handler http.Handler) (*Fs, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(withSDKMetadata(handler))

	m := configmap.Simple{
		"host":  ts.URL,
		"token": "test-token",
	}
	f, err := NewFs(context.Background(), "test", root, m)
	if err == fs.ErrorIsFile {
		// root pointed at a file; rclone adjusts root and returns ErrorIsFile.
		// Unwrap the Fs from the returned value.
		f, err = NewFs(context.Background(), "test", root, m)
	}
	require.NoError(t, err)
	return f.(*Fs), ts
}

// ---------------------------------------------------------------------------
// Fake API server helpers
// ---------------------------------------------------------------------------

type dirEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDirectory  bool   `json:"is_directory"`
	FileSize     int64  `json:"file_size,omitempty"`
	LastModified int64  `json:"last_modified,omitempty"`
}

type listDirResponse struct {
	Contents      []dirEntry `json:"contents"`
	NextPageToken string     `json:"next_page_token,omitempty"`
}

// apiError formats a Databricks-style error JSON body.
func apiError(code string, msg string) string {
	return fmt.Sprintf(`{"error_code":%q,"message":%q}`, code, msg)
}

// ---------------------------------------------------------------------------
// fullPath
// ---------------------------------------------------------------------------

func TestFullPath(t *testing.T) {
	f := &Fs{root: "Volumes/cat/sc/vol", opt: Options{Enc: defaultEncoding()}}
	assert.Equal(t, "/Volumes/cat/sc/vol/sub/file.txt", f.fullPath("sub/file.txt"))
	assert.Equal(t, "/Volumes/cat/sc/vol", f.fullPath(""))
}

// ---------------------------------------------------------------------------
// encodePath
// ---------------------------------------------------------------------------

func TestEncodePath(t *testing.T) {
	assert.Equal(t, "/Volumes/cat/sc/vol/file%20name.txt", encodePath("/Volumes/cat/sc/vol/file name.txt"))
	assert.Equal(t, "/a/b/c", encodePath("/a/b/c"))
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList(t *testing.T) {
	ts1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC).UnixMilli()
	ts2 := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC).UnixMilli()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/2.0/fs/directories/") {
			resp := listDirResponse{
				Contents: []dirEntry{
					{Name: "subdir", Path: "/Volumes/c/s/v/subdir", IsDirectory: true, LastModified: ts1},
					{Name: "file.txt", Path: "/Volumes/c/s/v/file.txt", FileSize: 1234, LastModified: ts2},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				panic(err)
			}
			return
		}
		// statFile HEAD for root check
		if r.Method == http.MethodHead {
			http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "Volumes/c/s/v", handler)
	defer ts.Close()

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// directory
	dir, ok := entries[0].(fs.Directory)
	require.True(t, ok)
	assert.Equal(t, "subdir", dir.Remote())
	assert.Equal(t, time.UnixMilli(ts1).UTC(), dir.ModTime(context.Background()).UTC())

	// file
	obj, ok := entries[1].(*Object)
	require.True(t, ok)
	assert.Equal(t, "file.txt", obj.Remote())
	assert.Equal(t, int64(1234), obj.Size())
	assert.Equal(t, time.UnixMilli(ts2).UTC(), obj.ModTime(context.Background()).UTC())
}

func TestListNotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
			return
		}
		http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
	})

	f, ts := newTestFs(t, "Volumes/c/s/v", handler)
	defer ts.Close()

	_, err := f.List(context.Background(), "missing")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

// ---------------------------------------------------------------------------
// NewObject / statFile
// ---------------------------------------------------------------------------

func TestNewObject(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "512")
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Last-Modified", "Mon, 15 Jan 2024 10:00:00 GMT")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	obj, err := f.NewObject(context.Background(), "file.txt")
	require.NoError(t, err)
	assert.Equal(t, "file.txt", obj.Remote())
	assert.Equal(t, int64(512), obj.Size())
	assert.Equal(t, "text/plain", obj.(*Object).contentType)
}

func TestNewObjectNotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	_, err := f.NewObject(context.Background(), "missing.txt")
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

// ---------------------------------------------------------------------------
// Open (download) with and without Range header
// ---------------------------------------------------------------------------

func TestOpen(t *testing.T) {
	content := "hello world"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/plain")
			if _, err := fmt.Fprint(w, content); err != nil {
				panic(err)
			}
			return
		}
		// HEAD for object stat
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	o := &Object{fs: f, remote: "file.txt", size: int64(len(content))}
	rc, err := o.Open(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, content, string(got))
}

func TestOpenWithRange(t *testing.T) {
	content := "hello world"

	var capturedRange string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			capturedRange = r.Header.Get("Range")
			w.Header().Set("Content-Type", "text/plain")
			// Return bytes 6-10 ("world") to simulate partial content
			w.WriteHeader(http.StatusPartialContent)
			if _, err := fmt.Fprint(w, "world"); err != nil {
				panic(err)
			}
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	o := &Object{fs: f, remote: "file.txt", size: int64(len(content))}
	rc, err := o.Open(context.Background(), &fs.RangeOption{Start: 6, End: 10})
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "world", string(got))
	assert.Equal(t, "bytes=6-10", capturedRange)
}

// ---------------------------------------------------------------------------
// Put / Update
// ---------------------------------------------------------------------------

func TestPut(t *testing.T) {
	var mkdirCalled, uploadCalled, headCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/api/2.0/fs/directories/"):
			mkdirCalled = true
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/api/2.0/fs/files/"):
			uploadCalled = true
			assert.Equal(t, "true", r.URL.Query().Get("overwrite"))
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodHead:
			headCalled = true
			w.Header().Set("Content-Length", "11")
			w.Header().Set("Last-Modified", "Mon, 15 Jan 2024 10:00:00 GMT")
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusInternalServerError)
		}
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	content := strings.NewReader("hello world")
	info := object.NewStaticObjectInfo("file.txt", time.Now(), 11, true, nil, nil)
	obj, err := f.Put(context.Background(), content, info)
	require.NoError(t, err)
	assert.True(t, mkdirCalled, "mkdir should have been called")
	assert.True(t, uploadCalled, "upload should have been called")
	assert.True(t, headCalled, "head (stat) should have been called")
	assert.Equal(t, int64(11), obj.Size())
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove(t *testing.T) {
	var deleteCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/2.0/fs/files/") {
			deleteCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}
		// HEAD for initial root check (empty root → skipped)
		if r.Method == http.MethodHead {
			http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	o := &Object{fs: f, remote: "file.txt"}
	err := o.Remove(context.Background())
	require.NoError(t, err)
	assert.True(t, deleteCalled)
}

// ---------------------------------------------------------------------------
// Mkdir
// ---------------------------------------------------------------------------

func TestMkdir(t *testing.T) {
	var called bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/api/2.0/fs/directories/") {
			called = true
			w.WriteHeader(http.StatusOK)
			return
		}
		// HEAD for initial root check (empty root → skipped)
		if r.Method == http.MethodHead {
			http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	err := f.Mkdir(context.Background(), "newdir")
	require.NoError(t, err)
	assert.True(t, called)
}

// ---------------------------------------------------------------------------
// Rmdir
// ---------------------------------------------------------------------------

func TestRmdir(t *testing.T) {
	var called bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/2.0/fs/directories/") {
			called = true
			w.WriteHeader(http.StatusOK)
			return
		}
		// HEAD for initial root check
		if r.Method == http.MethodHead {
			http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	err := f.Rmdir(context.Background(), "emptydir")
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRmdirNotEmpty(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/2.0/fs/directories/") {
			http.Error(w, apiError("RESOURCE_IS_NOT_EMPTY", "directory not empty"), http.StatusConflict)
			return
		}
		if r.Method == http.MethodHead {
			http.Error(w, apiError("RESOURCE_DOES_NOT_EXIST", "not found"), http.StatusNotFound)
			return
		}
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})

	f, ts := newTestFs(t, "", handler)
	defer ts.Close()

	err := f.Rmdir(context.Background(), "nonempty")
	assert.Equal(t, fs.ErrorDirectoryNotEmpty, err)
}

// ---------------------------------------------------------------------------
// Fs metadata
// ---------------------------------------------------------------------------

func TestFsName(t *testing.T) {
	f := &Fs{name: "mydb", root: "Volumes/c/s/v", opt: Options{Host: "https://host.azuredatabricks.net"}}
	assert.Equal(t, "mydb", f.Name())
	assert.Equal(t, "Volumes/c/s/v", f.Root())
	assert.Contains(t, f.String(), "host.azuredatabricks.net")
}

func TestFsPrecision(t *testing.T) {
	f := &Fs{}
	assert.Equal(t, fs.ModTimeNotSupported, f.Precision())
}

func TestFsHashes(t *testing.T) {
	f := &Fs{}
	assert.Equal(t, hash.Set(0), f.Hashes())
}

// ---------------------------------------------------------------------------
// Object metadata
// ---------------------------------------------------------------------------

func TestObjectString(t *testing.T) {
	o := &Object{remote: "path/to/file.txt"}
	assert.Equal(t, "path/to/file.txt", o.String())
	var nilObj *Object
	assert.Equal(t, "<nil>", nilObj.String())
}

func TestObjectSetModTime(t *testing.T) {
	f := &Fs{}
	o := &Object{fs: f, remote: "file.txt"}
	err := o.SetModTime(context.Background(), time.Now())
	assert.Equal(t, fs.ErrorCantSetModTime, err)
}

func TestObjectHash(t *testing.T) {
	o := &Object{}
	_, err := o.Hash(context.Background(), hash.MD5)
	assert.Equal(t, hash.ErrUnsupported, err)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func defaultEncoding() encoder.MultiEncoder {
	return encoder.EncodeInvalidUtf8 | encoder.EncodeCtl | encoder.EncodeSlash
}

// ---------------------------------------------------------------------------
// NewFs constructor edge-case tests
// ---------------------------------------------------------------------------

func TestNewFsRootIsFile(t *testing.T) {
	// When the root path points at a file, NewFs should return ErrorIsFile
	// and adjust the root to the parent directory.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GetMetadataByFilePath sends HEAD /api/2.0/fs/files/{path}
		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/api/2.0/fs/files/") {
			w.Header().Set("Content-Length", "42")
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Last-Modified", "Mon, 01 Jan 2024 00:00:00 GMT")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	})

	ts := httptest.NewServer(withSDKMetadata(handler))
	defer ts.Close()

	m := configmap.Simple{
		"host":  ts.URL,
		"token": "test-token",
	}
	result, err := NewFs(context.Background(), "test", "Volumes/cat/sch/vol/file.txt", m)
	assert.Equal(t, fs.ErrorIsFile, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Volumes/cat/sch/vol", result.Root())
}

func TestNewFsAuthError(t *testing.T) {
	// When the root probe returns a non-404 error (e.g. 403), NewFs should
	// propagate that error instead of silently swallowing it.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && strings.Contains(r.URL.Path, "/api/2.0/fs/files/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	})

	ts := httptest.NewServer(withSDKMetadata(handler))
	defer ts.Close()

	m := configmap.Simple{
		"host":  ts.URL,
		"token": "bad-token",
	}
	_, err := NewFs(context.Background(), "test", "Volumes/cat/sch/vol", m)
	assert.Error(t, err)
	assert.NotEqual(t, fs.ErrorIsFile, err)
}
