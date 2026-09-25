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

// TestListAllRetryDoesNotConcatenate verifies that when a listing PROPFIND
// fails after having partially decoded and is retried, the final listing is
// exactly the retried response, with no entries carried over from the
// failed attempt.
func TestListAllRetryDoesNotConcatenate(t *testing.T) {
	entryXML := func(i int) string {
		name := fmt.Sprintf("file-%03d.bin", i)
		return "<d:response><d:href>/" + name + "</d:href><d:propstat><d:prop>" +
			"<d:displayname>" + name + "</d:displayname><d:getcontentlength>1024</d:getcontentlength>" +
			"</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>\n"
	}
	head := `<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:">`
	self := head + `<d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection xmlns:d="DAV:"/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`
	writeEntries := func(w http.ResponseWriter, from, to int) {
		_, err := fmt.Fprint(w, head)
		require.NoError(t, err)
		for i := from; i <= to; i++ {
			_, err := fmt.Fprint(w, entryXML(i))
			require.NoError(t, err)
		}
	}

	var listCalls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PROPFIND", r.Method)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		if r.Header.Get("Depth") == "0" {
			w.WriteHeader(207)
			_, err := fmt.Fprint(w, self)
			require.NoError(t, err)
			return
		}
		if listCalls.Add(1) == 1 {
			// return file-000..file-010 but kill the response before it
			// completes, so the client has parsed those entries when it
			// hits an unexpected EOF
			w.Header().Set("Content-Length", "1000000")
			w.WriteHeader(207)
			writeEntries(w, 0, 10)
			return
		}
		// return the disjoint file-011..file-030 as a complete listing
		w.WriteHeader(207)
		writeEntries(w, 11, 30)
		_, err := fmt.Fprint(w, "</d:multistatus>")
		require.NoError(t, err)
	}))
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	}
	f, err := webdav.NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	var remotes []string
	for _, e := range entries {
		remotes = append(remotes, e.Remote())
	}
	var want []string
	for i := 11; i <= 30; i++ {
		want = append(want, fmt.Sprintf("file-%03d.bin", i))
	}
	assert.ElementsMatch(t, want, remotes)
}

// digestServer runs a WebDAV server on dir which only accepts digest
// authentication and returns a count of the requests it received.
func digestServer(t *testing.T, dir string, testUser, testPass, testDigestRealm string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	authenticator := auth.NewDigestAuthenticator(testDigestRealm, func(user, realm string) string {
		if user == testUser {
			return testPass
		}
		return ""
	})
	authenticator.PlainTextSecrets = true // the callback returns the password, not an HA1 hash
	handler := &netwebdav.Handler{
		FileSystem: netwebdav.Dir(dir),
		LockSystem: netwebdav.NewMemLS(),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		authenticator.Wrap(func(w http.ResponseWriter, ar *auth.AuthenticatedRequest) {
			handler.ServeHTTP(w, &ar.Request)
		})(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, &requests
}

// TestDigestAuth checks that a server which only accepts digest authentication
// can be listed, and that each listing after the first costs a single request.
//
// The nonce count must increase for every signed request: a server which sees
// one repeated treats it as a replay, answers 401 with a fresh nonce, and the
// listing costs two round trips instead of one.
func TestDigestAuth(t *testing.T) {
	testDigestFilename := "testDigestAuthFile.txt"
	testDigestFilenameContent := []byte("hello world")

	testDigestAuthUser := "user"
	testDigestAuthPwd := "pwd"
	testDigestAuthRealm := "test"

	ctx := context.Background()

	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, testDigestFilename), testDigestFilenameContent, 0600)
	require.NoError(t, err)
	configfile.Install()

	ts, requests := digestServer(t, dir, testDigestAuthUser, testDigestAuthPwd, testDigestAuthRealm)
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": testDigestAuthUser,
		"pass": obscure.MustObscure(testDigestAuthPwd),
	})
	require.NoError(t, err)

	reqCount := 3
	for range reqCount {
		entries, err := f.List(ctx, "")
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, testDigestFilename, entries[0].Remote())
		assert.Equal(t, int64(len(testDigestFilenameContent)), entries[0].Size())
	}

	// 1 unsigned request to get the challenge, then one signed request for each
	assert.Equal(t, int32(reqCount+1), requests.Load())
}

// TestDigestAuthWrongPassword checks that bad credentials fail after a single
// signed attempt rather than being retried against the server ten times.
//
// Every 401 carries a challenge, including the ones rejecting a signature, so
// the retry has to tell a first challenge from a rejection.
func TestDigestAuthWrongPassword(t *testing.T) {
	testDigestAuthUser := "user"
	testDigestAuthPwd := "pwd"
	testDigestAuthRealm := "test"

	ctx := context.Background()

	configfile.Install()

	ts, requests := digestServer(t, t.TempDir(), testDigestAuthUser, testDigestAuthPwd, testDigestAuthRealm)
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": testDigestAuthUser,
		"pass": obscure.MustObscure("wrong" + testDigestAuthPwd),
	})
	require.NoError(t, err)

	_, err = f.List(ctx, "")
	require.Error(t, err)

	// 1 unsigned request to get the challenge, then 1 signed attempt which the
	// server rejects because of the wrong password
	assert.Equal(t, int32(2), requests.Load())
}

