package swift

import (
	"context"
	"io"
	"maps"
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

func TestInternalMetadataToHeaders(t *testing.T) {
	modTime := time.Date(2002, 2, 3, 4, 5, 6, 0, time.UTC)

	t.Run("UserMetadata", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"potato":      "jersey",
			"Foo_Bar":     "baz",
			"encrypttype": "SM4",
		})
		metadata := options.headers.ObjectMetadata()
		assert.Equal(t, "jersey", metadata["potato"])
		assert.Equal(t, "baz", metadata["foo_bar"])
		assert.Equal(t, "SM4", metadata["encrypttype"])
	})

	t.Run("SystemAndIgnoredMetadata", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"btime":               modTime.Format(time.RFC3339Nano),
			"cache-control":       "no-cache",
			"content-disposition": "inline",
			"content-encoding":    "gzip",
			"content-language":    "en-US",
			"content-type":        "text/plain",
			"mtime":               modTime.Format(time.RFC3339Nano),
			"tier":                "STANDARD",
		})
		assert.Equal(t, "text/plain", options.contentType)
		assert.Equal(t, swift.TimeToFloatString(modTime), options.headers.ObjectMetadata()["mtime"])
		assert.Equal(t, "no-cache", options.headers["Cache-Control"])
		assert.Equal(t, "inline", options.headers["Content-Disposition"])
		// content-encoding is read only as rclone can't read gzip encoded
		// objects back
		assert.NotContains(t, options.headers, "Content-Encoding")
		assert.Equal(t, "en-US", options.headers["Content-Language"])
	})

	t.Run("Invalid", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"bad key": "value",
			"badval":  "line\nbreak",
			"ok":      "value",
		})
		metadata := options.headers.ObjectMetadata()
		assert.NotContains(t, metadata, "bad key")
		assert.NotContains(t, metadata, "badval")
		assert.Equal(t, "value", metadata["ok"])
	})

	t.Run("InvalidSystemMetadata", func(t *testing.T) {
		options := metadataToHeaders(fs.Metadata{
			"cache-control": "bad\nvalue",
			"content-type":  "bad\nvalue",
		})
		assert.NotContains(t, options.headers, "Cache-Control")
		assert.Empty(t, options.contentType)
	})
}

func TestInternalMergeObjectHeaders(t *testing.T) {
	src := swift.Headers{
		"Content-Type":        "text/plain",
		"X-Delete-At":         "123",
		"X-Object-Meta-Foo":   "old",
		"X-Object-Meta-Btime": "old",
	}
	metadata := swift.Headers{
		"Content-Type":      "application/octet-stream",
		"X-Object-Meta-Foo": "new",
	}

	merged := mergeObjectHeaders(src, metadata)
	assert.Equal(t, "application/octet-stream", merged["Content-Type"])
	assert.Equal(t, "123", merged["X-Delete-At"])
	assert.Equal(t, "new", merged["X-Object-Meta-Foo"])
	assert.Equal(t, "old", merged["X-Object-Meta-Btime"])
	assert.Equal(t, "old", src["X-Object-Meta-Foo"])
}

func TestInternalWithoutObjectMetadata(t *testing.T) {
	src := swift.Headers{
		"Content-Type":        "text/plain",
		"X-Delete-At":         "123",
		"X-Object-Meta-Foo":   "bar",
		"X-Object-Meta-Btime": "old",
	}

	filtered := withoutObjectMetadata(src)
	assert.Equal(t, "text/plain", filtered["Content-Type"])
	assert.Equal(t, "123", filtered["X-Delete-At"])
	assert.NotContains(t, filtered, "X-Object-Meta-Foo")
	assert.NotContains(t, filtered, "X-Object-Meta-Btime")
	// the source headers must not be modified
	assert.Equal(t, "bar", src["X-Object-Meta-Foo"])
}

func TestInternalObjectMetadata(t *testing.T) {
	modTime := time.Date(2002, 2, 3, 4, 5, 6, 0, time.UTC)
	o := &Object{
		contentType: "text/plain",
		headers: swift.Headers{
			"Content-Type":      "text/plain",
			"Cache-Control":     "no-cache",
			"X-Object-Meta-Foo": "bar",
		},
	}
	meta := swift.Metadata{}
	meta.SetModTime(modTime)
	maps.Copy(o.headers, meta.ObjectHeaders())

	metadata, err := o.Metadata(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "bar", metadata["foo"])
	assert.Equal(t, "no-cache", metadata["cache-control"])
	assert.Equal(t, "text/plain", metadata["content-type"])
	gotModTime, err := time.Parse(time.RFC3339Nano, metadata["mtime"])
	assert.NoError(t, err)
	assert.WithinDuration(t, modTime, gotModTime, time.Millisecond)
}

func TestInternalSystemMetadataInfo(t *testing.T) {
	// The system metadata keys must all be documented and, apart from the
	// read only ones, recognised by metadataToHeaders.
	options := metadataToHeaders(fs.Metadata{
		"cache-control":       "no-cache",
		"content-disposition": "inline",
		"content-encoding":    "gzip",
		"content-language":    "en-US",
		"content-type":        "text/plain",
	})
	assert.Equal(t, "no-cache", options.headers["Cache-Control"])
	assert.Equal(t, "inline", options.headers["Content-Disposition"])
	assert.Equal(t, "en-US", options.headers["Content-Language"])
	assert.Equal(t, "text/plain", options.contentType)
	// content-encoding is read only as rclone can't read gzip encoded
	// objects back
	assert.True(t, systemMetadataInfo["content-encoding"].ReadOnly)
	assert.NotContains(t, options.headers, "Content-Encoding")
	for _, key := range []string{
		"cache-control",
		"content-disposition",
		"content-encoding",
		"content-language",
		"content-type",
		"mtime",
	} {
		assert.Contains(t, systemMetadataInfo, key)
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
