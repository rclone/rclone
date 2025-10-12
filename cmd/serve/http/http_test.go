package http

import (
	"context"
	"flag"
	"io"
	stdfs "io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/rc"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	updateGolden = flag.Bool("updategolden", false, "update golden files for regression test")
)

const (
	testBindAddress = "localhost:0"
	testUser        = "user"
	testPass        = "pass"
	testTemplate    = "testdata/golden/testindex.html"
)

func start(ctx context.Context, t *testing.T, f fs.Fs) (s *HTTP, testURL string) {
	opts := Options{
		HTTP: libhttp.DefaultCfg(),
		Template: libhttp.TemplateConfig{
			Path: testTemplate,
		},
	}
	opts.HTTP.ListenAddr = []string{testBindAddress}
	if proxy.Opt.AuthProxy == "" {
		opts.Auth.BasicUser = testUser
		opts.Auth.BasicPass = testPass
	}

	s, err := newServer(ctx, f, &opts, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err, "failed to start server")
	go func() {
		require.NoError(t, s.Serve())
	}()

	urls := s.server.URLs()
	require.Len(t, urls, 1, "expected one URL")

	testURL = urls[0]

	// try to connect to the test server
	pause := time.Millisecond
	for range 10 {
		resp, err := http.Head(testURL)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		// t.Logf("couldn't connect, sleeping for %v: %v", pause, err)
		time.Sleep(pause)
		pause *= 2
	}
	t.Fatal("couldn't connect to server")

	return s, testURL
}

// setAllModTimes walks root and sets atime/mtime to t for every file & directory.
func setAllModTimes(root string, t time.Time) error {
	return filepath.WalkDir(root, func(path string, d stdfs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, t, t)
	})
}

var (
	datedObject  = "two.txt"
	expectedTime = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
)

// check body against the file, or re-write body if -updategolden is
// set.
func checkGolden(t *testing.T, fileName string, got []byte) {
	if *updateGolden {
		t.Logf("Updating golden file %q", fileName)
		err := os.WriteFile(fileName, got, 0666)
		require.NoError(t, err)
	} else {
		want, err := os.ReadFile(fileName)
		require.NoError(t, err)
		wants := strings.Split(string(want), "\n")
		gots := strings.Split(string(got), "\n")
		assert.Equal(t, wants, gots, fileName)
	}
}

func testGET(t *testing.T, useProxy bool) {
	ctx := context.Background()
	// ci := fs.GetConfig(ctx)
	// ci.LogLevel = fs.LogLevelDebug

	// exclude files called hidden.txt and directories called hidden
	fi := filter.GetConfig(ctx)
	require.NoError(t, fi.AddRule("- hidden.txt"))
	require.NoError(t, fi.AddRule("- hidden/**"))

	var f fs.Fs
	if useProxy {
		// the backend config will be made by the proxy
		prog, err := filepath.Abs("../servetest/proxy_code.go")
		require.NoError(t, err)
		files, err := filepath.Abs("testdata/files")
		require.NoError(t, err)
		cmd := "go run " + prog + " " + files

		// FIXME this is untidy setting a global variable!
		proxy.Opt.AuthProxy = cmd
		defer func() {
			proxy.Opt.AuthProxy = ""
		}()

		f = nil
	} else {
		// set all the mod times to expectedTime
		require.NoError(t, setAllModTimes("testdata/files", expectedTime))
		// Create a test Fs
		var err error
		f, err = fs.NewFs(context.Background(), "testdata/files")
		require.NoError(t, err)

		// set date of datedObject to expectedTime
		obj, err := f.NewObject(context.Background(), datedObject)
		require.NoError(t, err)
		require.NoError(t, obj.SetModTime(context.Background(), expectedTime))
	}

	s, testURL := start(ctx, t, f)
	defer func() {
		assert.NoError(t, s.server.Shutdown())
	}()

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
			Status: http.StatusMethodNotAllowed,
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
		{
			URL:    "/?download=zip",
			Status: http.StatusOK,
			Golden: "testdata/golden/root.zip",
		},
		{
			URL:    "/three/?download=zip",
			Status: http.StatusOK,
			Golden: "testdata/golden/three.zip",
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
		req.SetBasicAuth(testUser, testPass)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, test.Status, resp.StatusCode, test.Golden)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// Check we got a Last-Modified header and that it is a valid date
		if test.Status == http.StatusOK || test.Status == http.StatusPartialContent {
			lastModified := resp.Header.Get("Last-Modified")
			assert.NotEqual(t, "", lastModified, test.Golden)
			modTime, err := http.ParseTime(lastModified)
			assert.NoError(t, err, test.Golden)
			// check the actual date on our special file
			if test.URL == datedObject {
				assert.Equal(t, expectedTime, modTime, test.Golden)
			}
		}

		checkGolden(t, test.Golden, body)
	}
}

func TestGET(t *testing.T) {
	testGET(t, false)
}

func TestAuthProxy(t *testing.T) {
	testGET(t, true)
}

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "http",
		"vfs_cache_mode": "off",
	})
}