// TestDigestAuthStaleNonce checks that a request signed with an expired nonce
// is signed again with the replacement the server sends.
//
// A server only sets stale=true when the credentials were correct and the
// nonce had expired, so unlike any other 401 to a signed request, it is worth
// retrying.
func TestDigestAuthStaleNonce(t *testing.T) {
	ctx := context.Background()

	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorisation := r.Header.Get("Authorization")
		switch n := requests.Add(1); {
		case n == 1:
			// rclone sends basic authentication until it knows better
			assert.False(t, strings.HasPrefix(authorisation, "Digest "), "first request can't be signed yet")
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="nonce-1", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
		case n == 2:
			// the nonce has expired, so ask for a signature made with a new one
			assert.Contains(t, authorisation, `nonce="nonce-1"`)
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="nonce-2", algorithm=MD5, qop="auth", stale=true`)
			w.WriteHeader(http.StatusUnauthorized)
		default:
			assert.Contains(t, authorisation, `nonce="nonce-2"`, "the stale nonce should have been replaced")
			_, err := fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
			require.NoError(t, err)
		}
	}))
	defer ts.Close()

	configfile.Install()
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": "user",
		"pass": obscure.MustObscure("pwd"),
	})
	require.NoError(t, err)

	_, err = f.List(ctx, "")
	require.NoError(t, err)

	// 1 unsigned, 1 signed with the expired nonce, 1 signed with the new nonce
	assert.Equal(t, int32(3), requests.Load())
}

// TestDigestAuthRedirectToOtherHost checks that a redirect somewhere else is
// sent unsigned, so the credentials only reach the host which asked for them.
func TestDigestAuthRedirectToOtherHost(t *testing.T) {
	ctx := context.Background()

	// the redirect target, which should never see a digest signature
	var otherAuth atomic.Value
	otherAuth.Store("")
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherAuth.Store(r.Header.Get("Authorization"))
		_, err := fmt.Fprint(w, `<d:multistatus xmlns:d="DAV:"></d:multistatus>`)
		require.NoError(t, err)
	}))
	defer other.Close()

	// challenges once, then redirects the signed retry to the other host
	var originAuth atomic.Value
	originAuth.Store("")
	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("WWW-Authenticate", `Digest realm="test", nonce="nonce-1", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		originAuth.Store(r.Header.Get("Authorization"))
		http.Redirect(w, r, other.URL+"/", http.StatusFound)
	}))
	defer ts.Close()

	configfile.Install()
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": "user",
		"pass": obscure.MustObscure("pwd"),
	})
	require.NoError(t, err)

	_, err = f.List(ctx, "")
	require.NoError(t, err)

	assert.Contains(t, originAuth.Load(), "Digest ", "the challenging host should be signed")
	assert.NotContains(t, otherAuth.Load(), "Digest ", "another host shouldn't be sent the credentials")
}

// TestDigestAuthUpload checks that a file can be uploaded to a server which
// only accepts digest authentication.
//
// The signature covers the request URI but not the body, so the body
// is streamed rather than held in memory to be sent a second time.
func TestDigestAuthUpload(t *testing.T) {
	testDigestAuthUser := "user"
	testDigestAuthPwd := "pwd"
	testDigestAuthRealm := "test"

	testDigestFilename := "testDigestAuthFile.txt"
	testDigestFilenameContent := "hello world"

	ctx := context.Background()

	dir := t.TempDir()
	configfile.Install()

	ts, _ := digestServer(t, dir, testDigestAuthUser, testDigestAuthPwd, testDigestAuthRealm)
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": testDigestAuthUser,
		"pass": obscure.MustObscure(testDigestAuthPwd),
	})
	require.NoError(t, err)

	_, err = operations.Rcat(ctx, f, testDigestFilename,
		io.NopCloser(strings.NewReader(testDigestFilenameContent)), time.Now(), nil)
	require.NoError(t, err)

	written, err := os.ReadFile(filepath.Join(dir, testDigestFilename))
	require.NoError(t, err)
	assert.Equal(t, testDigestFilenameContent, string(written))
}

// TestBasicAuthNotRetried checks that a 401 from a server which doesn't offer
// digest authentication is reported straight away
func TestBasicAuthNotRetried(t *testing.T) {
	ctx := context.Background()

	var requests atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	configfile.Install()
	f, err := webdav.NewFs(ctx, remoteName, "", configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
		"user": "user",
		"pass": obscure.MustObscure("pwd"),
	})
	require.NoError(t, err)

	_, err = f.List(ctx, "")
	require.Error(t, err)
	assert.Equal(t, int32(1), requests.Load(), "a 401 without a digest challenge shouldn't be retried")
}
