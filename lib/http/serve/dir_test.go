package serve

import (
	"context"
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func GetTemplate(t *testing.T) *template.Template {
	htmlTemplate, err := libhttp.GetTemplate("../../../cmd/serve/http/testdata/golden/testindex.html")
	require.NoError(t, err)
	return htmlTemplate
}

func TestNewDirectory(t *testing.T) {
	d := NewDirectory("z", GetTemplate(t))
	assert.Equal(t, "z", d.DirRemote)
	assert.Equal(t, "Directory listing of /z", d.Title)
}

func TestSetQuery(t *testing.T) {
	d := NewDirectory("z", GetTemplate(t))
	assert.Equal(t, "", d.Query)
	d.SetQuery(url.Values{"potato": []string{"42"}})
	assert.Equal(t, "?potato=42", d.Query)
	d.SetQuery(url.Values{})
	assert.Equal(t, "", d.Query)
}

func TestAddHTMLEntry(t *testing.T) {
	var modtime = time.Now()
	var d = NewDirectory("z", GetTemplate(t))
	d.AddHTMLEntry("", true, 0, modtime)
	d.AddHTMLEntry("dir", true, 0, modtime)
	d.AddHTMLEntry("a/b/c/d.txt", false, 64, modtime)
	d.AddHTMLEntry("a/b/c/colon:colon.txt", false, 64, modtime)
	d.AddHTMLEntry("\"quotes\".txt", false, 64, modtime)
	assert.Equal(t, []DirEntry{
		{remote: "", URL: "/", ZipURL: "/?download=zip", Leaf: "/", IsDir: true, Size: 0, ModTime: modtime},
		{remote: "dir", URL: "dir/", ZipURL: "dir/?download=zip", Leaf: "dir/", IsDir: true, Size: 0, ModTime: modtime},
		{remote: "a/b/c/d.txt", URL: "d.txt", ZipURL: "", Leaf: "d.txt", IsDir: false, Size: 64, ModTime: modtime},
		{remote: "a/b/c/colon:colon.txt", URL: "./colon:colon.txt", ZipURL: "", Leaf: "colon:colon.txt", IsDir: false, Size: 64, ModTime: modtime},
		{remote: "\"quotes\".txt", URL: "%22quotes%22.txt", ZipURL: "", Leaf: "\"quotes\".txt", Size: 64, IsDir: false, ModTime: modtime},
	}, d.Entries)

	// Now test with a query parameter
	d = NewDirectory("z", GetTemplate(t)).SetQuery(url.Values{"potato": []string{"42"}})
	d.AddHTMLEntry("file", false, 64, modtime)
	d.AddHTMLEntry("dir", true, 0, modtime)
	assert.Equal(t, []DirEntry{
		{remote: "file", URL: "file?potato=42", ZipURL: "", Leaf: "file", IsDir: false, Size: 64, ModTime: modtime},
		{remote: "dir", URL: "dir/?potato=42", ZipURL: "dir/?download=zip", Leaf: "dir/", IsDir: true, Size: 0, ModTime: modtime},
	}, d.Entries)
}

func TestAddEntry(t *testing.T) {
	var d = NewDirectory("z", GetTemplate(t))
	d.AddEntry("", true)
	d.AddEntry("dir", true)
	d.AddEntry("a/b/c/d.txt", false)
	d.AddEntry("a/b/c/colon:colon.txt", false)
	d.AddEntry("\"quotes\".txt", false)
	assert.Equal(t, []DirEntry{
		{remote: "", URL: "/", Leaf: "/"},
		{remote: "dir", URL: "dir/", Leaf: "dir/"},
		{remote: "a/b/c/d.txt", URL: "d.txt", Leaf: "d.txt"},
		{remote: "a/b/c/colon:colon.txt", URL: "./colon:colon.txt", Leaf: "colon:colon.txt"},
		{remote: "\"quotes\".txt", URL: "%22quotes%22.txt", Leaf: "\"quotes\".txt"},
	}, d.Entries)

	// Now test with a query parameter
	d = NewDirectory("z", GetTemplate(t)).SetQuery(url.Values{"potato": []string{"42"}})
	d.AddEntry("file", false)
	d.AddEntry("dir", true)
	assert.Equal(t, []DirEntry{
		{remote: "file", URL: "file?potato=42", Leaf: "file"},
		{remote: "dir", URL: "dir/?potato=42", Leaf: "dir/"},
	}, d.Entries)
}

func TestError(t *testing.T) {
	ctx := context.Background()
	w := httptest.NewRecorder()
	err := errors.New("help")
	Error(ctx, "potato", w, "sausage", err)
	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "sausage.\n", string(body))
}

