// Serve webdav tests set up a server and run the integration tests
// for the webdav remote against it.
//
// We skip tests on platforms with troublesome character mappings

//go:build !windows && !darwin

package webdav

import (
	"compress/gzip"
	"context"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

const (
	testBindAddress = "localhost:0"
	testUser        = "user"
	testPass        = "pass"
	testTemplate    = "../http/testdata/golden/testindex.html"
	testAllowOrigin = "http://test.rclone.org"
)

// check interfaces
var (
	_ os.FileInfo         = FileInfo{nil, nil}
	_ webdav.ETager       = FileInfo{nil, nil}
	_ webdav.ContentTyper = FileInfo{nil, nil}
)

// TestWebDav runs the webdav server then runs the unit tests for the
// webdav remote against it.
func TestWebDav(t *testing.T) {
	// Configure and start the server
	start := func(f fs.Fs) (configmap.Simple, func()) {
		opt := Opt
		opt.HTTP.ListenAddr = []string{testBindAddress}
		opt.HTTP.BaseURL = "/prefix"
		opt.Auth.BasicUser = testUser
		opt.Auth.BasicPass = testPass
		opt.Template.Path = testTemplate
		opt.EtagHash = "MD5"

		// Start the server
		w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
		require.NoError(t, err)
		go func() {
			require.NoError(t, w.Serve())
		}()

		// Config for the backend we'll use to connect to the server
		config := configmap.Simple{
			"type":   "webdav",
			"vendor": "rclone",
			"url":    w.server.URLs()[0],
			"user":   testUser,
			"pass":   obscure.MustObscure(testPass),
		}

		return config, func() {
			assert.NoError(t, w.Shutdown())
		}
	}

	servetest.Run(t, "webdav", start)
}

// Test serve http functionality in serve webdav
// While similar to http serve, there are some inconsistencies
// in the handling of some requests such as POST requests

var (
	updateGolden = flag.Bool("updategolden", false, "update golden files for regression test")
)

func TestHTTPFunction(t *testing.T) {
	ctx := context.Background()
	// exclude files called hidden.txt and directories called hidden
	fi := filter.GetConfig(ctx)
	require.NoError(t, fi.AddRule("- hidden.txt"))
	require.NoError(t, fi.AddRule("- hidden/**"))

	// Uses the same test files as http tests but with different golden.
	f, err := fs.NewFs(context.Background(), "../http/testdata/files")
	assert.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{testBindAddress}
	opt.Template.Path = testTemplate

	// Start the server
	w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	assert.NoError(t, err)
	go func() {
		require.NoError(t, w.Serve())
	}()
	defer func() {
		assert.NoError(t, w.Shutdown())
	}()
	testURL := w.server.URLs()[0]
	HelpTestGET(t, testURL)
}

// check body against the file, or re-write body if -updategolden is
// set.
func checkGolden(t *testing.T, fileName string, got []byte) {
	if *updateGolden {
		t.Logf("Updating golden file %q", fileName)
		err := os.WriteFile(fileName, got, 0666)
		require.NoError(t, err)
	} else {
		want, err := os.ReadFile(fileName)
		require.NoError(t, err, "problem")
		wants := strings.Split(string(want), "\n")
		gots := strings.Split(string(got), "\n")
		assert.Equal(t, wants, gots, fileName)
	}
}

func HelpTestGET(t *testing.T, testURL string) {
	for _, test := range []struct {
		URL    string
		Status int
		Golden string
		Method string
		Range  string
	}{
		{
			URL:    "",
			Status: http.StatusOK,
			Golden: "testdata/golden/index.html",
		},
		{
			URL:    "notfound",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/notfound.html",
		},
		{
			URL:    "dirnotfound/",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/dirnotfound.html",
		},
		{
			URL:    "hidden/",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/hiddendir.html",
		},
		{
			URL:    "one%25.txt",
			Status: http.StatusOK,
			Golden: "testdata/golden/one.txt",
		},
		{
			URL:    "hidden.txt",
			Status: http.StatusNotFound,
			Golden: "testdata/golden/hidden.txt",
		},
		{
			URL:    "three/",
			Status: http.StatusOK,
			Golden: "testdata/golden/three.html",
		},
		{
			URL:    "three/a.txt",
			Status: http.StatusOK,
			Golden: "testdata/golden/a.txt",
		},
		{
			URL:    "",
			Method: "HEAD",
			Status: http.StatusOK,
			Golden: "testdata/golden/indexhead.txt",
		},
		{
			URL:    "one%25.txt",
			Method: "HEAD",
			Status: http.StatusOK,
			Golden: "testdata/golden/onehead.txt",
		},
		{
			URL:    "",
			Method: "POST",
			Status: http.StatusMethodNotAllowed,
			Golden: "testdata/golden/indexpost.txt",
		},
		{
			URL:    "one%25.txt",
			Method: "POST",
			Status: http.StatusOK,
			Golden: "testdata/golden/onepost.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusOK,
			Golden: "testdata/golden/two.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusPartialContent,
			Range:  "bytes=2-5",
			Golden: "testdata/golden/two2-5.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusPartialContent,
			Range:  "bytes=0-6",
			Golden: "testdata/golden/two-6.txt",
		},
		{
			URL:    "two.txt",
			Status: http.StatusPartialContent,
			Range:  "bytes=3-",
			Golden: "testdata/golden/two3-.txt",
		},
	} {
		method := test.Method
		if method == "" {
			method = "GET"
		}
		req, err := http.NewRequest(method, testURL+test.URL, nil)
		require.NoError(t, err)
		if test.Range != "" {
			req.Header.Add("Range", test.Range)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, test.Status, resp.StatusCode, test.Golden)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		checkGolden(t, test.Golden, body)
	}
}

// startAuthenticatedServer creates a webdav server with basic auth against
// the test files directory, starts it, waits for it to be ready, and returns
// the base URL. It registers cleanup to shut the server down.
func startAuthenticatedServer(t *testing.T) string {
	return startAuthenticatedServerAllowOrigin(t, "")
}

// startAuthenticatedServerAllowOrigin is startAuthenticatedServer
// with CORS enabled for allowOrigin if it is non-empty.
func startAuthenticatedServerAllowOrigin(t *testing.T, allowOrigin string) string {
	t.Helper()

	f, err := fs.NewFs(context.Background(), "../http/testdata/files")
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{testBindAddress}
	opt.HTTP.AllowOrigin = allowOrigin
	opt.Template.Path = testTemplate
	opt.Auth.BasicUser = testUser
	opt.Auth.BasicPass = testPass

	w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)
	go func() {
		require.NoError(t, w.Serve())
	}()
	t.Cleanup(func() {
		assert.NoError(t, w.Shutdown())
	})

	testURL := w.server.URLs()[0]
	return testURL
}

