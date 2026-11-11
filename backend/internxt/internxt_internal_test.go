package internxt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/internxt/rclone-adapter/config"
	"github.com/internxt/rclone-adapter/endpoints"
	"github.com/internxt/rclone-adapter/folders"
	"github.com/rclone/rclone/fs"
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

// lagServer wraps existenceServer so the first lag existence checks find
// nothing, as when the replicas haven't caught up with an upload.
func lagServer(t *testing.T, stored []storedFile, lag int32) (server *httptest.Server, calls *atomic.Int32) {
	t.Helper()
	inner := existenceServer(t, stored)
	t.Cleanup(inner.Close)
	calls = new(atomic.Int32)
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/meta") && calls.Add(1) <= lag {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"existentFiles": []any{}}))
			return
		}
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func TestWaitForFile(t *testing.T) {
	ctx := context.Background()
	stored := []storedFile{{"uuid-new", "foo", "txt"}}

	t.Run("Visible", func(t *testing.T) {
		server, calls := lagServer(t, stored, 0)
		file := testFs(t, server).waitForFile(ctx, "foo.txt", "dir-uuid", "uuid-new")
		require.NotNil(t, file)
		assert.Equal(t, "uuid-new", file.UUID)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("VisibleAfterLag", func(t *testing.T) {
		server, calls := lagServer(t, stored, 3)
		file := testFs(t, server).waitForFile(ctx, "foo.txt", "dir-uuid", "uuid-new")
		require.NotNil(t, file)
		assert.Equal(t, "uuid-new", file.UUID)
		assert.Equal(t, int32(4), calls.Load())
	})

	t.Run("NeverVisible", func(t *testing.T) {
		server, calls := lagServer(t, stored, 1000)
		f := testFs(t, server)
		f.pacer.SetRetries(3)
		assert.Nil(t, f.waitForFile(ctx, "foo.txt", "dir-uuid", "uuid-new"))
		assert.Equal(t, int32(3), calls.Load(), "gives up after the low level retries")
	})

	t.Run("OtherFileOfSameName", func(t *testing.T) {
		// The replicas still show the file we just replaced
		server, _ := lagServer(t, []storedFile{{"uuid-old", "foo", "txt"}}, 0)
		f := testFs(t, server)
		f.pacer.SetRetries(2)
		assert.Nil(t, f.waitForFile(ctx, "foo.txt", "dir-uuid", "uuid-new"))
	})
}

func TestGetFile(t *testing.T) {
	ctx := context.Background()
	server := existenceServer(t, []storedFile{{"uuid-1", "foo", "txt"}})
	defer server.Close()
	f := testFs(t, server)

	file, err := f.getFile(ctx, "uuid-1")
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Equal(t, "uuid-1", file.UUID)

	_, err = f.getFile(ctx, "uuid-missing")
	assert.Error(t, err, "a file the replicas don't have yet")
}

func TestObjectSetMetaData(t *testing.T) {
	modTime := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	o := &Object{size: -1, modTime: time.Unix(1, 0), uuid: "old-uuid", id: "old-id"}
	o.setMetaData(&folders.File{UUID: "uuid", FileID: "id", Size: "42", ModificationTime: modTime})
	assert.Equal(t, "uuid", o.uuid)
	assert.Equal(t, "id", o.id)
	assert.Equal(t, int64(42), o.size)
	assert.True(t, modTime.Equal(o.modTime), "modification time is replaced, not left as the old file's")
}
