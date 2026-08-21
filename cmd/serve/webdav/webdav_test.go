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
	"os"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/rc"
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
	t.Helper()

	f, err := fs.NewFs(context.Background(), "../http/testdata/files")
	require.NoError(t, err)

	opt := Opt
	opt.HTTP.ListenAddr = []string{testBindAddress}
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