// TestOPTIONSRequiresAuth checks OPTIONS can't be used to discover
// whether a path exists, and whether it is a file or a directory,
// without authenticating.
func TestOPTIONSRequiresAuth(t *testing.T) {
	doOPTIONS := func(url string, setup func(req *http.Request)) *http.Response {
		req, err := http.NewRequest("OPTIONS", url, nil)
		require.NoError(t, err)
		if setup != nil {
			setup(req)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		return resp
	}

	testURL := startAuthenticatedServer(t)

	// The Allow header the WebDAV handler returns differs for an
	// existing file, an existing directory and a missing path so
	// it must not be visible without credentials.
	allows := map[string]string{}
	for _, path := range []string{"two.txt", "three/", "doesnotexist"} {
		t.Run(path, func(t *testing.T) {
			resp := doOPTIONS(testURL+path, nil)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Empty(t, resp.Header.Get("Allow"))
			assert.Empty(t, resp.Header.Get("DAV"))
			assert.NotEmpty(t, resp.Header.Get("WWW-Authenticate"))

			resp = doOPTIONS(testURL+path, func(req *http.Request) {
				req.SetBasicAuth(testUser, testPass)
			})
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NotEmpty(t, resp.Header.Get("Allow"))
			assert.Equal(t, "1, 2", resp.Header.Get("DAV"))
			allows[path] = resp.Header.Get("Allow")
		})
	}
	assert.NotEqual(t, allows["two.txt"], allows["three/"])
	assert.NotEqual(t, allows["three/"], allows["doesnotexist"])
	assert.NotEqual(t, allows["two.txt"], allows["doesnotexist"])

	// A browser CORS preflight can't carry credentials so it must
	// succeed, but it mustn't reach the WebDAV handler
	t.Run("Preflight", func(t *testing.T) {
		testURL := startAuthenticatedServerAllowOrigin(t, testAllowOrigin)
		resp := doOPTIONS(testURL+"two.txt", func(req *http.Request) {
			req.Header.Set("Origin", testAllowOrigin)
			req.Header.Set("Access-Control-Request-Method", "PROPFIND")
		})
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Allow"))
		assert.Empty(t, resp.Header.Get("DAV"))
		assert.Equal(t, testAllowOrigin, resp.Header.Get("Access-Control-Allow-Origin"))
	})
}

func TestCompressedTextFile(t *testing.T) {
	testURL := startAuthenticatedServer(t)

	req, err := http.NewRequest("GET", testURL+"two.txt", nil)
	require.NoError(t, err)
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer func() { _ = gr.Close() }()

	body, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, "0123456789\n", string(body))
}

