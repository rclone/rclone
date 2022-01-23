package http

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	remoteName  = "TestHTTP"
	testPath    = "test"
	filesPath   = filepath.Join(testPath, "files")
	headers     = []string{"X-Potato", "sausage", "X-Rhubarb", "cucumber"}
	lineEndSize = 1
)

// prepareServer the test server and return a function to tidy it up afterwards
func prepareServer(t *testing.T) (configmap.Simple, func()) {
	// file server for test/files
	fileServer := http.FileServer(http.Dir(filesPath))

	// verify the file path is correct, and also check which line endings
	// are used to get sizes right ("\n" except on Windows, but even there
	// we may have "\n" or "\r\n" depending on git crlf setting)
	fileList, err := ioutil.ReadDir(filesPath)
	require.NoError(t, err)
	require.Greater(t, len(fileList), 0)
	for _, file := range fileList {
		if !file.IsDir() {
			data, _ := ioutil.ReadFile(filepath.Join(filesPath, file.Name()))
			if strings.HasSuffix(string(data), "\r\n") {
				lineEndSize = 2
			}
			break
		}
	}

	// test the headers are there then pass on to fileServer
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		what := fmt.Sprintf("%s %s: Header ", r.Method, r.URL.Path)
		assert.Equal(t, headers[1], r.Header.Get(headers[0]), what+headers[0])
		assert.Equal(t, headers[3], r.Header.Get(headers[2]), what+headers[2])
		fileServer.ServeHTTP(w, r)
	})

	// Make the test server
	ts := httptest.NewServer(handler)

	// Configure the remote
	configfile.Install()
	// fs.Config.LogLevel = fs.LogLevelDebug
	// fs.Config.DumpHeaders = true
	// fs.Config.DumpBodies = true
	// config.FileSet(remoteName, "type", "http")
	// config.FileSet(remoteName, "url", ts.URL)

	m := configmap.Simple{
		"type":    "http",
		"url":     ts.URL,
		"headers": strings.Join(headers, ","),
	}

	// return a function to tidy up
	return m, ts.Close
}

// prepare the test server and return a function to tidy it up afterwards
func prepare(t *testing.T) (fs.Fs, func()) {
	m, tidy := prepareServer(t)

	// Instantiate it
	f, err := NewFs(context.Background(), remoteName, "", m)
	require.NoError(t, err)

	return f, tidy
}

func testListRoot(t *testing.T, f fs.Fs, noSlash bool) {
	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)

	sort.Sort(entries)

	require.Equal(t, 4, len(entries))

	e := entries[0]
	assert.Equal(t, "four", e.Remote())
	assert.Equal(t, int64(-1), e.Size())
	_, ok := e.(fs.Directory)
	assert.True(t, ok)

	e = entries[1]
	assert.Equal(t, "one%.txt", e.Remote())
	assert.Equal(t, int64(5+lineEndSize), e.Size())
	_, ok = e.(*Object)
	assert.True(t, ok)

	e = entries[2]
	assert.Equal(t, "three", e.Remote())
	assert.Equal(t, int64(-1), e.Size())
	_, ok = e.(fs.Directory)
	assert.True(t, ok)

	e = entries[3]
	assert.Equal(t, "two.html", e.Remote())
	if noSlash {
		assert.Equal(t, int64(-1), e.Size())
		_, ok = e.(fs.Directory)
		assert.True(t, ok)
	} else {
		assert.Equal(t, int64(40+lineEndSize), e.Size())
		_, ok = e.(*Object)
		assert.True(t, ok)
	}
}

func TestListRoot(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()
	testListRoot(t, f, false)
}

func TestListRootNoSlash(t *testing.T) {
	f, tidy := prepare(t)
	f.(*Fs).opt.NoSlash = true
	defer tidy()

	testListRoot(t, f, true)
}

func TestListSubDir(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	entries, err := f.List(context.Background(), "three")
	require.NoError(t, err)

	sort.Sort(entries)

	assert.Equal(t, 1, len(entries))

	e := entries[0]
	assert.Equal(t, "three/underthree.txt", e.Remote())
	assert.Equal(t, int64(8+lineEndSize), e.Size())
	_, ok := e.(*Object)
	assert.True(t, ok)
}

func TestNewObject(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	o, err := f.NewObject(context.Background(), "four/under four.txt")
	require.NoError(t, err)

	assert.Equal(t, "four/under four.txt", o.Remote())
	assert.Equal(t, int64(8+lineEndSize), o.Size())
	_, ok := o.(*Object)
	assert.True(t, ok)

	// Test the time is correct on the object

	tObj := o.ModTime(context.Background())

	fi, err := os.Stat(filepath.Join(filesPath, "four", "under four.txt"))
	require.NoError(t, err)
	tFile := fi.ModTime()

	fstest.AssertTimeEqualWithPrecision(t, o.Remote(), tFile, tObj, time.Second)

	// check object not found
	o, err = f.NewObject(context.Background(), "not found.txt")
	assert.Nil(t, o)
	assert.Equal(t, fs.ErrorObjectNotFound, err)
}

