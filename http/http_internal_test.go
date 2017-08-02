// +build go1.8

package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	remoteName = "TestHTTP"
	testPath   = "test"
	filesPath  = filepath.Join(testPath, "files")
)

// prepareServer the test server and return a function to tidy it up afterwards
func prepareServer(t *testing.T) func() {
	// file server for test/files
	fileServer := http.FileServer(http.Dir(filesPath))

	// Make the test server
	ts := httptest.NewServer(fileServer)

	// Configure the remote
	fs.LoadConfig()
	// fs.Config.LogLevel = fs.LogLevelDebug
	// fs.Config.DumpHeaders = true
	// fs.Config.DumpBodies = true
	fs.ConfigFileSet(remoteName, "type", "http")
	fs.ConfigFileSet(remoteName, "url", ts.URL)

	// return a function to tidy up
	return ts.Close
}

// prepare the test server and return a function to tidy it up afterwards
func prepare(t *testing.T) (fs.Fs, func()) {
	tidy := prepareServer(t)

	// Instantiate it
	f, err := NewFs(remoteName, "")
	require.NoError(t, err)

	return f, tidy
}

func testListRoot(t *testing.T, f fs.Fs) {
	entries, err := f.List("")
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
	assert.Equal(t, int64(6), e.Size())
	_, ok = e.(*Object)
	assert.True(t, ok)

	e = entries[2]
	assert.Equal(t, "three", e.Remote())
	assert.Equal(t, int64(-1), e.Size())
	_, ok = e.(fs.Directory)
	assert.True(t, ok)

	e = entries[3]
	assert.Equal(t, "two.html", e.Remote())
	assert.Equal(t, int64(7), e.Size())
	_, ok = e.(*Object)
	assert.True(t, ok)
}

func TestListRoot(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()
	testListRoot(t, f)
}

func TestListSubDir(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	entries, err := f.List("three")
	require.NoError(t, err)

	sort.Sort(entries)

	assert.Equal(t, 1, len(entries))

	e := entries[0]
	assert.Equal(t, "three/underthree.txt", e.Remote())
	assert.Equal(t, int64(9), e.Size())
	_, ok := e.(*Object)
	assert.True(t, ok)
}