func TestCompressedPROPFIND(t *testing.T) {
	testURL := startAuthenticatedServer(t)

	req, err := http.NewRequest("PROPFIND", testURL, nil)
	require.NoError(t, err)
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Depth", "1")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusMultiStatus, resp.StatusCode)
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gr, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	defer func() { _ = gr.Close() }()

	body, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Contains(t, string(body), "multistatus")
}

func TestRangeRequestNotCompressed(t *testing.T) {
	testURL := startAuthenticatedServer(t)

	req, err := http.NewRequest("GET", testURL+"two.txt", nil)
	require.NoError(t, err)
	req.SetBasicAuth(testUser, testPass)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Range", "bytes=2-5")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Empty(t, resp.Header.Get("Content-Encoding"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "2345", string(body))
}

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "webdav",
		"vfs_cache_mode": "off",
	})
}

// startWritableServer starts a webdav server backed by a fresh temp
// directory and returns the server URL. It is used by the Overwrite tests
// which need to exercise mutating verbs such as MKCOL and MOVE.
func startWritableServer(t *testing.T) string {
	t.Helper()

	f, err := fs.NewFs(context.Background(), t.TempDir())
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{testBindAddress}

	w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)
	go func() {
		require.NoError(t, w.Serve())
	}()
	t.Cleanup(func() {
		assert.NoError(t, w.Shutdown())
	})

	return w.server.URLs()[0]
}

func mkcol(t *testing.T, baseURL, path string) {
	t.Helper()
	req, err := http.NewRequest("MKCOL", baseURL+path, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "MKCOL %s", path)
}

// TestMoveDefaultsToOverwrite is a regression test for
// https://github.com/rclone/rclone/issues/9496
//
// RFC 4918 section 10.6 requires that when the Overwrite header is omitted
// from a COPY or MOVE request, the resource MUST behave as if Overwrite: T
// had been sent. The upstream golang.org/x/net/webdav library mis-handles
// the MOVE case (see https://github.com/golang/go/issues/66059), so rclone
// normalises the header before delegating so the default matches the RFC.
func TestMoveDefaultsToOverwrite(t *testing.T) {
	testURL := startWritableServer(t)

	mkcol(t, testURL, "dir1")
	mkcol(t, testURL, "dir2")

	// MOVE without Overwrite header: per RFC 4918 the default is T, so the
	// existing destination must be replaced and the server must return 2xx.
	req, err := http.NewRequest("MOVE", testURL+"dir2", nil)
	require.NoError(t, err)
	req.Header.Set("Destination", testURL+"dir1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.NotEqual(t, http.StatusPreconditionFailed, resp.StatusCode,
		"MOVE without Overwrite header must not return 412; RFC 4918 default is Overwrite: T")
	assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
		"expected 2xx, got %d", resp.StatusCode)
}

// TestMoveOverwriteFalseStillRejects ensures the rclone normalisation only
// fills in a missing Overwrite header and never overrides an explicit
// Overwrite: F sent by the client.
func TestMoveOverwriteFalseStillRejects(t *testing.T) {
	testURL := startWritableServer(t)

	mkcol(t, testURL, "dir1")
	mkcol(t, testURL, "dir2")

	req, err := http.NewRequest("MOVE", testURL+"dir2", nil)
	require.NoError(t, err)
	req.Header.Set("Destination", testURL+"dir1")
	req.Header.Set("Overwrite", "F")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusPreconditionFailed, resp.StatusCode,
		"MOVE with explicit Overwrite: F must still return 412 when destination exists")
}

