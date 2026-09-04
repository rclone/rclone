package webdav_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/backend/webdav"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	netwebdav "golang.org/x/net/webdav"
)

var (
	remoteName = "TestWebDAV"
	headers    = []string{"X-Potato", "sausage", "X-Rhubarb", "cucumber"}
)

// prepareServer the test server and return a function to tidy it up afterwards
// with each request the headers option tests are executed
func prepareServer(t *testing.T) (configmap.Simple, func()) {
	// test the headers are there send send a dummy response to About
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		what := fmt.Sprintf("%s %s: Header ", r.Method, r.URL.Path)
		assert.Equal(t, headers[1], r.Header.Get(headers[0]), what+headers[0])
		assert.Equal(t, headers[3], r.Header.Get(headers[2]), what+headers[2])
		_, err := fmt.Fprintf(w, `<d:multistatus xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns" xmlns:oc="http://owncloud.org/ns" xmlns:nc="http://nextcloud.org/ns">
<d:response>
 <d:href>/remote.php/webdav/</d:href>
 <d:propstat>
  <d:prop>
   <d:quota-available-bytes>-3</d:quota-available-bytes>
   <d:quota-used-bytes>376461895</d:quota-used-bytes>
  </d:prop>
  <d:status>HTTP/1.1 200 OK</d:status>
 </d:propstat>
</d:response>
</d:multistatus>`)
		require.NoError(t, err)
	})
	// Make the test server
	ts := httptest.NewServer(handler)

	// Configure the remote
	configfile.Install()

	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		// add headers to test the headers option
		"headers": strings.Join(headers, ","),
	}

	// return a function to tidy up
	return m, ts.Close
}

// prepare the test server and return a function to tidy it up afterwards
func prepare(t *testing.T) (fs.Fs, func()) {
	m, tidy := prepareServer(t)

	// Instantiate the WebDAV server
	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	return f, tidy
}

// TestHeaders any request will test the headers option
func TestHeaders(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	// send an About response since that is all the dummy server can return
	_, err := f.Features().About(context.Background())
	require.NoError(t, err)
}

// TestListAllAuthRedirect checks auth_redirect is honoured on listAll PROPFIND.
func TestListAllAuthRedirect(t *testing.T) {
	var targetAuth string
	var targetHits int

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits++
		targetAuth = r.Header.Get("Authorization")
		_, err := fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
		require.NoError(t, err)
	}))
	defer target.Close()

	// Redirect via a different hostname so net/http strips Authorization on cross-host redirect.
	targetURL := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetURL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	configfile.Install()
	m := configmap.Simple{
		"type":          "webdav",
		"url":           source.URL,
		"user":          "alice",
		"pass":          obscure.MustObscure("secret"),
		"auth_redirect": "true",
	}

	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	_, _ = f.List(context.Background(), "")

	assert.GreaterOrEqual(t, targetHits, 1, "redirect target should receive the request")
	assert.NotEmpty(t, targetAuth, "Authorization header should be preserved across redirect")
}

// TestReservedCharactersInPathAreEscaped verifies that reserved characters
// like semicolons and equals signs in file paths are percent-encoded in
// HTTP requests to the WebDAV server (RFC 3986 compliance).
func TestReservedCharactersInPathAreEscaped(t *testing.T) {
	var capturedPath string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.RequestURI
		// Return a 404 so the NewObject call fails cleanly
		w.WriteHeader(http.StatusNotFound)
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	}

	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	// Try to access a file with a semicolon in the name.
	// We expect the request to fail (404), but the path should be escaped.
	_, _ = f.NewObject(context.Background(), "my;test")

	// The semicolon must be percent-encoded as %3B
	assert.Contains(t, capturedPath, "my%3Btest", "semicolons in path should be percent-encoded")
	assert.NotContains(t, capturedPath, "my;test", "raw semicolons should not appear in path")
}

const fileInfoResponse = `<d:multistatus xmlns:d="DAV:">
<d:response>
 <d:href>/file.txt</d:href>
 <d:propstat>
  <d:prop>
   <d:getcontentlength>10</d:getcontentlength>
   <d:resourcetype/>
  </d:prop>
  <d:status>HTTP/1.1 200 OK</d:status>
 </d:propstat>
</d:response>
</d:multistatus>`