func TestNewObject(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	o, err := f.NewObject("four/under four.txt")
	require.NoError(t, err)

	assert.Equal(t, "four/under four.txt", o.Remote())
	assert.Equal(t, int64(9), o.Size())
	_, ok := o.(*Object)
	assert.True(t, ok)

	// Test the time is correct on the object

	tObj := o.ModTime()

	fi, err := os.Stat(filepath.Join(filesPath, "four", "under four.txt"))
	require.NoError(t, err)
	tFile := fi.ModTime()

	dt, ok := fstest.CheckTimeEqualWithPrecision(tObj, tFile, time.Second)
	assert.True(t, ok, fmt.Sprintf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", o.Remote(), dt, time.Second, tObj, tFile, time.Second))
}

func TestOpen(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	o, err := f.NewObject("four/under four.txt")
	require.NoError(t, err)

	// Test normal read
	fd, err := o.Open()
	require.NoError(t, err)
	data, err := ioutil.ReadAll(fd)
	require.NoError(t, fd.Close())
	assert.Equal(t, "beetroot\n", string(data))

	// Test with range request
	fd, err = o.Open(&fs.RangeOption{Start: 1, End: 5})
	require.NoError(t, err)
	data, err = ioutil.ReadAll(fd)
	require.NoError(t, fd.Close())
	assert.Equal(t, "eetro", string(data))
}

func TestMimeType(t *testing.T) {
	f, tidy := prepare(t)
	defer tidy()

	o, err := f.NewObject("four/under four.txt")
	require.NoError(t, err)

	do, ok := o.(fs.MimeTyper)
	require.True(t, ok)
	assert.Equal(t, "text/plain; charset=utf-8", do.MimeType())
}

func TestIsAFileRoot(t *testing.T) {
	tidy := prepareServer(t)
	defer tidy()

	f, err := NewFs(remoteName, "one%.txt")
	assert.Equal(t, err, fs.ErrorIsFile)

	testListRoot(t, f)
}

func TestIsAFileSubDir(t *testing.T) {
	tidy := prepareServer(t)
	defer tidy()

	f, err := NewFs(remoteName, "three/underthree.txt")
	assert.Equal(t, err, fs.ErrorIsFile)

	entries, err := f.List("")
	require.NoError(t, err)

	sort.Sort(entries)

	assert.Equal(t, 1, len(entries))

	e := entries[0]
	assert.Equal(t, "underthree.txt", e.Remote())
	assert.Equal(t, int64(9), e.Size())
	_, ok := e.(*Object)
	assert.True(t, ok)
}

func TestURLJoin(t *testing.T) {
	for i, test := range []struct {
		base   string
		path   string
		wantOK bool
		want   string
	}{
		{"http://example.com/", "potato", true, "http://example.com/potato"},
		{"http://example.com/dir/", "potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "../dir/potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "..", true, "http://example.com/"},
		{"http://example.com/dir/", "http://example.com/", true, "http://example.com/"},
		{"http://example.com/dir/", "http://example.com/dir/", true, "http://example.com/dir/"},
		{"http://example.com/dir/", "http://example.com/dir/potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "/dir/", true, "http://example.com/dir/"},
		{"http://example.com/dir/", "/dir/potato", true, "http://example.com/dir/potato"},
		{"http://example.com/dir/", "subdir/potato", true, "http://example.com/dir/subdir/potato"},
		{"http://example.com/dir/", "With percent %25.txt", true, "http://example.com/dir/With%20percent%20%25.txt"},
		{"http://example.com/dir/", "With colon :", false, ""},
		{"http://example.com/dir/", urlEscape("With colon :"), true, "http://example.com/dir/With%20colon%20:"},
	} {
		u, err := url.Parse(test.base)
		require.NoError(t, err)
		got, err := urlJoin(u, test.path)
		gotOK := err == nil
		what := fmt.Sprintf("test %d base=%q, val=%q", i, test.base, test.path)
		assert.Equal(t, test.wantOK, gotOK, what)
		var gotString string
		if gotOK {
			gotString = got.String()
		}
		assert.Equal(t, test.want, gotString, what)
	}
}

func TestURLEscape(t *testing.T) {
	for i, test := range []struct {
		path string
		want string
	}{
		{"", ""},
		{"/hello.txt", "/hello.txt"},
		{"With Space", "With%20Space"},
		{"With Colon:", "./With%20Colon:"},
		{"With Percent%", "With%20Percent%25"},
	} {
		got := urlEscape(test.path)
		assert.Equal(t, test.want, got, fmt.Sprintf("Test %d path = %q", i, test.path))
	}
}

func TestParseName(t *testing.T) {
	for i, test := range []struct {
		base   string
		val    string
		wantOK bool
		want   string
	}{
		{"http://example.com/", "potato", true, "potato"},
		{"http://example.com/dir/", "potato", true, "potato"},
		{"http://example.com/dir/", "../dir/potato", true, "potato"},
		{"http://example.com/dir/", "..", false, ""},
		{"http://example.com/dir/", "http://example.com/", false, ""},
		{"http://example.com/dir/", "http://example.com/dir/", false, ""},
		{"http://example.com/dir/", "http://example.com/dir/potato", true, "potato"},
		{"http://example.com/dir/", "/dir/", false, ""},
		{"http://example.com/dir/", "/dir/potato", true, "potato"},
		{"http://example.com/dir/", "subdir/potato", false, ""},
		{"http://example.com/dir/", "With percent %25.txt", true, "With percent %.txt"},
		{"http://example.com/dir/", "With colon :", false, ""},
		{"http://example.com/dir/", urlEscape("With colon :"), true, "With colon :"},
	} {
		u, err := url.Parse(test.base)
		require.NoError(t, err)
		got, gotOK := parseName(u, test.val)
		what := fmt.Sprintf("test %d base=%q, val=%q", i, test.base, test.val)
		assert.Equal(t, test.wantOK, gotOK, what)
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
