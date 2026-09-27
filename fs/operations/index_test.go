package operations_test

import (
	"context"
	"encoding/json"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// indexRun holds a test remote with a small tree of files in it
type indexRun struct {
	*fstest.Run
	ctx context.Context
	opt operations.IndexOpt
}

// newIndexRun makes a remote with file1.txt, sub/file2.txt and
// sub/deep/file3.txt in it
func newIndexRun(t *testing.T) *indexRun {
	opt := operations.IndexOptDefault
	opt.NoModTime = true
	r := &indexRun{Run: fstest.NewRun(t), ctx: context.Background(), opt: opt}
	r.writeFiles()
	return r
}

// writeFiles writes the test files to the remote
func (r *indexRun) writeFiles() {
	r.WriteObject(r.ctx, "file1.txt", "hello", t1)
	r.WriteObject(r.ctx, "sub/file2.txt", "hello2", t2)
	r.WriteObject(r.ctx, "sub/deep/file3.txt", "hello3", t3)
}

// index runs Index and returns the number of transfers and deletes it made
func (r *indexRun) index(t *testing.T) (transfers, deletes int64) {
	stats := accounting.GlobalStats()
	stats.ResetCounters()
	require.NoError(t, operations.Index(r.ctx, r.Fremote, &r.opt))
	return stats.GetTransfers(), stats.GetDeletes()
}

// read returns the contents of the object at remote
func (r *indexRun) read(t *testing.T, remote string) string {
	o, err := r.Fremote.NewObject(r.ctx, remote)
	require.NoError(t, err, remote)
	return fstests.ReadObject(r.ctx, t, o, -1)
}

// files returns the names of all the objects on the remote
func (r *indexRun) files(t *testing.T) (names []string) {
	err := walk.ListR(r.ctx, r.Fremote, "", true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		for _, entry := range entries {
			names = append(names, entry.Remote())
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(names)
	return names
}

// checkMimeType checks the MIME type of the object at remote is want,
// ignoring formatting differences, on backends which store it
func (r *indexRun) checkMimeType(t *testing.T, remote, want string) {
	o, err := r.Fremote.NewObject(r.ctx, remote)
	require.NoError(t, err)
	if _, ok := o.(fs.MimeTyper); !ok {
		return
	}
	got := fs.MimeType(r.ctx, o)
	wantType, wantParams, err := mime.ParseMediaType(want)
	require.NoError(t, err)
	gotType, gotParams, err := mime.ParseMediaType(got)
	require.NoError(t, err, got)
	assert.Equal(t, wantType, gotType)
	assert.Equal(t, wantParams, gotParams)
}

// checkFiles checks the remote contains exactly the objects named
func (r *indexRun) checkFiles(t *testing.T, want ...string) {
	sort.Strings(want)
	assert.Equal(t, want, r.files(t))
}

func TestIndex(t *testing.T) {
	r := newIndexRun(t)
	r.opt.Outputs = []string{"index.html=html", "index.json=json", "caddy.json=caddy"}

	transfers, deletes := r.index(t)
	assert.Equal(t, int64(9), transfers)
	assert.Equal(t, int64(0), deletes)
	r.checkFiles(t,
		"file1.txt", "index.html", "index.json", "caddy.json",
		"sub/file2.txt", "sub/index.html", "sub/index.json", "sub/caddy.json",
		"sub/deep/file3.txt", "sub/deep/index.html", "sub/deep/index.json", "sub/deep/caddy.json",
	)

	// Check the formats at the root
	assert.Equal(t, `[
{"Path":"sub","Name":"sub","Size":-1,"MimeType":"inode/directory","ModTime":"","IsDir":true},
{"Path":"file1.txt","Name":"file1.txt","Size":5,"MimeType":"text/plain; charset=utf-8","ModTime":"","IsDir":false}
]
`, r.read(t, "index.json"))
	assert.Equal(t, `[{"name":"sub/","size":4096,"url":"./sub/","mod_time":"0001-01-01T00:00:00Z","mode":2147484141,"is_dir":true,"is_symlink":false},{"name":"file1.txt","size":5,"url":"./file1.txt","mod_time":"0001-01-01T00:00:00Z","mode":420,"is_dir":false,"is_symlink":false}]
`, r.read(t, "caddy.json"))
	html := r.read(t, "index.html")
	assert.Contains(t, html, `<a href="sub/">sub/</a>`)
	assert.Contains(t, html, `<a href="file1.txt">file1.txt</a>`)
	assert.NotContains(t, html, "Go up")
	assert.Contains(t, html, "sortBy")
	assert.NotContains(t, html, "download=zip")

	// The listings never show the outputs and link up below the root
	html = r.read(t, "sub/index.html")
	assert.Contains(t, html, `<a href="deep/">deep/</a>`)
	assert.Contains(t, html, `<a href="file2.txt">file2.txt</a>`)
	assert.NotContains(t, html, "index.html")
	assert.Contains(t, html, "Go up")

	// The MIME type is set where the backend supports it
	r.checkMimeType(t, "index.html", "text/html; charset=utf-8")

	// A second run changes nothing
	transfers, deletes = r.index(t)
	assert.Equal(t, int64(0), transfers)
	assert.Equal(t, int64(0), deletes)

	// Unless it is told to rewrite everything
	r.opt.Rewrite = true
	transfers, _ = r.index(t)
	assert.Equal(t, int64(9), transfers)
	r.opt.Rewrite = false

	// Adding a file only rewrites the listing of its directory
	r.WriteObject(r.ctx, "sub/deep/file4.txt", "hello4", t1)
	transfers, deletes = r.index(t)
	assert.Equal(t, int64(3), transfers)
	assert.Equal(t, int64(0), deletes)
	assert.Contains(t, r.read(t, "sub/deep/index.html"), "file4.txt")

	// A run with --dry-run changes nothing
	ctx, ci := fs.AddConfig(r.ctx)
	ci.DryRun = true
	r.WriteObject(ctx, "sub/deep/file5.txt", "hello5", t1)
	require.NoError(t, operations.Index(ctx, r.Fremote, &r.opt))
	assert.NotContains(t, r.read(t, "sub/deep/index.html"), "file5.txt")
}

func TestIndexDelete(t *testing.T) {
	r := newIndexRun(t)
	r.index(t)
	canHaveEmptyDirectories := r.Fremote.Features().CanHaveEmptyDirectories

	// Remove the only file in sub/deep, leaving just the listing
	o, err := r.Fremote.NewObject(r.ctx, "sub/deep/file3.txt")
	require.NoError(t, err)
	require.NoError(t, o.Remove(r.ctx))

	transfers, deletes := r.index(t)
	if canHaveEmptyDirectories {
		// The empty directory still exists so keeps its listing
		assert.Equal(t, int64(1), transfers)
		assert.Equal(t, int64(0), deletes)
		r.checkFiles(t, "file1.txt", "index.html", "sub/file2.txt", "sub/index.html", "sub/deep/index.html")
		assert.Contains(t, r.read(t, "sub/index.html"), "deep/")
		assert.NotContains(t, r.read(t, "sub/deep/index.html"), "file3.txt")
	} else {
		// The directory only existed because of the listing, so it goes
		assert.Equal(t, int64(1), transfers)
		assert.Equal(t, int64(1), deletes)
		r.checkFiles(t, "file1.txt", "index.html", "sub/file2.txt", "sub/index.html")
		assert.NotContains(t, r.read(t, "sub/index.html"), "deep/")

		// Removing sub/file2.txt cascades: sub is now only listings
		o, err = r.Fremote.NewObject(r.ctx, "sub/file2.txt")
		require.NoError(t, err)
		require.NoError(t, o.Remove(r.ctx))
		transfers, deletes = r.index(t)
		assert.Equal(t, int64(1), transfers)
		assert.Equal(t, int64(1), deletes)
		r.checkFiles(t, "file1.txt", "index.html")
		assert.NotContains(t, r.read(t, "index.html"), "sub/")
	}
}

func TestIndexFilter(t *testing.T) {
	r := newIndexRun(t)
	r.WriteObject(r.ctx, "sub/deep/index.html", "hand written", t1)
	r.WriteObject(r.ctx, "other/notes.md", "notes", t1)

	// Excluded directories are not listed and not touched, and a
	// directory whose files are all excluded still gets an empty listing
	fi, err := filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, fi.AddRule("- /sub/deep/**"))
	require.NoError(t, fi.AddRule("- *.md"))
	r.ctx = filter.ReplaceConfig(r.ctx, fi)

	r.index(t)
	r.checkFiles(t, "file1.txt", "index.html", "sub/file2.txt", "sub/index.html", "sub/deep/file3.txt", "sub/deep/index.html", "other/notes.md", "other/index.html")
	assert.Equal(t, "hand written", r.read(t, "sub/deep/index.html"))
	assert.NotContains(t, r.read(t, "sub/index.html"), "deep/")
	other := r.read(t, "other/index.html")
	assert.NotContains(t, other, "notes.md")
	assert.Contains(t, r.read(t, "index.html"), `<a href="other/">other/</a>`)

	// A second run changes nothing, in particular it doesn't delete
	// other/index.html even though other/ has no visible content
	transfers, deletes := r.index(t)
	assert.Equal(t, int64(0), transfers)
	assert.Equal(t, int64(0), deletes)
}

func TestIndexRules(t *testing.T) {
	r := newIndexRun(t)
	r.WriteObject(r.ctx, "sub/deep/index.html", "hand written", t1)

	// A directory excluded by the index rules is still listed by
	// its parent but nothing is written or deleted in it
	r.opt.Rules.ExcludeRule = []string{"/sub/deep/**"}
	r.index(t)
	r.checkFiles(t, "file1.txt", "index.html", "sub/file2.txt", "sub/index.html", "sub/deep/file3.txt", "sub/deep/index.html")
	assert.Equal(t, "hand written", r.read(t, "sub/deep/index.html"))
	assert.Contains(t, r.read(t, "sub/index.html"), `<a href="deep/">deep/</a>`)

	// Include rules imply excluding everything else
	r.opt.Rules = filter.RulesOpt{IncludeRule: []string{"/sub/**"}}
	o, err := r.Fremote.NewObject(r.ctx, "index.html")
	require.NoError(t, err)
	require.NoError(t, o.Remove(r.ctx))
	transfers, _ := r.index(t)
	assert.Equal(t, int64(1), transfers) // sub/deep/index.html
	r.checkFiles(t, "file1.txt", "sub/file2.txt", "sub/index.html", "sub/deep/file3.txt", "sub/deep/index.html")
	assert.NotEqual(t, "hand written", r.read(t, "sub/deep/index.html"))
}

func TestIndexMaxDepth(t *testing.T) {
	r := newIndexRun(t)
	r.opt.MaxDepth = 1
	r.index(t)
	r.checkFiles(t, "file1.txt", "index.html", "sub/file2.txt", "sub/deep/file3.txt")
	assert.Contains(t, r.read(t, "index.html"), `<a href="sub/">sub/</a>`)
}

func TestIndexLinkIndex(t *testing.T) {
	r := newIndexRun(t)
	r.opt.LinkIndex = true
	r.index(t)
	assert.Contains(t, r.read(t, "index.html"), `<a href="sub/index.html">sub/</a>`)
	html := r.read(t, "sub/index.html")
	assert.Contains(t, html, `<a href="deep/index.html">deep/</a>`)
	assert.Contains(t, html, `<a href="../index.html">`)
}

// indexModTimes returns the ModTime of each entry in the json listing at remote
func (r *indexRun) indexModTimes(t *testing.T, remote string) map[string]time.Time {
	var items []struct {
		Name    string
		ModTime string
	}
	require.NoError(t, json.Unmarshal([]byte(r.read(t, remote)), &items))
	modTimes := map[string]time.Time{}
	for _, item := range items {
		if item.ModTime == "" {
			continue
		}
		modTime, err := time.Parse(time.RFC3339Nano, item.ModTime)
		require.NoError(t, err)
		modTimes[item.Name] = modTime
	}
	return modTimes
}

func TestIndexDirTime(t *testing.T) {
	r := newIndexRun(t)
	r.opt.NoModTime = false
	r.opt.Outputs = []string{"index.json=json"}
	precision := r.Fremote.Precision()

	// newest: sub shows the newest time below it, and so does its listing
	r.opt.DirTime = operations.IndexDirTimeNewest
	r.index(t)
	modTimes := r.indexModTimes(t, "index.json")
	fstest.AssertTimeEqualWithPrecision(t, "file1.txt", t1, modTimes["file1.txt"], precision)
	fstest.AssertTimeEqualWithPrecision(t, "sub", t3, modTimes["sub"], precision)
	o, err := r.Fremote.NewObject(r.ctx, "index.json")
	require.NoError(t, err)
	fstest.AssertTimeEqualWithPrecision(t, "index.json", t3, o.ModTime(r.ctx), precision)

	// none: no directory times
	r.opt.DirTime = operations.IndexDirTimeNone
	r.index(t)
	modTimes = r.indexModTimes(t, "index.json")
	fstest.AssertTimeEqualWithPrecision(t, "file1.txt", t1, modTimes["file1.txt"], precision)
	_, found := modTimes["sub"]
	assert.False(t, found)

	// no modtime: no times at all
	r.opt.NoModTime = true
	r.index(t)
	assert.Empty(t, r.indexModTimes(t, "index.json"))
}

func TestIndexTemplate(t *testing.T) {
	r := newIndexRun(t)
	tmpl := filepath.Join(t.TempDir(), "list.tmpl")
	require.NoError(t, os.WriteFile(tmpl, []byte("{{.Path}}\n{{range .Entries}}{{.Leaf}} {{.MimeType}}\n{{end}}"), 0600))
	r.opt.Outputs = []string{"list.txt=" + tmpl}
	r.index(t)
	assert.Equal(t, "/sub/\ndeep/ inode/directory\nfile2.txt text/plain; charset=utf-8\n", r.read(t, "sub/list.txt"))
	r.checkMimeType(t, "sub/list.txt", "text/plain; charset=utf-8")

	// Bad outputs
	for _, output := range []string{"index.html", "=html", "a/b=html", "index.html=nosuchformat"} {
		r.opt.Outputs = []string{output}
		assert.Error(t, operations.Index(r.ctx, r.Fremote, &r.opt), output)
	}
}

func TestIndexTemplateBuiltin(t *testing.T) {
	for format := range map[string]bool{"html": true, "json": true, "caddy": true} {
		text, err := operations.IndexTemplate(format)
		require.NoError(t, err)
		assert.NotEmpty(t, text)
	}
	_, err := operations.IndexTemplate("nosuchformat")
	assert.Error(t, err)
}

func TestRcIndex(t *testing.T) {
	r, call := rcNewRun(t, "operations/index")
	r.WriteObject(context.Background(), "sub/a", "a", t1)
	in := rc.Params{
		"fs": r.FremoteName,
		"opt": rc.Params{
			"output":    []string{"index.json=json"},
			"noModTime": true,
			"maxDepth":  1,
		},
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Nil(t, out)
	_, err = r.Fremote.NewObject(context.Background(), "index.json")
	assert.NoError(t, err)
	_, err = r.Fremote.NewObject(context.Background(), "sub/index.json")
	assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
}

// TestIndexNoHash checks the listings are read and compared on a
// backend without hashes so that unchanged listings aren't rewritten
func TestIndexNoHash(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping test on non local remote")
	}
	opt := operations.IndexOptDefault
	opt.NoModTime = true
	r := &indexRun{Run: fstest.NewRun(t), ctx: context.Background(), opt: opt}
	f, err := fs.NewFs(r.ctx, ":crypt,remote='"+r.Fremote.Root()+"',password="+obscure.MustObscure("potato")+":")
	require.NoError(t, err)
	require.Equal(t, 0, f.Hashes().Count())
	r.Fremote = f
	r.writeFiles()

	transfers, _ := r.index(t)
	assert.Equal(t, int64(3), transfers)
	transfers, _ = r.index(t)
	assert.Equal(t, int64(0), transfers)
	r.WriteObject(r.ctx, "sub/file4.txt", "hello4", t1)
	transfers, _ = r.index(t)
	assert.Equal(t, int64(1), transfers)
	assert.Contains(t, r.read(t, "sub/index.html"), "file4.txt")
}
