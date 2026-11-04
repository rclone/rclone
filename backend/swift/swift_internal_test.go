package swift

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalUrlEncode(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{"0123456789", "0123456789"},
		{"abc/ABC/123", "abc/ABC/123"},
		{"   ", "%20%20%20"},
		{"&", "%26"},
		{"ß£", "%C3%9F%C2%A3"},
		{"Vidéo Potato Sausage?&£.mkv", "Vid%C3%A9o%20Potato%20Sausage%3F%26%C2%A3.mkv"},
	} {
		got := urlEncode(test.in)
		if got != test.want {
			t.Logf("%q: want %q got %q", test.in, test.want, got)
		}
	}
}

func TestInternalShouldRetryHeaders(t *testing.T) {
	ctx := context.Background()
	headers := swift.Headers{
		"Content-Length": "64",
		"Content-Type":   "text/html; charset=UTF-8",
		"Date":           "Mon: 18 Mar 2019 12:11:23 GMT",
		"Retry-After":    "1",
	}
	err := &swift.Error{
		StatusCode: 429,
		Text:       "Too Many Requests",
	}

	// Short sleep should just do the sleep
	start := time.Now()
	retry, gotErr := shouldRetryHeaders(ctx, headers, err)
	dt := time.Since(start)
	assert.True(t, retry)
	assert.Equal(t, err, gotErr)
	assert.True(t, dt > time.Second/2)

	// Long sleep should return RetryError
	headers["Retry-After"] = "3600"
	start = time.Now()
	retry, gotErr = shouldRetryHeaders(ctx, headers, err)
	dt = time.Since(start)
	assert.True(t, dt < time.Second)
	assert.False(t, retry)
	assert.Equal(t, true, fserrors.IsRetryAfterError(gotErr))
	after := gotErr.(fserrors.RetryAfter).RetryAfter()
	dt = after.Sub(start)
	assert.True(t, dt >= time.Hour-time.Second && dt <= time.Hour+time.Second)

}

func TestInternalDeleteSegmentsWithoutBulkDelete(t *testing.T) {
	ctx := context.Background()
	var (
		mu          sync.Mutex
		bulkDeletes int
		deleted     []string
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1.0":
			w.Header().Set("X-Storage-Url", "http://"+r.Host+"/v1/AUTH_test")
			w.Header().Set("X-Auth-Token", "token")
		case r.URL.Query().Get("bulk-delete") == "1":
			mu.Lock()
			bulkDeletes++
			mu.Unlock()
			// Swift returns this when the bulk middleware isn't in the pipeline
			w.WriteHeader(http.StatusForbidden)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/missing"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	f := &Fs{
		c: &swift.Connection{
			UserName: "test",
			ApiKey:   "test",
			AuthUrl:  ts.URL + "/v1.0",
		},
		pacer: fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep))),
	}
	segments := []string{"file/00000000", "file/00000001"}
	want := []string{
		"/v1/AUTH_test/segments/file/00000000",
		"/v1/AUTH_test/segments/file/00000001",
	}
	check := func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 1, bulkDeletes)
		assert.Equal(t, want, deleted)
		bulkDeletes, deleted = 0, nil
	}

	t.Run("RemoveLargeObject", func(t *testing.T) {
		o := &Object{fs: f}
		require.NoError(t, o.removeSegmentsLargeObject(ctx, "segments", segments))
		check(t)
	})

	t.Run("FailedUpload", func(t *testing.T) {
		su := &segmentedUpload{f: f, container: "segments", segments: segments}
		su.onFail()
		check(t)
	})

	t.Run("MissingSegment", func(t *testing.T) {
		o := &Object{fs: f}
		require.NoError(t, o.removeSegmentsLargeObject(ctx, "segments", append(segments, "file/missing")))
		check(t)
	})
}
