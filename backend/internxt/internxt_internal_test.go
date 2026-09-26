package internxt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/internxt/rclone-adapter/config"
	"github.com/internxt/rclone-adapter/endpoints"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storedFile is a file as the service holds it: a (plainName, type) pair
// split however the client that uploaded it chose to split it.
type storedFile struct {
	uuid      string
	plainName string
	fileType  string
}

// wireType renders a stored type the way the service does: a file with no
// extension comes back as JSON null, not an empty string.
func wireType(fileType string) any {
	if fileType == "" {
		return nil
	}
	return fileType
}

// existenceServer answers the existence check and file meta endpoints from
// files, matching plainName exactly and only constraining type when the
// criterion supplies one, as the service does.
func existenceServer(t *testing.T, stored []storedFile) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/meta") {
			uuid := strings.Split(strings.TrimPrefix(r.URL.Path, "/drive/files/"), "/")[0]
			for _, f := range stored {
				if f.uuid == uuid {
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"uuid": f.uuid, "plainName": f.plainName, "type": wireType(f.fileType),
					}))
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			Files []struct {
				PlainName string `json:"plainName"`
				Type      string `json:"type"`
			} `json:"files"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		var existent []map[string]any
		for _, f := range stored {
			for _, c := range req.Files {
				if c.PlainName == f.plainName && (c.Type == "" || c.Type == f.fileType) {
					existent = append(existent, map[string]any{
						"exists": true, "status": "EXISTS",
						"uuid": f.uuid, "plainName": f.plainName, "type": wireType(f.fileType),
					})
					break
				}
			}
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"existentFiles": existent}))
	}))
}

func testFs(t *testing.T, server *httptest.Server) *Fs {
	t.Helper()
	f := &Fs{
		opt: Options{Encoding: encoder.EncodeInvalidUtf8},
		cfg: &config.Config{
			HTTPClient: server.Client(),
			Endpoints:  endpoints.NewConfig(server.URL),
		},
	}
	f.pacer = fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))
	return f
}

// storedFolder is the content of a folder as listServer serves it.
type storedFolder struct {
	folders []string
	files   []string
}

// listServer serves the cursor paginated folder listings and the folder
// existence check from a tree keyed by folder UUID. Items are named
// "<folderUUID>-<kind>-<index>" and the cursor is the index of the next page.
type listServer struct {
	*httptest.Server
	mu      sync.Mutex
	cursors map[string][]string // cursor of each request keyed by "<folderUUID>/<kind>"
	// fail, if set, may write a response instead and return true
	fail func(w http.ResponseWriter, kind, cursor string) bool
}

// listPageSize is the page size listServer serves.
const listPageSize = 2

// newListFs returns an Fs with its root at "root-uuid" served by a listServer
// holding tree.
func newListFs(t *testing.T, tree map[string]storedFolder) (*Fs, *listServer) {
	t.Helper()
	s := &listServer{cursors: map[string][]string{}}
	// record logs the request and reports whether fail handled it
	record := func(w http.ResponseWriter, uuid, kind, cursor string) bool {
		s.mu.Lock()
		s.cursors[uuid+"/"+kind] = append(s.cursors[uuid+"/"+kind], cursor)
		s.mu.Unlock()
		return s.fail != nil && s.fail(w, kind, cursor)
	}
	item := func(uuid, kind string, i int, name string) map[string]any {
		return map[string]any{"uuid": fmt.Sprintf("%s-%s-%d", uuid, kind, i), "plainName": name, "status": "EXISTS"}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /drive/folders/v2/content/{uuid}/{kind}", func(w http.ResponseWriter, r *http.Request) {
		uuid, kind, cursor := r.PathValue("uuid"), r.PathValue("kind"), r.URL.Query().Get("cursor")
		if record(w, uuid, kind, cursor) {
			return
		}
		names := tree[uuid].folders
		if kind == "files" {
			names = tree[uuid].files
		}
		start := 0
		if cursor != "" {
			var err error
			start, err = strconv.Atoi(cursor)
			assert.NoError(t, err)
		}
		end := min(start+listPageSize, len(names))
		items := []map[string]any{}
		for i := start; i < end; i++ {
			items = append(items, item(uuid, kind, i, names[i]))
		}
		var next any
		if end < len(names) {
			next = strconv.Itoa(end)
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{kind: items, "nextCursor": next}))
	})
	mux.HandleFunc("POST /drive/folders/content/{uuid}/folders/existence", func(w http.ResponseWriter, r *http.Request) {
		uuid := r.PathValue("uuid")
		if record(w, uuid, "existence", "") {
			return
		}
		var body struct {
			PlainNames []string `json:"plainNames"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		existent := []map[string]any{}
		for i, name := range tree[uuid].folders {
			if slices.Contains(body.PlainNames, name) {
				existent = append(existent, item(uuid, "folders", i, name))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"existentFolders": existent}))
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)

	f := testFs(t, s.Server)
	f.dirCache = dircache.New("", "root-uuid", f)
	require.NoError(t, f.dirCache.FindRoot(context.Background(), false))
	return f, s
}

