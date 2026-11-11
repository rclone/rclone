package swift

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
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

// testToken is the auth token the redirect tests look for. It is
// distinctive so that it can't match anything else in a request.
const testToken = "AUTH_tk_secret_4242"

// authTestHeaders is a deliberately literal copy of authHeaders so
// that a header dropped from the production list shows up as a leak
// in the cross host tests rather than silently losing coverage.
var authTestHeaders = []string{"X-Auth-Token", "X-Auth-Key", "X-Auth-User"}

// listHandler answers any request with an empty JSON listing.
func listHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("[]"))
}

// newTestFs makes an Fs via NewFs for a container on storageURL,
// using a pre-issued token so that no auth request is made.
func newTestFs(ctx context.Context, t *testing.T, storageURL string) *Fs {
	m := configmap.Simple{
		"storage_url": storageURL + "/v1/AUTH_test",
		"auth_token":  testToken,
		"chunk_size":  "5Gi",
	}
	f, err := NewFs(ctx, "swift-test", "container", m)
	require.NoError(t, err)
	return f.(*Fs)
}

// assertAuthHeaders checks that every auth header is absent from r
// (!want) or that the ones named in present are there (want).
func assertAuthHeaders(t *testing.T, r *http.Request, want bool, present ...string) {
	if want {
		for _, header := range present {
			assert.NotEmpty(t, r.Header.Get(header), "%s should be sent", header)
		}
		return
	}
	for _, header := range authTestHeaders {
		assert.Empty(t, r.Header.Get(header), "%s should have been stripped", header)
	}
}

// TestRedirectStripsAuthTokenOnCrossHost checks the token is sent to
// the storage host but not to another host it redirects to, nor to
// the storage host again once the chain has been elsewhere.
func TestRedirectStripsAuthTokenOnCrossHost(t *testing.T) {
	var originHits, otherHits, backHits atomic.Int32
	var origin *httptest.Server
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
		assertAuthHeaders(t, r, false)
		http.Redirect(w, r, origin.URL+"/back", http.StatusFound)
	}))
	defer other.Close()

	origin = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/back" {
			backHits.Add(1)
			assertAuthHeaders(t, r, false)
			listHandler(w, r)
			return
		}
		originHits.Add(1)
		assertAuthHeaders(t, r, true, "X-Auth-Token")
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	ctx := context.Background()
	f := newTestFs(ctx, t, origin.URL)
	_, err := f.List(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), originHits.Load())
	assert.Equal(t, int32(1), otherHits.Load())
	assert.Equal(t, int32(1), backHits.Load())
}

// TestRedirectKeepsAuthTokenOnSameHost checks the token is still sent
// after a redirect which stays on the storage host.
func TestRedirectKeepsAuthTokenOnSameHost(t *testing.T) {
	var hits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		assertAuthHeaders(t, r, true, "X-Auth-Token")
		if !strings.HasSuffix(r.URL.Path, "/moved") {
			http.Redirect(w, r, r.URL.Path+"/moved", http.StatusTemporaryRedirect)
			return
		}
		listHandler(w, r)
	}))
	defer origin.Close()

	ctx := context.Background()
	f := newTestFs(ctx, t, origin.URL)
	_, err := f.List(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, int32(2), hits.Load())
}

// TestRedirectRefusesHTTPSDowngrade checks a redirect from HTTPS to
// plaintext HTTP is not followed, so the token never goes over the
// network in the clear.
func TestRedirectRefusesHTTPSDowngrade(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		listHandler(w, r)
	}))
	defer target.Close()

	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	// The transport is a process wide singleton so it has to be
	// rebuilt to trust the httptest certificate, and rebuilt again
	// afterwards so later tests don't inherit that.
	fshttp.ResetTransport()
	t.Cleanup(fshttp.ResetTransport)
	ctx, ci := fs.AddConfig(context.Background())
	ci.InsecureSkipVerify = true
	f := newTestFs(ctx, t, origin.URL)
	_, err := f.List(ctx, "")
	require.ErrorIs(t, err, rest.ErrHTTPSDowngrade)
	assert.Equal(t, int32(0), targetHits.Load())
}

