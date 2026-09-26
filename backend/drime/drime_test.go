// Drime filesystem interface
package drime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/require"
)

func TestShouldUsePresignedUpload(t *testing.T) {
	const fiveMiB = int64(5 * fs.Mebi)
	for _, test := range []struct {
		name    string
		enabled bool
		cutoff  int64
		size    int64
		want    bool
	}{
		{name: "disabled", cutoff: int64(10 * fs.Mebi), size: 1, want: false},
		{name: "unknown size", enabled: true, cutoff: int64(10 * fs.Mebi), size: -1, want: false},
		{name: "below limits", enabled: true, cutoff: int64(10 * fs.Mebi), size: fiveMiB - 1, want: true},
		{name: "at presign limit", enabled: true, cutoff: int64(10 * fs.Mebi), size: fiveMiB, want: false},
		{name: "at cutoff", enabled: true, cutoff: int64(2 * fs.Mebi), size: int64(2 * fs.Mebi), want: true},
		{name: "above cutoff", enabled: true, cutoff: int64(2 * fs.Mebi), size: int64(2*fs.Mebi + 1), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := Fs{opt: Options{
				UsePresignedUploads: test.enabled,
				UploadCutoff:        fs.SizeSuffix(test.cutoff),
			}}
			require.Equal(t, test.want, f.shouldUsePresignedUpload(test.size))
		})
	}
}

func TestUploadPresigned(t *testing.T) {
	const (
		contents      = "hello"
		authorization = "Bearer secret"
		filename      = "371add24d1b9ee47aa4912e2a5f9f608"
	)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/s3/simple/presign":
			require.Equal(t, authorization, r.Header.Get("Authorization"))
			var request map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, map[string]any{
				"extension":   "bin",
				"filename":    filename,
				"mime":        "text/plain",
				"parentId":    float64(42),
				"size":        float64(len(contents)),
				"workspaceId": float64(0),
			}, request)
			_, err := w.Write([]byte(`{"url":"` + server.URL + `/storage/object","key":"uploads/uuid/uuid","status":"success"}`))
			require.NoError(t, err)
		case "/storage/object":
			require.Equal(t, http.MethodPut, r.Method)
			require.Empty(t, r.Header.Get("Authorization"))
			require.Equal(t, "text/plain", r.Header.Get("Content-Type"))
			require.Equal(t, int64(len(contents)), r.ContentLength)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, contents, string(body))
		case "/s3/entries":
			require.Equal(t, authorization, r.Header.Get("Authorization"))
			var request map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, map[string]any{
				"clientExtension": "bin",
				"clientMime":      "text/plain",
				"clientName":      filename,
				"filename":        "uuid",
				"parentId":        float64(42),
				"size":            float64(len(contents)),
				"workspaceId":     float64(0),
			}, request)
			_, err := w.Write([]byte(`{"fileEntry":{"id":123,"name":"` + filename + `","file_size":5,"mime":"text/plain"},"status":"success"}`))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := rest.NewClient(server.Client()).SetRoot(server.URL)
	client.SetHeader("Authorization", authorization)
	f := &Fs{
		opt:   Options{},
		srv:   client,
		pacer: fs.NewPacer(context.Background(), pacer.NewDefault()),
	}
	o := &Object{fs: f, remote: filename}
	src := object.NewStaticObjectInfo(filename, time.Now(), int64(len(contents)), true, nil, nil).WithMimeType("text/plain")
	require.NoError(t, o.uploadPresigned(context.Background(), bytes.NewBufferString(contents), src, filename, "42"))
	require.Equal(t, "123", o.id)
}

func TestCleanUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/file-entries/delete", r.URL.Path)

		var request map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, map[string]any{
			"emptyTrash": true,
			"entryIds":   []any{},
		}, request)

		_, err := w.Write([]byte(`{"status":"success"}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	f := &Fs{
		srv:   rest.NewClient(server.Client()).SetRoot(server.URL),
		pacer: fs.NewPacer(context.Background(), pacer.NewDefault()),
	}
	require.NoError(t, f.CleanUp(context.Background()))
}

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestDrime:",
		NilObject:  (*Object)(nil),
		ChunkedUpload: fstests.ChunkedUploadConfig{
			MinChunkSize: minChunkSize,
		},
	})
}

func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadChunkSize(cs)
}

func (f *Fs) SetUploadCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	return f.setUploadCutoff(cs)
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)
