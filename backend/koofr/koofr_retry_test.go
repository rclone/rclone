package koofr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListRetriesTooManyRequests checks that 429 responses are retried
// rather than failing the directory listing straight away.
func TestListRetriesTooManyRequests(t *testing.T) {
	ctx := context.Background()
	var listCalls atomic.Int32
	failures := int32(3)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"m1","isPrimary":true}]}`))
		case strings.HasSuffix(r.URL.Path, "/files/info"):
			_, _ = w.Write([]byte(`{"name":"root","type":"dir"}`))
		case strings.HasSuffix(r.URL.Path, "/files/list"):
			if listCalls.Add(1) <= failures {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte("<html><body><h1>429 Too Many Requests</h1></body></html>"))
				return
			}
			_, _ = w.Write([]byte(`{"files":[{"name":"a.txt","type":"file","size":1}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f, err := NewFsFromOptions(ctx, "test", "", &Options{
		Endpoint: srv.URL,
		User:     "user",
		Password: obscure.MustObscure("pass"),
	})
	require.NoError(t, err)

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, failures+1, listCalls.Load())
}
