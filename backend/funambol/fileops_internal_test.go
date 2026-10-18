package funambol

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/funambol/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fileOpsFixture struct {
	f            *Fs
	items        map[string]api.Item
	listed       []api.Item
	requests     []string
	deleted      []string
	failures     []string
	copyName     string
	copyID       string
	moveName     string
	moveID       string
	pending      bool
	legacyParent bool
}

func newFileOpsFixture(t *testing.T) *fileOpsFixture {
	t.Helper()
	s := &fileOpsFixture{items: map[string]api.Item{}}
	s.items["10"] = api.Item{ID: "10", Name: "source.bin", FolderID: "1", MediaType: "file", Status: "U", URL: "https://example.invalid/content", Size: 17, ModificationDate: time.Date(2026, 10, 18, 12, 34, 56, 0, time.UTC).UnixMilli()}
	decode := func(r *http.Request) map[string]any {
		t.Helper()
		var envelope struct{ Data map[string]any }
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			require.NoError(t, r.ParseForm())
			require.NoError(t, json.Unmarshal([]byte(r.PostForm.Get("data")), &envelope))
		} else {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&envelope))
		}
		return envelope.Data
	}
	reply := func(w http.ResponseWriter, v any) {
		t.Helper()
		require.NoError(t, json.NewEncoder(w).Encode(v))
	}
	fail := func(w http.ResponseWriter) bool {
		if len(s.failures) == 0 {
			return false
		}
		failure := s.failures[0]
		s.failures = s.failures[1:]
		if failure == "503" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":{"code":"MED-1000","message":"busy"}}`)
		} else {
			_, _ = fmt.Fprintf(w, `{"error":{"code":%q,"message":"retry","data":"new-key"}}`, failure)
		}
		return true
	}
	s.f, _ = newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media/folder": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"data":{"folders":[]}}`)
		},
		"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
			d := decode(r)
			if r.URL.Query().Get("action") == "delete" {
				for _, id := range d["ids"].([]any) {
					key := fmt.Sprint(id)
					s.deleted = append(s.deleted, key)
					delete(s.items, key)
				}
				reply(w, map[string]any{})
				return
			}
			items := []api.Item{}
			for _, item := range s.listed {
				if item.FolderID.String() == r.URL.Query().Get("folderid") {
					items = append(items, item)
				}
			}
			if ids, ok := d["ids"].([]any); ok {
				items = nil
				for _, id := range ids {
					if item, ok := s.items[fmt.Sprint(id)]; ok {
						if s.pending {
							item.Status, item.URL = "P", ""
						}
						items = append(items, item)
					}
				}
			}
			if items == nil {
				items = []api.Item{}
			}
			if s.legacyParent {
				encoded, err := json.Marshal(map[string]any{"data": map[string]any{"media": items}})
				require.NoError(t, err)
				_, _ = io.WriteString(w, strings.ReplaceAll(string(encoded), `"folderid":`, `"folder":`))
				return
			}
			reply(w, map[string]any{"data": map[string]any{"media": items}})
		},
		"/sapi/media/file": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "copy", r.URL.Query().Get("action"))
			d := decode(r)
			assert.IsType(t, float64(0), d["id"])
			assert.IsType(t, float64(0), d["parentid"])
			s.requests = append(s.requests, "copy")
			if fail(w) {
				return
			}
			item := s.items[fmt.Sprint(d["id"])]
			item.ID, item.Name, item.FolderID = "30", d["name"].(string), api.ID(fmt.Sprint(d["parentid"]))
			if s.copyName != "" {
				item.Name = s.copyName
			}
			if s.copyID != "" {
				item.ID = api.ID(s.copyID)
			}
			s.items[item.ID.String()] = item
			reply(w, map[string]any{"data": map[string]any{"id": item.ID, "name": item.Name, "success": "Media copied successfully"}})
		},
		"/sapi/upload/file": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "save-metadata", r.URL.Query().Get("action"))
			assert.Contains(t, r.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
			d := decode(r)
			assert.IsType(t, "", d["id"])
			assert.IsType(t, float64(0), d["folderid"])
			assert.Equal(t, "20261018T123456Z", d["modificationdate"])
			s.requests = append(s.requests, "move")
			if fail(w) {
				return
			}
			item := s.items[d["id"].(string)]
			item.Name, item.FolderID = d["name"].(string), api.ID(fmt.Sprint(d["folderid"]))
			if s.moveName != "" {
				item.Name = s.moveName
			}
			s.items[item.ID.String()] = item
			id := item.ID.String()
			if s.moveID != "" {
				id = s.moveID
			}
			reply(w, map[string]any{"id": id})
		},
	})
	s.f.dirCache.Put("dest", "2")
	return s
}