func prepareFileObject(ctx context.Context, t *testing.T, getHandler http.HandlerFunc) (fs.Object, func()) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(http.StatusMultiStatus)
			_, err := fmt.Fprint(w, fileInfoResponse)
			require.NoError(t, err)
			return
		}
		if r.Method == http.MethodGet {
			getHandler(w, r)
			return
		}
		http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
	})
	ts := httptest.NewServer(handler)

	configfile.Install()
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	})
	require.NoError(t, err)
	o, err := f.NewObject(ctx, "file.txt")
	require.NoError(t, err)
	return o, ts.Close
}

func TestOpenDoesNotRetryIgnoredRange(t *testing.T) {
	var getRequests atomic.Int32
	o, tidy := prepareFileObject(context.Background(), t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=2-4", r.Header.Get("Range"))
		getRequests.Add(1)
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, "abcdefghij")
		require.NoError(t, err)
	})
	defer tidy()

	in, err := o.Open(context.Background(), &fs.RangeOption{Start: 2, End: 4})
	assert.Nil(t, in)
	assert.ErrorIs(t, err, fs.ErrorRangeIgnored)
	assert.Equal(t, int32(1), getRequests.Load())
}

func TestOpenRetriesMismatchedContentRange(t *testing.T) {
	var getRequests atomic.Int32
	o, tidy := prepareFileObject(context.Background(), t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "bytes=2-4", r.Header.Get("Range"))
		if getRequests.Add(1) == 1 {
			w.Header().Set("Content-Length", "3")
			w.Header().Set("Content-Range", "bytes 0-2/10")
			w.WriteHeader(http.StatusPartialContent)
			_, err := io.WriteString(w, "abc")
			require.NoError(t, err)
			return
		}
		w.Header().Set("Content-Length", "3")
		w.Header().Set("Content-Range", "bytes 2-4/10")
		w.WriteHeader(http.StatusPartialContent)
		_, err := io.WriteString(w, "cde")
		require.NoError(t, err)
	})
	defer tidy()

	in, err := o.Open(context.Background(), &fs.RangeOption{Start: 2, End: 4})
	require.NoError(t, err)
	defer func() { require.NoError(t, in.Close()) }()
	contents, err := io.ReadAll(in)
	require.NoError(t, err)
	assert.Equal(t, "cde", string(contents))
	assert.Equal(t, int32(2), getRequests.Load())
}

func TestCopyFallsBackWhenRangeIgnored(t *testing.T) {
	var rangeRequests atomic.Int32
	var fullRequests atomic.Int32
	ctx, ci := fs.AddConfig(context.Background())
	ci.LowLevelRetries = 2
	ci.MultiThreadStreams = 2
	ci.MultiThreadSet = true
	ci.MultiThreadCutoff = 1
	ci.MultiThreadChunkSize = 4

	src, tidy := prepareFileObject(ctx, t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "" {
			fullRequests.Add(1)
		} else {
			rangeRequests.Add(1)
		}
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, "abcdefghij")
		require.NoError(t, err)
	})
	defer tidy()

	dstFs, err := local.NewFs(ctx, "local", t.TempDir(), configmap.Simple{
		"no_preallocate": "true",
		"no_sparse":      "true",
	})
	require.NoError(t, err)
	dst, err := operations.Copy(ctx, dstFs, nil, "file.txt", src)
	require.NoError(t, err)

	in, err := dst.Open(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, in.Close()) }()
	contents, err := io.ReadAll(in)
	require.NoError(t, err)
	assert.Equal(t, "abcdefghij", string(contents))
	assert.Positive(t, rangeRequests.Load())
	assert.LessOrEqual(t, rangeRequests.Load(), int32(3))
	assert.Equal(t, int32(1), fullRequests.Load())
}

const (
	digestUser  = "user"
	digestPass  = "pass"
	digestRealm = "rclone"
)

// digestServer runs a WebDAV server on dir which only accepts digest
// authentication
func digestServer(t *testing.T, dir string) *httptest.Server {
	authenticator := auth.NewDigestAuthenticator(digestRealm, func(user, realm string) string {
		if user == digestUser {
			return digestPass
		}
		return ""
	})
	authenticator.PlainTextSecrets = true
	handler := &netwebdav.Handler{
		FileSystem: netwebdav.Dir(dir),
		LockSystem: netwebdav.NewMemLS(),
	}
	ts := httptest.NewServer(authenticator.Wrap(func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		handler.ServeHTTP(w, &r.Request)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// newDigestFs makes an Fs pointing at ts authenticating with pass
func newDigestFs(t *testing.T, ts *httptest.Server, pass string) (fs.Fs, error) {
	configfile.Install()
	return webdav.NewFs(context.Background(), remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": digestUser,
		"pass": obscure.MustObscure(pass),
	})
}

// TestDigestAuth checks that a server which only accepts digest
// authentication can be listed
func TestDigestAuth(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0600))

	f, err := newDigestFs(t, digestServer(t, dir), digestPass)
	require.NoError(t, err)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "file.txt", entries[0].Remote())
	assert.Equal(t, int64(5), entries[0].Size())
}