// requests returns the cursor of each request made for kind in folderUUID.
func (s *listServer) requests(folderUUID, kind string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursors[folderUUID+"/"+kind]
}

// names returns n names made from prefix and an index.
func names(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return out
}

func TestSplitJoinNameExt(t *testing.T) {
	for _, test := range []struct {
		baseName string
		name     string
		ext      string
	}{
		{"foo.txt", "foo", "txt"},
		{"foo.tar.gz", "foo.tar", "gz"},
		{"README", "README", ""},
		{".bashrc", ".bashrc", ""},
		{"..hidden", ".", "hidden"},
		{"", "", ""},
	} {
		name, ext := splitNameExt(test.baseName)
		assert.Equal(t, test.name, name, test.baseName)
		assert.Equal(t, test.ext, ext, test.baseName)
		assert.Equal(t, test.baseName, joinNameExt(name, ext), test.baseName)
	}
}

func TestFindFile(t *testing.T) {
	for _, test := range []struct {
		what     string
		stored   []storedFile
		leaf     string
		wantUUID string
	}{{
		what:     "split as rclone splits it",
		stored:   []storedFile{{"uuid-1", "foo.tar", "gz"}},
		leaf:     "foo.tar.gz",
		wantUUID: "uuid-1",
	}, {
		what:     "whole name stored as plainName by another client",
		stored:   []storedFile{{"uuid-2", "foo.tar.gz", ""}},
		leaf:     "foo.tar.gz",
		wantUUID: "uuid-2",
	}, {
		what:     "extensionless name is not confused with an extended one",
		stored:   []storedFile{{"uuid-3", "README", "md"}, {"uuid-4", "README", ""}},
		leaf:     "README",
		wantUUID: "uuid-4",
	}, {
		what:     "dotfile stored the way the web client splits it",
		stored:   []storedFile{{"uuid-6", ".bashrc", ""}},
		leaf:     ".bashrc",
		wantUUID: "uuid-6",
	}, {
		what:     "dotfile stored the way older rclone versions split it",
		stored:   []storedFile{{"uuid-7", "", "bashrc"}},
		leaf:     ".bashrc",
		wantUUID: "uuid-7",
	}, {
		what:     "trailing dot kept in plainName by a rename",
		stored:   []storedFile{{"uuid-8", "trailing.", ""}},
		leaf:     "trailing.",
		wantUUID: "uuid-8",
	}, {
		what:   "missing file",
		stored: []storedFile{{"uuid-5", "other", "txt"}},
		leaf:   "foo.txt",
	}} {
		t.Run(test.what, func(t *testing.T) {
			server := existenceServer(t, test.stored)
			defer server.Close()
			file, err := testFs(t, server).findFile(context.Background(), test.leaf, "dir-uuid")
			require.NoError(t, err)
			if test.wantUUID == "" {
				assert.Nil(t, file)
				return
			}
			require.NotNil(t, file)
			assert.Equal(t, test.wantUUID, file.UUID)
		})
	}
}

func TestFindFileIgnoresStale(t *testing.T) {
	ctx := context.Background()
	server := existenceServer(t, []storedFile{{"uuid-moved", "foo", "txt"}, {"uuid-deleted", "bar", "txt"}, {"uuid-renamed", "baz", "txt"}})
	defer server.Close()
	f := testFs(t, server)

	recent.moved("uuid-moved", "other-dir-uuid", "foo.txt.bak")
	recent.deleted("uuid-deleted")
	recent.moved("uuid-renamed", "dir-uuid", "baz.txt")

	file, err := f.findFile(ctx, "foo.txt", "dir-uuid")
	require.NoError(t, err)
	assert.Nil(t, file, "moved away")

	file, err = f.findFile(ctx, "bar.txt", "dir-uuid")
	require.NoError(t, err)
	assert.Nil(t, file, "deleted")

	file, err = f.findFile(ctx, "baz.txt", "dir-uuid")
	require.NoError(t, err)
	require.NotNil(t, file, "moved here")
	assert.Equal(t, "uuid-renamed", file.UUID)
}