// TestNewWebDAVError checks that a server initialisation failure is
// returned as an error rather than panicking in the cleanup.
func TestNewWebDAVError(t *testing.T) {
	f, err := fs.NewFs(context.Background(), t.TempDir())
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{"localhost:-1"}

	w, err := newWebDAV(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.Error(t, err)
	assert.Nil(t, w)
}

// TestAuthProxyDownloadOutlivesCache checks a download in progress
// carries on working when the auth proxy drops its VFS from its cache,
// as it does when a download takes longer than the cache expiry time.
func TestAuthProxyDownloadOutlivesCache(t *testing.T) {
	root := t.TempDir()
	contents := random.String(32 * 1024 * 1024)
	require.NoError(t, os.WriteFile(filepath.Join(root, "download.bin"), []byte(contents), 0666))

	prog, err := filepath.Abs("../servetest/proxy_code.go")
	require.NoError(t, err)
	opt := Opt
	opt.HTTP.ListenAddr = []string{testBindAddress}
	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "go run " + prog + " " + root
	w, err := newWebDAV(context.Background(), nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	go func() {
		require.NoError(t, w.Serve())
	}()
	defer func() { assert.NoError(t, w.Shutdown()) }()

	req, err := http.NewRequest("GET", w.server.URLs()[0]+"download.bin", nil)
	require.NoError(t, err)
	req.SetBasicAuth(testUser, testPass)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	start := make([]byte, 1024)
	_, err = io.ReadFull(resp.Body, start)
	require.NoError(t, err)

	// Drop everything from the proxy's cache as if it had expired
	w.provider.Proxy().Shutdown()

	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.True(t, contents == string(start)+string(rest), "download corrupted")
}

// TestWebDavPathTraversal checks that request paths with "." or ".."
// components are rejected with 400 before they reach the VFS, so they
// can't be joined with the Fs root to reach objects outside the
// directory served.
//
// It is run with and without a BaseURL as the webdav handler strips
// the BaseURL from the Destination header itself.
func TestWebDavPathTraversal(t *testing.T) {
	for _, baseURL := range []string{"", "/base"} {
		t.Run("BaseURL="+baseURL, func(t *testing.T) {
			testPathTraversal(t, baseURL)
		})
	}
}

// testPathTraversal runs the path traversal checks against a server
// with the BaseURL given.
//
// The memory backend is used because it resolves ".." by joining paths,
// unlike the local backend which encodes the components as file names.
func testPathTraversal(t *testing.T, baseURL string) {
	ctx := context.Background()
	bucket := ":memory:webdav-traversal-test" + strings.ReplaceAll(baseURL, "/", "-")
	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")

	// outside is the parent of the directory served and holds an object
	// which must stay unreachable through the server.
	outside, err := fs.NewFs(ctx, bucket)
	require.NoError(t, err)
	_, err = operations.Rcat(ctx, outside, "outside-secret.txt", io.NopCloser(strings.NewReader("SECRET")), modTime, nil)
	require.NoError(t, err)

	f, err := fs.NewFs(ctx, bucket+"/served-root")
	require.NoError(t, err)
	_, err = operations.Rcat(ctx, f, "inside.txt", io.NopCloser(strings.NewReader("INSIDE")), modTime, nil)
	require.NoError(t, err)
	_, err = operations.Rcat(ctx, f, "dir/inside-dir.txt", io.NopCloser(strings.NewReader("INSIDE DIR")), modTime, nil)
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{"localhost:0"}
	opt.HTTP.BaseURL = baseURL
	w, err := newWebDAV(ctx, f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)
	// The requests are made directly to the router, but the server
	// must be serving for Shutdown to close its listener.
	w.server.Serve()
	defer func() {
		assert.NoError(t, w.Shutdown())
	}()
	router := w.server.Router()

	// do makes a request for urlPath, and for COPY and MOVE with the
	// Destination dest, both of which have the baseURL added.
	do := func(method, urlPath, dest string) *httptest.ResponseRecorder {
		body := ""
		switch method {
		case "PUT":
			body = "EVIL"
		case "LOCK":
			body = `<?xml version="1.0" encoding="utf-8"?><D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`
		}
		req := httptest.NewRequest(method, baseURL+urlPath, strings.NewReader(body))
		switch method {
		case "COPY", "MOVE":
			req.Header.Set("Destination", baseURL+dest)
		case "LOCK":
			req.Header.Set("Depth", "0")
		case "PROPFIND":
			req.Header.Set("Depth", "1")
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	// A path inside the served root is unaffected
	rec := do("GET", "/inside.txt", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "INSIDE", rec.Body.String())

	// The root itself can be made
	rec = do("MKCOL", "/", "")
	assert.Equal(t, http.StatusCreated, rec.Code)

	type request struct {
		method  string
		urlPath string
		dest    string
	}
	// Every request naming a path with a "." or ".." element must fail.
	//
	// The MKCOL requests come first so that, should one succeed, the
	// later requests are routed through the directory node it made.
	requests := []request{
		{"MKCOL", "/..", ""},
		{"MKCOL", "/../", ""},
		{"MKCOL", "/../new-dir", ""},
		{"MKCOL", "/a/../../new-dir", ""},
		{"MKCOL", "/.", ""},
		{"MKCOL", "/./new-dir", ""},
		{"PUT", "/../outside-write.txt", ""},
		{"PUT", "/../outside-secret.txt", ""},
		{"PUT", "/%2e%2e/outside-write.txt", ""},
		{"PUT", "/a/../../outside-write.txt", ""},
		{"LOCK", "/../outside-lock.txt", ""},
		{"COPY", "/inside.txt", "/../outside-copy.txt"},
		{"COPY", "/inside.txt", "/../outside-secret.txt"},
		{"COPY", "/inside.txt", "/%2e%2e/outside-copy.txt"},
		{"MOVE", "/inside.txt", "/../outside-move.txt"},
		{"MOVE", "/inside.txt", "/a/../../outside-move.txt"},
		{"MKCOL", "/../outside-dir", ""},
		{"MOVE", "/../outside-dir", "/stolen-dir"},
		{"COPY", "/../outside-dir", "/copied-dir"},
		{"MOVE", "/../outside-secret.txt", "/stolen.txt"},
		{"COPY", "/../outside-secret.txt", "/copied.txt"},
		{"PROPPATCH", "/../outside-secret.txt", ""},
		{"PROPFIND", "/..", ""},
		{"GET", "/../outside-secret.txt", ""},
		{"GET", "/../", ""},
		{"GET", "/../?download=zip", ""},
		{"HEAD", "/../", ""},
		{"DELETE", "/../outside-secret.txt", ""},
		{"DELETE", "/..", ""},
	}
	if baseURL != "" {
		// Stripping the BaseURL as a string prefix from these
		// destinations leaves a path starting with a ".." element.
		//
		// The directory COPY comes first so that, should it succeed,
		// the later requests are routed through the node it made.
		requests = append([]request{
			{"COPY", "/dir", ".."},
			{"COPY", "/inside.txt", "../outside-copy.txt"},
			{"COPY", "/inside.txt", "../outside-secret.txt"},
			{"COPY", "/inside.txt", "%2e%2e/outside-copy.txt"},
			{"MOVE", "/inside.txt", "../outside-move.txt"},
		}, requests...)
	}
	for _, test := range requests {
		name := test.method + " " + test.urlPath
		if test.dest != "" {
			name += " to " + test.dest
		}
		t.Run(name, func(t *testing.T) {
			rec := do(test.method, test.urlPath, test.dest)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}

	// Nothing was created, modified, moved or removed, either outside
	// the served root or inside it.
	fstest.CheckListingWithPrecision(t, outside, []fstest.Item{
		fstest.NewItem("outside-secret.txt", "SECRET", modTime),
		fstest.NewItem("served-root/inside.txt", "INSIDE", modTime),
		fstest.NewItem("served-root/dir/inside-dir.txt", "INSIDE DIR", modTime),
	}, []string{"served-root", "served-root/dir"}, fs.GetModifyWindow(ctx, outside))
}