// TestDigestAuthUpload checks that a file can be uploaded to a server which
// only accepts digest authentication.
//
// The signature covers the request URI but not the body, so the body is
// streamed rather than held in memory to be sent a second time.
func TestDigestAuthUpload(t *testing.T) {
	dir := t.TempDir()
	f, err := newDigestFs(t, digestServer(t, dir), digestPass)
	require.NoError(t, err)

	contents := "hello digest"
	_, err = operations.Rcat(context.Background(), f, "uploaded.txt", io.NopCloser(strings.NewReader(contents)), time.Now(), nil)
	require.NoError(t, err)

	written, err := os.ReadFile(filepath.Join(dir, "uploaded.txt"))
	require.NoError(t, err)
	assert.Equal(t, contents, string(written))
}

// TestDigestAuthWrongPassword checks that bad credentials fail rather than
// being retried against the server for ever
func TestDigestAuthWrongPassword(t *testing.T) {
	f, err := newDigestFs(t, digestServer(t, t.TempDir()), "wrong")
	require.NoError(t, err)

	_, err = f.List(context.Background(), "")
	require.Error(t, err)
}

// TestDigestAuthStaleNonce checks that a request signed with an expired nonce
// is signed again with the replacement the server sends
func TestDigestAuthStaleNonce(t *testing.T) {
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		switch n := requests.Add(1); {
		case n == 1:
			// Rclone sends basic authentication until it knows better
			assert.False(t, strings.HasPrefix(authorization, "Digest"), "the first request can't be signed yet")
			w.Header().Set("WWW-Authenticate", `Digest realm="rclone", nonce="nonce-1", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
		case n == 2:
			assert.Contains(t, authorization, `nonce="nonce-1"`)
			// The nonce has expired, so ask for a signature made with a new one
			w.Header().Set("WWW-Authenticate", `Digest realm="rclone", nonce="nonce-2", algorithm=MD5, qop="auth", stale=true`)
			w.WriteHeader(http.StatusUnauthorized)
		default:
			assert.Contains(t, authorization, `nonce="nonce-2"`, "the stale nonce should have been replaced")
			_, err := fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
			require.NoError(t, err)
		}
	}))
	defer ts.Close()

	f, err := newDigestFs(t, ts, digestPass)
	require.NoError(t, err)

	_, err = f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, int32(3), requests.Load())
}

// TestBasicAuthNotRetried checks that a 401 from a server which doesn't offer
// digest authentication is reported straight away
func TestBasicAuthNotRetried(t *testing.T) {
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="rclone"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	f, err := newDigestFs(t, ts, digestPass)
	require.NoError(t, err)

	_, err = f.List(context.Background(), "")
	require.Error(t, err)
	assert.Equal(t, int32(1), requests.Load(), "a 401 without a digest challenge shouldn't be retried")
}

// TestDigestAuthRedirectToOtherHost checks that a redirect somewhere else is
// sent unsigned, so the credentials only reach the host which asked for them
func TestDigestAuthRedirectToOtherHost(t *testing.T) {
	var otherAuth atomic.Value
	otherAuth.Store("")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherAuth.Store(r.Header.Get("Authorization"))
		_, err := fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
		require.NoError(t, err)
	}))
	defer other.Close()

	var originAuth atomic.Value
	originAuth.Store("")
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("WWW-Authenticate", `Digest realm="rclone", nonce="nonce-1", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		originAuth.Store(r.Header.Get("Authorization"))
		http.Redirect(w, r, other.URL+"/", http.StatusFound)
	}))
	defer ts.Close()

	f, err := newDigestFs(t, ts, digestPass)
	require.NoError(t, err)

	_, err = f.List(context.Background(), "")
	require.NoError(t, err)

	assert.Contains(t, originAuth.Load(), "Digest ", "the challenging host should be signed")
	assert.NotContains(t, otherAuth.Load(), "Digest ", "another host shouldn't be sent the credentials")
}