func TestOpen(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	o, err := f.NewObject(context.Background(), "four/under four.txt")
	require.NoError(t, err)

	// Test normal read
	fd, err := o.Open(context.Background())
	require.NoError(t, err)
	data, err := ioutil.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())
	if lineEndSize == 2 {
		assert.Equal(t, "beetroot\r\n", string(data))
	} else {
		assert.Equal(t, "beetroot\n", string(data))
	}

	// Test with range request
	fd, err = o.Open(context.Background(), &fs.RangeOption{Start: 1, End: 5})
	require.NoError(t, err)
	data, err = ioutil.ReadAll(fd)
	require.NoError(t, err)
	require.NoError(t, fd.Close())
	assert.Equal(t, "eetro", string(data))
}

func TestMimeType(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	o, err := f.NewObject(context.Background(), "four/under four.txt")
	require.NoError(t, err)

	do, ok := o.(fs.MimeTyper)
	require.True(t, ok)
	assert.Equal(t, "text/plain; charset=utf-8", do.MimeType(context.Background()))
}

func TestIsAFileRoot(t *testing.T) {
	m, tidy := prepareServer(t)
	defer tidy()

	f, err := NewFs(context.Background(), remoteName, "one%.txt", m)
	assert.Equal(t, err, fs.ErrorIsFile)

	testListRoot(t, f, false)
}

func TestIsAFileSubDir(t *testing.T) {
	m, tidy := prepareServer(t)
	defer tidy()

	f, err := NewFs(context.Background(), remoteName, "three/underthree.txt", m)
	assert.Equal(t, err, fs.ErrorIsFile)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)

	sort.Sort(entries)

	assert.Equal(t, 1, len(entries))

	e := entries[0]
	assert.Equal(t, "underthree.txt", e.Remote())
	assert.Equal(t, int64(8+lineEndSize), e.Size())
	_, ok := e.(*Object)
	assert.True(t, ok)
}

func TestParseName(t *testing.T) {
	for i, test := range []struct {
		base    string
		val     string
		wantErr error
		want    string
	}{
		{"http://example.com/", "potato", nil, "potato"},
		{"http://example.com/dir/", "potato", nil, "potato"},
		{"http://example.com/dir/", "potato?download=true", errFoundQuestionMark, ""},
		{"http://example.com/dir/", "../dir/potato", nil, "potato"},
		{"http://example.com/dir/", "..", errNotUnderRoot, ""},
		{"http://example.com/dir/", "http://example.com/", errNotUnderRoot, ""},
		{"http://example.com/dir/", "http://example.com/dir/", errNameIsEmpty, ""},
		{"http://example.com/dir/", "http://example.com/dir/potato", nil, "potato"},
		{"http://example.com/dir/", "https://example.com/dir/potato", errSchemeMismatch, ""},
		{"http://example.com/dir/", "http://notexample.com/dir/potato", errHostMismatch, ""},
		{"http://example.com/dir/", "/dir/", errNameIsEmpty, ""},
		{"http://example.com/dir/", "/dir/potato", nil, "potato"},
		{"http://example.com/dir/", "subdir/potato", errNameContainsSlash, ""},
		{"http://example.com/dir/", "With percent %25.txt", nil, "With percent %.txt"},
		{"http://example.com/dir/", "With colon :", errURLJoinFailed, ""},
		{"http://example.com/dir/", rest.URLPathEscape("With colon :"), nil, "With colon :"},
		{"http://example.com/Dungeons%20%26%20Dragons/", "/Dungeons%20&%20Dragons/D%26D%20Basic%20%28Holmes%2C%20B%2C%20X%2C%20BECMI%29/", nil, "D&D Basic (Holmes, B, X, BECMI)/"},
	} {
		u, err := url.Parse(test.base)
		require.NoError(t, err)
		got, gotErr := parseName(u, test.val)
		what := fmt.Sprintf("test %d base=%q, val=%q", i, test.base, test.val)
		assert.Equal(t, test.wantErr, gotErr, what)
		assert.Equal(t, test.want, got, what)
	}
}

// Load HTML from the file given and parse it, checking it against the entries passed in
func parseHTML(t *testing.T, name string, base string, want []string) {
	in, err := os.Open(filepath.Join(testPath, "index_files", name))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, in.Close())
	}()
	if base == "" {
		base = "http://example.com/"
	}
	u, err := url.Parse(base)
	require.NoError(t, err)
	entries, err := parse(u, in)
	require.NoError(t, err)
	assert.Equal(t, want, entries)
}

func TestParseEmpty(t *testing.T) {
	parseHTML(t, "empty.html", "", []string(nil))
}

