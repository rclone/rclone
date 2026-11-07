package ulozto

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/backend/ulozto/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authTestHeaders is a deliberately literal copy of authHeaders so
// that a header dropped from the production list fails
// TestAuthHeaders rather than silently losing test coverage.
var authTestHeaders = map[string]string{
	"X-Auth-Token": "secret-app-token",
	"X-User-Token": "secret-user-token",
}

func TestAuthHeaders(t *testing.T) {
	assert.ElementsMatch(t, []string{"X-Auth-Token", "X-User-Token"}, authHeaders)
}

// newTestFs makes an Fs talking to apiURL with the auth headers set,
// without going through NewFs which would authenticate.
func newTestFs(ctx context.Context, apiURL string) *Fs {
	client := newClient(ctx)
	f := &Fs{
		name:  "ulozto-test",
		cdn:   rest.NewClient(client),
		rest:  rest.NewClient(client).SetRoot(apiURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.rest.SetErrorHandler(errorHandler)
	for header, value := range authTestHeaders {
		f.rest.SetHeader(header, value)
	}
	return f
}

// TestOpenStripsAuthHeadersOnCrossHostRedirect checks that a download
// sends the auth headers to the host the API named but not to a host
// that download redirects to.
func TestOpenStripsAuthHeadersOnCrossHostRedirect(t *testing.T) {
	ctx := context.Background()

	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
		for header := range authTestHeaders {
			assert.Empty(t, r.Header.Get(header), "%s should have been stripped", header)
		}
		_, _ = w.Write([]byte("payload"))
	}))
	defer other.Close()

	var cdnHits atomic.Int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cdnHits.Add(1)
		assert.Equal(t, "/file?hash=h", r.URL.RequestURI())
		for header, value := range authTestHeaders {
			assert.Equal(t, value, r.Header.Get(header), "%s should be sent to the download host", header)
		}
		http.Redirect(w, r, other.URL+"/payload", http.StatusTemporaryRedirect)
	}))
	defer cdn.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v5/file/download-link/vipdata", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(api.GetDownloadLinkResponse{
			Link: cdn.URL + "/file",
			Hash: "h",
		}))
	}))
	defer apiServer.Close()

	o := &Object{fs: newTestFs(ctx, apiServer.URL), slug: "slug"}
	rc, err := o.Open(ctx)
	require.NoError(t, err)
	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "payload", string(body))
	assert.Equal(t, int32(1), cdnHits.Load())
	assert.Equal(t, int32(1), otherHits.Load())
}
