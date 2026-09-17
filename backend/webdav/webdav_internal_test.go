package webdav_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rclone/rclone/backend/webdav"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// fileInfoResponse is a PROPFIND response describing a single file.
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

// redirectTestHeaders are the headers option for the redirect tests.
// The one starting with "*" is sent without canonicalising.
var redirectTestHeaders = []string{"X-Potato", "sausage", "*x-marrow", "pea"}

// redirectHeaders makes a webdav remote with redirectTestHeaders
// pointing at a server which redirects every request to a second
// server on hostname, or back to itself if hostname is empty, and
// exercises each of the backend's redirect policies against it: the
// PROPFIND used to look up an object, the GET used to read it and the
// PROPFIND used to list a directory. It returns the headers the
// redirect target saw on each of those requests.
func redirectHeaders(t *testing.T, hostname string) []http.Header {
	var target string
	var mu sync.Mutex
	var seen []http.Header
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/moved/") {
			http.Redirect(w, r, target+"/moved"+r.URL.Path, http.StatusTemporaryRedirect)
			return
		}
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		switch r.Method {
		case "PROPFIND":
			w.WriteHeader(http.StatusMultiStatus)
			_, err := fmt.Fprint(w, fileInfoResponse)
			assert.NoError(t, err)
		case http.MethodGet:
			_, err := fmt.Fprint(w, "hello")
			assert.NoError(t, err)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	if hostname == "" {
		target = ts.URL
	} else {
		// Serve the final response from a second server on a different host
		other := httptest.NewServer(handler)
		defer other.Close()
		target = strings.Replace(other.URL, "127.0.0.1", hostname, 1)
	}

	configfile.Install()
	m := configmap.Simple{
		"type":    "webdav",
		"url":     ts.URL,
		"headers": strings.Join(redirectTestHeaders, ","),
	}
	ctx := context.Background()
	f, err := webdav.NewFs(ctx, remoteName, "", m)
	require.NoError(t, err)

	// PROPFIND with Depth 0 uses rest.PreserveMethodRedirectFn
	o, err := f.NewObject(ctx, "file.txt")
	require.NoError(t, err)
	// GET uses the client's default redirect policy
	fd, err := o.Open(ctx)
	require.NoError(t, err)
	data, err := io.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())
	assert.Equal(t, "hello", string(data))
	// PROPFIND with Depth 1 uses the client's default redirect policy
	_, err = f.List(ctx, "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, seen, 3, "expected the redirect target to see all three requests")
	return seen
}

func TestRedirectKeepsHeadersOnSameHost(t *testing.T) {
	for _, got := range redirectHeaders(t, "") {
		for i := 0; i < len(redirectTestHeaders); i += 2 {
			assert.Equal(t, redirectTestHeaders[i+1], got.Get(strings.TrimPrefix(redirectTestHeaders[i], "*")))
		}
	}
}

func TestRedirectStripsHeadersOnHostChange(t *testing.T) {
	// Redirect to a different host name and port. net/http strips
	// Authorization on its own here so this checks the custom headers.
	for _, got := range redirectHeaders(t, "localhost") {
		for i := 0; i < len(redirectTestHeaders); i += 2 {
			assert.Empty(t, got.Values(strings.TrimPrefix(redirectTestHeaders[i], "*")), "header %q leaked to redirect target", redirectTestHeaders[i])
		}
	}
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

// TestNotFoundInMultistatus checks that a missing path is detected when the
// server reports it by appending a Sabre NotFound error to a 207 response it
// has already started, as ownCloud 10.16 does, rather than by returning 404.
func TestNotFoundInMultistatus(t *testing.T) {
	head := `<?xml version="1.0"?>` + "\n" + `<d:multistatus xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns" xmlns:oc="http://owncloud.org/ns"`
	root := head + `><d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`
	notFound := head + `<?xml version="1.0" encoding="utf-8"?>
<d:error xmlns:d="DAV:" xmlns:s="http://sabredav.org/ns">
  <s:exception>Sabre\DAV\Exception\NotFound</s:exception>
  <s:message>File with name missing could not be located</s:message>
</d:error>
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "PROPFIND", r.Method)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(207)
		body := notFound
		if r.URL.Path == "/" {
			body = root
		}
		_, err := fmt.Fprint(w, body)
		require.NoError(t, err)
	}))
	defer ts.Close()

	configfile.Install()
	m := configmap.Simple{
		"type": "webdav",
		"url":  ts.URL,
	}
	ctx := context.Background()

	// A root which doesn't exist yet
	_, err := webdav.NewFs(ctx, remoteName, "missing", m)
	require.NoError(t, err)

	f, err := webdav.NewFs(ctx, remoteName, "", m)
	require.NoError(t, err)

	_, err = f.NewObject(ctx, "missing")
	assert.Equal(t, fs.ErrorObjectNotFound, err)

	_, err = f.List(ctx, "missing")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}
