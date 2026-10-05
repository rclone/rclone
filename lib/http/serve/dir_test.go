package serve

import (
	"bytes"
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
		{remote: "", URL: "/", ZipURL: "/?download=zip", Leaf: "/", IsDir: true, Size: 0, ModTime: modtime, MimeType: "inode/directory"},
		{remote: "dir", URL: "./dir/", ZipURL: "./dir/?download=zip", Leaf: "dir/", IsDir: true, Size: 0, ModTime: modtime, MimeType: "inode/directory"},
		{remote: "a/b/c/d.txt", URL: "./d.txt", ZipURL: "", Leaf: "d.txt", IsDir: false, Size: 64, ModTime: modtime, MimeType: "text/plain; charset=utf-8"},
		{remote: "a/b/c/colon:colon.txt", URL: "./colon:colon.txt", ZipURL: "", Leaf: "colon:colon.txt", IsDir: false, Size: 64, ModTime: modtime, MimeType: "text/plain; charset=utf-8"},
		{remote: "\"quotes\".txt", URL: "./%22quotes%22.txt", ZipURL: "", Leaf: "\"quotes\".txt", Size: 64, IsDir: false, ModTime: modtime, MimeType: "text/plain; charset=utf-8"},
	}, d.Entries)

	// Now test with a query parameter
	d = NewDirectory("z", GetTemplate(t)).SetQuery(url.Values{"potato": []string{"42"}})
	d.AddHTMLEntry("file", false, 64, modtime)
	d.AddHTMLEntry("dir", true, 0, modtime)
	assert.Equal(t, []DirEntry{
		{remote: "file", URL: "./file?potato=42", ZipURL: "", Leaf: "file", IsDir: false, Size: 64, ModTime: modtime, MimeType: "application/octet-stream"},
		{remote: "dir", URL: "./dir/?potato=42", ZipURL: "./dir/?download=zip", Leaf: "dir/", IsDir: true, Size: 0, ModTime: modtime, MimeType: "inode/directory"},
	}, d.Entries)

	// Now test with a link index
	d = NewDirectory("z", GetTemplate(t)).SetLinkIndex("index.html")
	d.AddHTMLEntry("file", false, 64, modtime)
	d.AddHTMLEntry("dir", true, 0, modtime)
	assert.Equal(t, []DirEntry{
		{remote: "file", URL: "./file", ZipURL: "", Leaf: "file", IsDir: false, Size: 64, ModTime: modtime, MimeType: "application/octet-stream"},
		{remote: "dir", URL: "./dir/index.html", ZipURL: "./dir/?download=zip", Leaf: "dir/", IsDir: true, Size: 0, ModTime: modtime, MimeType: "inode/directory"},
	}, d.Entries)
	assert.Equal(t, []Crumb{{Link: "../index.html", Text: "/"}, {Link: "index.html", Text: "z"}}, d.Breadcrumb)
	assert.Equal(t, "../index.html", d.UpLink())
}

func TestDirectoryPath(t *testing.T) {
	d := NewDirectory("", GetTemplate(t))
	assert.Equal(t, "/", d.Path())
	assert.True(t, d.IsRoot())
	assert.Equal(t, "..", d.UpLink())

	d = NewDirectory("a/b", GetTemplate(t))
	assert.Equal(t, "/a/b/", d.Path())
	assert.False(t, d.IsRoot())

	d = NewDirectory("a/b/", GetTemplate(t))
	assert.Equal(t, "/a/b/", d.Path())
}

func TestDirectorySummary(t *testing.T) {
	d := NewDirectory("z", GetTemplate(t))
	d.AddHTMLEntry("file1", false, 64, time.Time{})
	d.AddHTMLEntry("file2", false, 100, time.Time{})
	d.AddHTMLEntry("dir", true, 0, time.Time{})
	assert.Equal(t, 1, d.NumDirs())
	assert.Equal(t, 2, d.NumFiles())
	assert.Equal(t, int64(164), d.TotalSize())
}

func TestRenderStatic(t *testing.T) {
	htmlTemplate, err := libhttp.GetTemplate("")
	require.NoError(t, err)
	render := func(dirRemote string, static bool) string {
		d := NewDirectory(dirRemote, htmlTemplate)
		d.Static = static
		d.AddHTMLEntry(dirRemote+"/file", false, 64, time.Time{})
		var buf bytes.Buffer
		require.NoError(t, d.Render(&buf))
		return buf.String()
	}

	// serve http output has an up link everywhere
	assert.Contains(t, render("z", false), "Go up")
	assert.Contains(t, render("", false), "Go up")

	// static output has no up link at the root but has one below it
	assert.NotContains(t, render("", true), "Go up")
	assert.Contains(t, render("z", true), "Go up")
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