func TestParseApache(t *testing.T) {
	parseHTML(t, "apache.html", "http://example.com/nick/pub/", []string{
		"SWIG-embed.tar.gz",
		"avi2dvd.pl",
		"cambert.exe",
		"cambert.gz",
		"fedora_demo.gz",
		"gchq-challenge/",
		"mandelterm/",
		"pgp-key.txt",
		"pymath/",
		"rclone",
		"readdir.exe",
		"rush_hour_solver_cut_down.py",
		"snake-puzzle/",
		"stressdisk/",
		"timer-test",
		"words-to-regexp.pl",
		"Now 100% better.mp3",
		"Now better.mp3",
	})
}

func TestParseMemstore(t *testing.T) {
	parseHTML(t, "memstore.html", "", []string{
		"test/",
		"v1.35/",
		"v1.36-01-g503cd84/",
		"rclone-beta-latest-freebsd-386.zip",
		"rclone-beta-latest-freebsd-amd64.zip",
		"rclone-beta-latest-windows-amd64.zip",
	})
}

func TestParseNginx(t *testing.T) {
	parseHTML(t, "nginx.html", "", []string{
		"deltas/",
		"objects/",
		"refs/",
		"state/",
		"config",
		"summary",
	})
}

func TestParseCaddy(t *testing.T) {
	parseHTML(t, "caddy.html", "", []string{
		"mimetype.zip",
		"rclone-delete-empty-dirs.py",
		"rclone-show-empty-dirs.py",
		"stat-windows-386.zip",
		"v1.36-155-gcf29ee8b-team-driveβ/",
		"v1.36-156-gca76b3fb-team-driveβ/",
		"v1.36-156-ge1f0e0f5-team-driveβ/",
		"v1.36-22-g06ea13a-ssh-agentβ/",
	})
}

func TestFsNoSlashRoots(t *testing.T) {
	// Test Fs with roots that does not end with '/', the logic that
	// decides if url is to be considered a file or directory, based
	// on result from a HEAD request.

	// Handler for faking HEAD responses with different status codes
	headCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			headCount++
			responseCode, err := strconv.Atoi(path.Base(r.URL.String()))
			require.NoError(t, err)
			if strings.HasPrefix(r.URL.String(), "/redirect/") {
				var redir string
				if strings.HasPrefix(r.URL.String(), "/redirect/file/") {
					redir = "/redirected"
				} else if strings.HasPrefix(r.URL.String(), "/redirect/dir/") {
					redir = "/redirected/"
				} else {
					require.Fail(t, "Redirect test requests must start with '/redirect/file/' or '/redirect/dir/'")
				}
				http.Redirect(w, r, redir, responseCode)
			} else {
				http.Error(w, http.StatusText(responseCode), responseCode)
			}
		}
	})

	// Make the test server
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Configure the remote
	configfile.Install()
	m := configmap.Simple{
		"type": "http",
		"url":  ts.URL,
	}

	// Test
	for i, test := range []struct {
		root   string
		isFile bool
	}{
		// 2xx success
		{"parent/200", true},
		{"parent/204", true},

		// 3xx redirection Redirect status 301, 302, 303, 307, 308
		{"redirect/file/301", true}, // Request is redirected to "/redirected"
		{"redirect/dir/301", false}, // Request is redirected to "/redirected/"
		{"redirect/file/302", true}, // Request is redirected to "/redirected"
		{"redirect/dir/302", false}, // Request is redirected to "/redirected/"
		{"redirect/file/303", true}, // Request is redirected to "/redirected"
		{"redirect/dir/303", false}, // Request is redirected to "/redirected/"

		{"redirect/file/304", true}, // Not really a redirect, handled like 4xx errors (below)
		{"redirect/file/305", true}, // Not really a redirect, handled like 4xx errors (below)
		{"redirect/file/306", true}, // Not really a redirect, handled like 4xx errors (below)

		{"redirect/file/307", true}, // Request is redirected to "/redirected"
		{"redirect/dir/307", false}, // Request is redirected to "/redirected/"
		{"redirect/file/308", true}, // Request is redirected to "/redirected"
		{"redirect/dir/308", false}, // Request is redirected to "/redirected/"

		// 4xx client errors
		{"parent/403", true},  // Forbidden status (head request blocked)
		{"parent/404", false}, // Not found status
	} {
		for _, noHead := range []bool{false, true} {
			var isFile bool
			if noHead {
				m.Set("no_head", "true")
				isFile = true
			} else {
				m.Set("no_head", "false")
				isFile = test.isFile
			}
			headCount = 0
			f, err := NewFs(context.Background(), remoteName, test.root, m)
			if noHead {
				assert.Equal(t, 0, headCount)
			} else {
				assert.Equal(t, 1, headCount)
			}
			if isFile {
				assert.ErrorIs(t, err, fs.ErrorIsFile)
			} else {
				assert.NoError(t, err)
			}
			var endpoint string
			if isFile {
				parent, _ := path.Split(test.root)
				endpoint = "/" + parent
			} else {
				endpoint = "/" + test.root + "/"
			}
			what := fmt.Sprintf("i=%d, root=%q, isFile=%v, noHead=%v", i, test.root, isFile, noHead)
			assert.Equal(t, ts.URL+endpoint, f.String(), what)
		}
	}
}