func TestServe(t *testing.T) {
	d := NewDirectory("aDirectory", GetTemplate(t))
	d.AddEntry("file", false)
	d.AddEntry("dir", true)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://example.com/aDirectory/", nil)
	d.Serve(w, r)
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Directory listing of /aDirectory</title>
</head>
<body>
<h1>Directory listing of /aDirectory</h1>
<a href="file">file</a><br />
<a href="dir/">dir/</a><br />
</body>
</html>
`, string(body))
}

func TestDirectory_ProcessQueryParams(t *testing.T) {
	newDate := func(day int) time.Time {
		return time.Date(2025, time.September, day, 0, 0, 0, 0, time.UTC)
	}

	dir := Directory{
		Entries: []DirEntry{
			{Leaf: "a/", IsDir: true, ModTime: newDate(1), Size: 0},
			{Leaf: "test/", IsDir: true, ModTime: newDate(2), Size: 0},
			{Leaf: "test 1/", IsDir: true, ModTime: newDate(3), Size: 0},
			{Leaf: "hello.jpg", ModTime: newDate(1), Size: 1253028},
			{Leaf: "report 1.txt", ModTime: newDate(3), Size: 1000},
			{Leaf: "report 2.txt", ModTime: newDate(2), Size: 2000},
			{Leaf: "report 12.txt", ModTime: newDate(3), Size: 1500},
			{Leaf: "x.png", ModTime: newDate(3), Size: 1363148},
			{Leaf: "X.jpeg", ModTime: newDate(10), Size: 454382},
		},
	}

	assertEntriesOrder := func(wantLeafs ...string) {
		leafs := make([]string, 0, len(dir.Entries))
		for _, e := range dir.Entries {
			leafs = append(leafs, e.Leaf)
		}
		assert.Equal(t, wantLeafs, leafs)
	}

	// Sort by name
	dir.ProcessQueryParams(sortByName, "asc")
	assertEntriesOrder(
		"a/",
		"hello.jpg",
		"report 1.txt",
		"report 12.txt",
		"report 2.txt",
		"test/",
		"test 1/",
		"X.jpeg",
		"x.png",
	)

	// Sort by name, dirs first (should be the default order)
	for _, order := range []string{"", sortByNameDirFirst, "qwerty"} {
		dir.ProcessQueryParams(order, "asc")
		assertEntriesOrder(
			"a/",
			"test/",
			"test 1/",
			"hello.jpg",
			"report 1.txt",
			"report 12.txt",
			"report 2.txt",
			"X.jpeg",
			"x.png",
		)
	}
	//
	dir.ProcessQueryParams(sortByNameDirFirst, "desc")
	assertEntriesOrder(
		"x.png",
		"X.jpeg",
		"report 2.txt",
		"report 12.txt",
		"report 1.txt",
		"hello.jpg",
		"test 1/",
		"test/",
		"a/",
	)

	// Sort by size
	dir.ProcessQueryParams(sortBySize, "desc")
	assertEntriesOrder(
		"x.png",
		"hello.jpg",
		"X.jpeg",
		"report 2.txt",
		"report 12.txt",
		"report 1.txt",
		"test 1/",
		"test/",
		"a/",
	)

	// Sort by mod time
	dir.ProcessQueryParams(sortByTime, "asc")
	assertEntriesOrder(
		"a/",            // 2025-09-01
		"hello.jpg",     // 2025-09-01
		"test/",         // 2025-09-02
		"report 2.txt",  // 2025-09-02
		"test 1/",       // 2025-09-03
		"report 1.txt",  // 2025-09-03
		"report 12.txt", // 2025-09-03
		"x.png",         // 2025-09-03
		"X.jpeg",        // 2025-09-10
	)
}
