package mailru

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/mailru/api"
	"github.com/rclone/rclone/backend/mailru/mrhash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// TestUploadRetry checks that an upload from a seekable pooled buffer
// whose first attempt fails with a retriable error is retried with the
// full body rather than the drained reader.
func TestUploadRetry(t *testing.T) {
	content := bytes.Repeat([]byte("mailru"), 512)
	wantHash := mrhash.Sum(content)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if attempts.Add(1) == 1 {
			// Drop the connection so the client sees a retriable error
			conn, _, err := w.(http.Hijacker).Hijack()
			require.NoError(t, err)
			_ = conn.Close()
			return
		}
		assert.Equal(t, content, body, "retried upload should re-send the full body")
		_, _ = w.Write([]byte(hex.EncodeToString(wantHash) + "\n"))
	}))
	defer server.Close()

	ctx := context.Background()
	f := &Fs{
		srv:         rest.NewClient(server.Client()),
		source:      oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
		shardURL:    server.URL,
		shardExpiry: time.Now().Add(time.Hour),
	}
	o := &Object{fs: f, remote: "remote"}

	rw := multipart.NewRW()
	defer func() { _ = rw.Close() }()
	_, err := rw.Write(content)
	require.NoError(t, err)

	gotHash, err := o.upload(ctx, rw, int64(len(content)))
	require.NoError(t, err)
	assert.Equal(t, wantHash, gotHash)
	assert.Equal(t, int32(2), attempts.Load(), "expected exactly one failed attempt and one retry")
}

// TestListM1Paging checks that a directory bigger than the server's page
// size is listed completely.
func TestListM1Paging(t *testing.T) {
	const total, pageSize = 5, 2
	var offsets []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/m1/folder", r.URL.Path)
		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		require.NoError(t, err)
		offsets = append(offsets, r.URL.Query().Get("offset"))
		var info api.FolderInfoResponse
		info.Body.Kind = "folder"
		info.Body.Count.Files = total
		// The server caps each page whatever limit is requested
		for i := offset; i < total && i < offset+pageSize; i++ {
			info.Body.List = append(info.Body.List, api.ListItem{
				Kind: "file",
				Home: fmt.Sprintf("/dir/file%d", i),
				Hash: strings.Repeat("00", mrhash.Size),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(&info))
	}))
	defer server.Close()

	ctx := context.Background()
	f := &Fs{
		srv:    rest.NewClient(server.Client()).SetRoot(server.URL),
		source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}),
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}

	entries, err := f.listM1(ctx, "/dir", 0, maxInt32)
	require.NoError(t, err)
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Remote())
	}
	assert.Equal(t, []string{"dir/file0", "dir/file1", "dir/file2", "dir/file3", "dir/file4"}, names)
	assert.Equal(t, []string{"0", "2", "4"}, offsets)
}