// TestAuthRedirectStripsCredentialsOnCrossHost checks the user name
// and API key sent on a v1 auth request go to the auth host but not
// to another host it redirects to.
func TestAuthRedirectStripsCredentialsOnCrossHost(t *testing.T) {
	var originHits, otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
		assertAuthHeaders(t, r, false)
		w.Header().Set("X-Storage-Url", "http://"+r.Host+"/v1/AUTH_test")
		w.Header().Set("X-Auth-Token", testToken)
	}))
	defer other.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originHits.Add(1)
		assertAuthHeaders(t, r, true, "X-Auth-Key", "X-Auth-User")
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusFound)
	}))
	defer origin.Close()

	m := configmap.Simple{
		"auth":         origin.URL + "/v1.0",
		"auth_version": "1",
		"user":         "test-user",
		"key":          "test-key",
		"chunk_size":   "5Gi",
	}
	_, err := NewFs(context.Background(), "swift-test", "container", m)
	require.NoError(t, err)
	assert.Equal(t, int32(1), originHits.Load())
	assert.Equal(t, int32(1), otherHits.Load())
}

// TestAuthRedirectRefusesToResendPassword checks a v2 or v3 auth
// request, which carries the password in its body, is not resent to
// another host the auth server redirects it to.
func TestAuthRedirectRefusesToResendPassword(t *testing.T) {
	for _, authVersion := range []string{"2", "3"} {
		t.Run("v"+authVersion, func(t *testing.T) {
			var originHits, otherHits atomic.Int32
			other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				otherHits.Add(1)
				body, _ := io.ReadAll(r.Body)
				assert.NotContains(t, string(body), "test-key", "password was resent to another host")
			}))
			defer other.Close()

			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				originHits.Add(1)
				body, _ := io.ReadAll(r.Body)
				assert.Contains(t, string(body), "test-key")
				http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
			}))
			defer origin.Close()

			m := configmap.Simple{
				"auth":         origin.URL + "/v" + authVersion,
				"auth_version": authVersion,
				"user":         "test-user",
				"key":          "test-key",
				"chunk_size":   "5Gi",
			}
			_, err := NewFs(context.Background(), "swift-test", "container", m)
			require.ErrorIs(t, err, errRedirectResendsBody)
			assert.Equal(t, int32(1), originHits.Load())
			assert.Equal(t, int32(0), otherHits.Load())
		})
	}
}

// newForbiddenAccountFs makes an Fs for the root of a server which
// forbids listing the account and has one container called container.
func newForbiddenAccountFs(ctx context.Context, t *testing.T, user, container string) *Fs {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/AUTH_test":
			http.Error(w, "<html><h1>Forbidden</h1></html>", http.StatusForbidden)
		case "/v1/AUTH_test/" + container:
			w.Header().Set("X-Container-Bytes-Used", "123")
			w.Header().Set("X-Container-Object-Count", "4")
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	f, err := NewFs(ctx, "swift-test", "", configmap.Simple{
		"user":        user,
		"storage_url": srv.URL + "/v1/AUTH_test",
		"auth_token":  testToken,
		"chunk_size":  "5Gi",
	})
	require.NoError(t, err)
	return f.(*Fs)
}

func TestInternalForbiddenAccountUsesUserContainer(t *testing.T) {
	const user = "user@example.com"
	ctx := context.Background()
	f := newForbiddenAccountFs(ctx, t, user, user)

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, user, entries[0].Remote())
	assert.Equal(t, int64(123), entries[0].Size())

	usage, err := f.About(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(123), *usage.Used)
	assert.Equal(t, int64(4), *usage.Objects)
}

func TestInternalForbiddenAccountWithoutUserContainer(t *testing.T) {
	ctx := context.Background()
	f := newForbiddenAccountFs(ctx, t, "user@example.com", "other")

	_, err := f.List(ctx, "")
	require.ErrorContains(t, err, "Operation forbidden")

	_, err = f.About(ctx)
	require.ErrorContains(t, err, "Operation forbidden")
}
