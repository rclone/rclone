// Drime filesystem interface
package drime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/require"
)

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