func (s *fileOpsFixture) source(t *testing.T) fs.Object {
	t.Helper()
	item := s.items["10"]
	o, err := s.f.newObjectWithInfo(context.Background(), item.Name, &item)
	require.NoError(t, err)
	return o
}

func TestFileOperations(t *testing.T) {
	for _, move := range []bool{false, true} {
		t.Run(fmt.Sprintf("move=%v", move), func(t *testing.T) {
			s := newFileOpsFixture(t)
			require.NotNil(t, s.f.Features().Copy)
			require.NotNil(t, s.f.Features().Move)
			src := s.source(t)
			original := s.items["10"]
			name := "dest/copy.dat"
			do := s.f.Copy
			wantID := "30"
			if move {
				name, do, wantID = "dest/moved space-ñ.bin", s.f.Move, "10"
			}
			dst, err := do(context.Background(), src, name)
			require.NoError(t, err)
			assert.Equal(t, name, dst.Remote())
			assert.Equal(t, wantID, dst.(*Object).id)
			assert.Equal(t, src.Size(), dst.Size())
			assert.Equal(t, src.ModTime(context.Background()), dst.ModTime(context.Background()))
			assert.Empty(t, s.deleted)
			// The server still lists the old source, but no destination.
			s.listed = []api.Item{original}
			found, err := s.f.NewObject(context.Background(), name)
			require.NoError(t, err)
			assert.Equal(t, wantID, found.(*Object).id)
			entries, err := s.f.List(context.Background(), "dest")
			require.NoError(t, err)
			assert.Len(t, entries, 1)
			if move {
				_, err = s.f.NewObject(context.Background(), "source.bin")
				assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
				entries, err = s.f.List(context.Background(), "")
				require.NoError(t, err)
				assert.Empty(t, entries)
			}
		})
	}
}

func TestFileOperationRetriesReplayBody(t *testing.T) {
	for _, move := range []bool{false, true} {
		t.Run(fmt.Sprintf("move=%v", move), func(t *testing.T) {
			s := newFileOpsFixture(t)
			s.failures = []string{"SEC-1003", "503"}
			do := s.f.Copy
			if move {
				do = s.f.Move
			}
			_, err := do(context.Background(), s.source(t), "dest/new.bin")
			require.NoError(t, err)
			assert.Len(t, s.requests, 3)
		})
	}
}

func TestFileMoveExtensionFallback(t *testing.T) {
	s := newFileOpsFixture(t)
	src := s.source(t)
	_, err := s.f.Move(context.Background(), src, "dest/file.rclone_chunk.001")
	require.ErrorIs(t, err, fs.ErrorCantMove)
	assert.Empty(t, s.requests)
	dst, err := operations.Move(context.Background(), s.f, nil, "dest/file.rclone_chunk.001", src)
	require.NoError(t, err)
	assert.Equal(t, "30", dst.(*Object).id)
	assert.Equal(t, []string{"copy"}, s.requests)
	assert.Equal(t, []string{"10"}, s.deleted)
}

func TestFileMoveExtensionFreeChunk(t *testing.T) {
	s := newFileOpsFixture(t)
	item := s.items["10"]
	item.Name = "encrypted__chunk_001_txn"
	s.items["10"] = item
	dst, err := s.f.Move(context.Background(), s.source(t), "dest/encrypted__chunk_001")
	require.NoError(t, err)
	assert.Equal(t, "10", dst.(*Object).id)
	assert.Equal(t, []string{"move"}, s.requests)
	assert.Empty(t, s.deleted)
}

func TestFileOperationOverwrite(t *testing.T) {
	for _, move := range []bool{false, true} {
		t.Run(fmt.Sprintf("move=%v", move), func(t *testing.T) {
			s := newFileOpsFixture(t)
			old := s.items["10"]
			old.ID, old.Name, old.FolderID = "20", "target.bin", "2"
			s.items["20"] = old
			s.listed = []api.Item{old}
			s.f.uploads.put("dest/target.bin", old)
			do := s.f.Copy
			if move {
				do = s.f.Move
			}
			_, err := do(context.Background(), s.source(t), "dest/target.bin")
			require.NoError(t, err)
			assert.Equal(t, []string{"20"}, s.deleted)
		})
	}
}