func TestListP(t *testing.T) {
	f, _ := newListFs(t, map[string]storedFolder{
		"root-uuid": {folders: names("dir", 3), files: names("file", 5)},
	})

	var entries fs.DirEntries
	require.NoError(t, f.ListP(context.Background(), "", func(page fs.DirEntries) error {
		entries = append(entries, page...)
		return nil
	}))
	dirs, objs := 0, 0
	entries.ForDir(func(fs.Directory) { dirs++ })
	entries.ForObject(func(fs.Object) { objs++ })
	assert.Equal(t, 3, dirs)
	assert.Equal(t, 5, objs)

	id, ok := f.dirCache.Get("dir2")
	assert.True(t, ok, "listed directory cached")
	assert.Equal(t, "root-uuid-folders-2", id)
}

func TestListPSkipsStaleDirs(t *testing.T) {
	f, _ := newListFs(t, map[string]storedFolder{
		"stale-uuid": {folders: names("dir", 2)},
	})
	f.dirCache.Put("stale", "stale-uuid")
	recent.moved("stale-uuid-folders-0", "other-dir-uuid", "dir0")

	entries, err := f.List(context.Background(), "stale")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "stale/dir1", entries[0].Remote())

	_, ok := f.dirCache.Get("stale/dir0")
	assert.False(t, ok, "stale directory cached")
}

func TestListPRetriesOnlyFailedPage(t *testing.T) {
	f, server := newListFs(t, map[string]storedFolder{
		"root-uuid": {files: names("file", 5)},
	})
	failed := false
	server.fail = func(w http.ResponseWriter, kind, cursor string) bool {
		if cursor == "2" && !failed {
			failed = true
			w.WriteHeader(http.StatusTooManyRequests)
			return true
		}
		return false
	}

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Len(t, entries, 5)
	assert.Equal(t, []string{"", "2", "2", "4"}, server.requests("root-uuid", "files"))
}

func TestListPRepeatedCursor(t *testing.T) {
	f, server := newListFs(t, map[string]storedFolder{
		"root-uuid": {folders: names("dir", 5)},
	})
	server.fail = func(w http.ResponseWriter, kind, cursor string) bool {
		if cursor == "2" {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"folders": []any{}, "nextCursor": "2"}))
			return true
		}
		return false
	}

	_, err := f.List(context.Background(), "")
	assert.ErrorContains(t, err, "already visited list cursor")
}

func TestRmdirStopsAtFirstChild(t *testing.T) {
	ctx := context.Background()
	f, server := newListFs(t, map[string]storedFolder{
		"dir-uuid":   {folders: names("dir", 5)},
		"files-uuid": {files: names("file", 5)},
	})
	f.dirCache.Put("dir", "dir-uuid")
	f.dirCache.Put("files", "files-uuid")

	assert.ErrorIs(t, f.Rmdir(ctx, "dir"), fs.ErrorDirectoryNotEmpty)
	assert.Equal(t, []string{""}, server.requests("dir-uuid", "folders"))
	assert.Empty(t, server.requests("dir-uuid", "files"))

	assert.ErrorIs(t, f.Rmdir(ctx, "files"), fs.ErrorDirectoryNotEmpty)
	assert.Equal(t, []string{""}, server.requests("files-uuid", "folders"))
	assert.Equal(t, []string{""}, server.requests("files-uuid", "files"))
}

func TestFindLeaf(t *testing.T) {
	ctx := context.Background()
	f, server := newListFs(t, map[string]storedFolder{
		"find-uuid": {folders: names("dir", 3)},
	})
	recent.moved("find-uuid-folders-1", "other-dir-uuid", "dir1")

	id, found, err := f.FindLeaf(ctx, "find-uuid", "dir2")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "find-uuid-folders-2", id)

	_, found, err = f.FindLeaf(ctx, "find-uuid", "missing")
	require.NoError(t, err)
	assert.False(t, found)

	_, found, err = f.FindLeaf(ctx, "find-uuid", "dir1")
	require.NoError(t, err)
	assert.False(t, found, "moved away")

	assert.Empty(t, server.requests("find-uuid", "folders"), "listed folders")
}

func TestFindLeafFallsBackToListing(t *testing.T) {
	ctx := context.Background()
	f, server := newListFs(t, map[string]storedFolder{
		"root-uuid": {folders: names("dir", 5)},
	})
	// the read replica behind the existence check hasn't seen the parent yet
	server.fail = func(w http.ResponseWriter, kind, cursor string) bool {
		if kind == "existence" {
			w.WriteHeader(http.StatusBadRequest)
			return true
		}
		return false
	}

	id, found, err := f.FindLeaf(ctx, "root-uuid", "dir1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "root-uuid-folders-1", id)
	assert.Equal(t, []string{""}, server.requests("root-uuid", "folders"), "stops at the page with the match")

	id, found, err = f.FindLeaf(ctx, "root-uuid", "dir4")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "root-uuid-folders-4", id)

	_, found, err = f.FindLeaf(ctx, "root-uuid", "missing")
	require.NoError(t, err)
	assert.False(t, found)
}
