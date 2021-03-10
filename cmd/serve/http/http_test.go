package http

import (
	"context"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/httplib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	updateGolden = flag.Bool("updategolden", false, "update golden files for regression test")
	httpServer   *server
	testURL      string
)

const (
	testBindAddress = "localhost:0"
	testTemplate    = "testdata/golden/testindex.html"
)

func startServer(t *testing.T, f fs.Fs) {
	opt := httplib.DefaultOpt
	opt.ListenAddr = testBindAddress
	opt.Template = testTemplate
	httpServer = newServer(f, &opt)
	assert.NoError(t, httpServer.Serve())
	testURL = httpServer.Server.URL()

	// try to connect to the test server
	pause := time.Millisecond
	for i := 0; i < 10; i++ {
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

}

var (
	datedObject  = "two.txt"
	expectedTime = time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
)

func TestInit(t *testing.T) {
	ctx := context.Background()
	// Configure the remote
	configfile.LoadConfig(context.Background())
	// fs.Config.LogLevel = fs.LogLevelDebug
	// fs.Config.DumpHeaders = true
	// fs.Config.DumpBodies = true

	// exclude files called hidden.txt and directories called hidden
	fi := filter.GetConfig(ctx)
	require.NoError(t, fi.AddRule("- hidden.txt"))
	require.NoError(t, fi.AddRule("- hidden/**"))

	// Create a test Fs
	f, err := fs.NewFs(context.Background(), "testdata/files")
	require.NoError(t, err)

	// set date of datedObject to expectedTime
	obj, err := f.NewObject(context.Background(), datedObject)
	require.NoError(t, err)
	require.NoError(t, obj.SetModTime(context.Background(), expectedTime))

	startServer(t, f)
}

// check body against the file, or re-write body if -updategolden is
// set.
func checkGolden(t *testing.T, fileName string, got []byte) {
	if *updateGolden {
		t.Logf("Updating golden file %q", fileName)
		err := ioutil.WriteFile(fileName, got, 0666)
		require.NoError(t, err)
	} else {
		want, err := ioutil.ReadFile(fileName)
		require.NoError(t, err)
		wants := strings.Split(string(want), "\n")
		gots := strings.Split(string(got), "\n")
		assert.Equal(t, wants, gots, fileName)
	}
}

func TestGET(t *testing.T) {
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
		body, err := ioutil.ReadAll(resp.Body)
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

func TestFinalise(t *testing.T) {
	httpServer.Close()
	httpServer.Wait()
}