func TestFileOperationGuards(t *testing.T) {
	for _, mode := range []string{"other-backend", "other-config", "other-account", "other-endpoint", "other-media-type", "self-copy", "bad-id", "bad-copy-name", "same-copy-id", "bad-move-id"} {
		t.Run(mode, func(t *testing.T) {
			s := newFileOpsFixture(t)
			src := s.source(t)
			do, remote := s.f.Copy, "dest/copy.bin"
			switch mode {
			case "other-backend":
				src = object.NewMemoryObject("source.bin", time.Now(), []byte("content"))
			case "other-config", "other-account", "other-endpoint":
				other := *s.f
				if mode == "other-config" {
					other.name = "another-config"
				} else if mode == "other-account" {
					other.rootID = "999"
				} else {
					other.opt.Endpoint = "https://another.invalid"
				}
				src.(*Object).fs = &other
			case "other-media-type":
				item := s.items["10"]
				item.MediaType = "video"
				s.items["10"] = item
			case "self-copy":
				remote = "source.bin"
			case "bad-id":
				src.(*Object).id = "not-numeric"
			case "bad-copy-name":
				s.copyName = "copy (1).bin"
			case "same-copy-id":
				s.copyID = "10"
			case "bad-move-id":
				do, s.moveID = s.f.Move, "99"
			}
			_, err := do(context.Background(), src, remote)
			require.Error(t, err)
			assert.Empty(t, s.deleted, "a failed copy must not delete its source")
		})
	}
}

func TestFileOperationValidationIsBounded(t *testing.T) {
	s := newFileOpsFixture(t)
	s.pending = true
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := s.f.Copy(ctx, s.source(t), "dest/copy.bin")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Empty(t, s.requests)
	assert.Empty(t, s.deleted)
}

func TestFileOperationLegacyParent(t *testing.T) {
	s := newFileOpsFixture(t)
	s.legacyParent = true
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := s.f.Copy(ctx, s.source(t), "dest/copy.bin")
	require.NoError(t, err)
}

func TestFileOperationValidationRace(t *testing.T) {
	s := newFileOpsFixture(t)
	s.failures = []string{"MED-1017"}
	_, err := s.f.Move(context.Background(), s.source(t), "dest/ready.bin")
	require.NoError(t, err)
	assert.Equal(t, []string{"move", "move"}, s.requests)
}

func TestFileOperationFailureKeepsSource(t *testing.T) {
	s := newFileOpsFixture(t)
	s.failures = []string{"MED-1001"}
	_, err := operations.Move(context.Background(), s.f, nil, "dest/changed.dat", s.source(t))
	require.Error(t, err)
	assert.Empty(t, s.deleted)
	assert.Equal(t, "source.bin", s.items["10"].Name)
}

func TestRecentFileListingExpires(t *testing.T) {
	s := newFileOpsFixture(t)
	ctx := context.Background()
	original := s.items["10"]
	s.listed = []api.Item{original}
	s.f.uploads.remove("source.bin")
	entries, err := s.f.List(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, entries)
	s.f.uploads.mu.Lock()
	s.f.uploads.items["source.bin"] = uploadEntry{at: time.Now().Add(-2 * recentUploadTTL)}
	s.f.uploads.mu.Unlock()
	entries, err = s.f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "source.bin", entries[0].Remote())
}

func TestOpenWaitsForDownloadURL(t *testing.T) {
	var reads atomic.Int32
	f, _ := newSAPIServer(t, map[string]http.HandlerFunc{
		"/sapi/media": func(w http.ResponseWriter, r *http.Request) {
			if reads.Add(1) == 1 {
				_, _ = io.WriteString(w, `{"data":{"media":[{"id":10}]}}`)
				return
			}
			_, _ = fmt.Fprintf(w, `{"data":{"media":[{"id":10,"url":"http://%s/content"}]}}`, r.Host)
		},
		"/content": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "chunker metadata")
		},
	})
	o := &Object{fs: f, remote: "meta", id: "10", hasMetaData: true, size: 16}
	r, err := o.Open(context.Background())
	require.NoError(t, err)
	data, err := io.ReadAll(r)
	require.NoError(t, r.Close())
	require.NoError(t, err)
	assert.Equal(t, "chunker metadata", string(data))
	assert.EqualValues(t, 2, reads.Load())
}

func TestDownloadURLWaitIsBounded(t *testing.T) {
	s := newFileOpsFixture(t)
	s.pending = true
	o := s.source(t).(*Object)
	o.url = ""
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := o.Open(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
