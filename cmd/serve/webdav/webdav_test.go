// Serve webdav tests set up a server and run the integration tests
// for the webdav remote against it.
//
// We skip tests on platforms with troublesome character mappings

//+build !windows,!darwin,go1.9

package webdav

import (
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/ncw/rclone/backend/local"
	"github.com/ncw/rclone/cmd/serve/httplib"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/filter"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

const (
	testBindAddress = "localhost:0"
)

// check interfaces
var (
	_ os.FileInfo         = FileInfo{nil}
	_ webdav.ETager       = FileInfo{nil}
	_ webdav.ContentTyper = FileInfo{nil}
)

// TestWebDav runs the webdav server then runs the unit tests for the
// webdav remote against it.
func TestWebDav(t *testing.T) {
	opt := httplib.DefaultOpt
	opt.ListenAddr = testBindAddress

	fstest.Initialise()

	fremote, _, clean, err := fstest.RandomRemote(*fstest.RemoteName, *fstest.SubDir)
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir("")
	assert.NoError(t, err)

	// Start the server
	w := newWebDAV(fremote, &opt)
	assert.NoError(t, w.serve())
	defer func() {
		w.Close()
		w.Wait()
	}()

	// Change directory to run the tests
	err = os.Chdir("../../../backend/webdav")
	assert.NoError(t, err, "failed to cd to webdav remote")

	// Run the webdav tests with an on the fly remote
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	args = append(args, "-remote", "webdavtest:")
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(),
		"RCLONE_CONFIG_WEBDAVTEST_TYPE=webdav",
		"RCLONE_CONFIG_WEBDAVTEST_URL="+w.Server.URL(),
		"RCLONE_CONFIG_WEBDAVTEST_VENDOR=other",
	)
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running webdav integration tests")
}

// Test serve http functionality in serve webdav
// While similar to http serve, there are some inconsistencies
// in the handling of some requests such as POST requests

var (
	updateGolden = flag.Bool("updategolden", false, "update golden files for regression test")
)

func TestHTTPFunction(t *testing.T) {
	// cd to correct directory for testing
	err := os.Chdir("../../cmd/serve/webdav")
	assert.NoError(t, err, "failed to cd to webdav cmd directory")

	// exclude files called hidden.txt and directories called hidden
	require.NoError(t, filter.Active.AddRule("- hidden.txt"))
	require.NoError(t, filter.Active.AddRule("- hidden/**"))

	// Uses the same test files as http tests but with different golden.
	f, err := fs.NewFs("../http/testdata/files")
	assert.NoError(t, err)

	opt := httplib.DefaultOpt
	opt.ListenAddr = testBindAddress

	// Start the server
	w := newWebDAV(f, &opt)
	assert.NoError(t, w.serve())
	defer func() {
		w.Close()
		w.Wait()
	}()
	testURL := w.Server.URL()
	pause := time.Millisecond
	i := 0
	for ; i < 10; i++ {
		resp, err := http.Head(testURL)
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		// t.Logf("couldn't connect, sleeping for %v: %v", pause, err)
		time.Sleep(pause)
		pause *= 2
	}
	if i >= 10 {
		t.Fatal("couldn't connect to server")
	}

	HelpTestGET(t, testURL)
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
		body, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)

		checkGolden(t, test.Golden, body)
	}
}
