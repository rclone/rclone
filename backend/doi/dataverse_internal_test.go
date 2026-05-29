// Internal tests for the Dataverse direct mode of the doi backend.
//
// These exercise the Dataverse-specific paths (host + dataset_pid direct
// addressing, token auth + cross-host strip, ingest_format, the
// no-probe/resume read path, and the lazy /tree listing) against
// httptest servers that mimic a Dataverse installation.

package doi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/doi/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
)

// dvFixtureServer wires an httptest.Server that mimics a Dataverse
// installation: a versions/:latest endpoint returning the given file
// list, and an access/datafile/{id} endpoint that 302s to a presigned
// URL on the same server (so the byte stream stays local). It exposes no
// /tree endpoint, so feature detection falls back to the whole-version
// listing.
func dvFixtureServer(t *testing.T, dsPID string, files []api.DataverseFile, contents map[int64][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(api.AuthHeader) != "secret" {
			http.Error(w, `{"status":"ERROR","message":"bad token"}`, http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("persistentId") != dsPID {
			http.Error(w, `{"status":"ERROR","message":"unknown dataset"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DataverseVersionResponse{
			Status: "OK",
			Data:   api.DataverseDatasetVersion{LastUpdateTime: "2026-05-01T00:00:00Z", Files: files},
		})
	})

	// The "presigned" URL — no auth required, supports Range.
	mux.HandleFunc("/presigned/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/presigned/")
		var id int64
		_, _ = fmt.Sscanf(idStr, "%d", &id)
		data, ok := contents[id]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	})

	// Access endpoint: 302 to the presigned URL above.
	mux.HandleFunc("/api/access/datafile/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(api.AuthHeader) != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		idStr := strings.TrimPrefix(r.URL.Path, "/api/access/datafile/")
		var id int64
		_, _ = fmt.Sscanf(idStr, "%d", &id)
		if _, ok := contents[id]; !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		host := "http://" + r.Host
		w.Header().Set("Location", host+"/presigned/"+idStr+"?X-Amz-Expires=3600&X-Amz-Signature=fake")
		w.WriteHeader(http.StatusFound)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// newDvTestFs builds a doi Fs in Dataverse direct mode against srvURL,
// with token "secret" and the given dataset PID. root is forwarded as-is.
func newDvTestFs(t *testing.T, srvURL, dsPID, root string) *Fs {
	t.Helper()
	got, err := NewFs(context.Background(), "test", root, configmap.Simple{
		"type":        "doi",
		"host":        srvURL,
		"token":       "secret",
		"dataset_pid": dsPID,
	})
	if err != nil && err != fs.ErrorIsFile {
		t.Fatalf("NewFs: %v", err)
	}
	f, ok := got.(*Fs)
	if !ok {
		t.Fatalf("NewFs returned %T", got)
	}
	return f
}

func remoteSet(entries fs.DirEntries) map[string]bool {
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Remote()] = true
	}
	return out
}

func equalSets(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func mustList(t *testing.T, f *Fs, dir string) fs.DirEntries {
	t.Helper()
	entries, err := f.List(context.Background(), dir)
	if err != nil {
		t.Fatalf("List(%q): %v", dir, err)
	}
	return entries
}

const testPID = "doi:10.5072/FK2/ABCD"

func TestDataverseDirectListsRoot(t *testing.T) {
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 1, Filename: "readme.txt", FileSize: 11, MD5: "md5-1"}},
		{DirectoryLabel: "papers/2026", DataFile: api.DataverseDataFile{ID: 2, Filename: "report.pdf", FileSize: 100, MD5: "md5-2"}},
		{DirectoryLabel: "papers/2026/figures", DataFile: api.DataverseDataFile{ID: 3, Filename: "fig1.png", FileSize: 50}},
		{DirectoryLabel: "data", DataFile: api.DataverseDataFile{ID: 4, Filename: "raw.bin", FileSize: 4096}},
	}
	srv := dvFixtureServer(t, testPID, files, nil)
	f := newDvTestFs(t, srv.URL, testPID, "")

	got := remoteSet(mustList(t, f, ""))
	want := map[string]bool{"readme.txt": true, "papers": true, "data": true}
	if !equalSets(got, want) {
		t.Errorf(`List(""): got %v want %v`, got, want)
	}
}

func TestDataverseListSubdir(t *testing.T) {
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 1, Filename: "readme.txt", FileSize: 11}},
		{DirectoryLabel: "papers/2026", DataFile: api.DataverseDataFile{ID: 2, Filename: "report.pdf", FileSize: 100}},
		{DirectoryLabel: "papers/2026/figures", DataFile: api.DataverseDataFile{ID: 3, Filename: "fig1.png", FileSize: 50}},
		{DirectoryLabel: "papers/2026/figures", DataFile: api.DataverseDataFile{ID: 5, Filename: "fig2.png", FileSize: 70}},
	}
	srv := dvFixtureServer(t, testPID, files, nil)
	f := newDvTestFs(t, srv.URL, testPID, "")

	got := remoteSet(mustList(t, f, "papers/2026"))
	want := map[string]bool{"papers/2026/report.pdf": true, "papers/2026/figures": true}
	if !equalSets(got, want) {
		t.Errorf("List(papers/2026): got %v want %v", got, want)
	}

	got = remoteSet(mustList(t, f, "papers/2026/figures"))
	want = map[string]bool{"papers/2026/figures/fig1.png": true, "papers/2026/figures/fig2.png": true}
	if !equalSets(got, want) {
		t.Errorf("List figures: got %v want %v", got, want)
	}
}

// On the whole-version (non-/tree) path an unknown directory lists empty,
// matching the doi backend's existing behaviour for all providers (the
// lazy /tree path returns ErrorDirNotFound — see TestDataverseTreeUnknownDir).
func TestDataverseListUnknownDirEmpty(t *testing.T) {
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 1, Filename: "readme.txt", FileSize: 11}},
	}
	srv := dvFixtureServer(t, testPID, files, nil)
	f := newDvTestFs(t, srv.URL, testPID, "")
	entries, err := f.List(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("unknown dir should list empty, got %v", remoteSet(entries))
	}
}

func TestDataverseRootIsFile(t *testing.T) {
	files := []api.DataverseFile{
		{DirectoryLabel: "papers/2026", DataFile: api.DataverseDataFile{ID: 2, Filename: "report.pdf", FileSize: 100}},
	}
	srv := dvFixtureServer(t, testPID, files, nil)
	got, err := NewFs(context.Background(), "test", "papers/2026/report.pdf", configmap.Simple{
		"type": "doi", "host": srv.URL, "token": "secret", "dataset_pid": testPID,
	})
	if err != fs.ErrorIsFile {
		t.Fatalf("want ErrorIsFile, got %v", err)
	}
	if got.Root() != "papers/2026" {
		t.Errorf("want root=papers/2026, got %q", got.Root())
	}
}

func TestDataverseNewObject(t *testing.T) {
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 1, Filename: "readme.txt", FileSize: 11, MD5: "deadbeef"}},
	}
	srv := dvFixtureServer(t, testPID, files, nil)
	f := newDvTestFs(t, srv.URL, testPID, "")

	obj, err := f.NewObject(context.Background(), "readme.txt")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	if obj.Size() != 11 {
		t.Errorf("Size: got %d want 11", obj.Size())
	}
	if h, _ := obj.Hash(context.Background(), hash.MD5); h != "deadbeef" {
		t.Errorf("Hash: got %q want deadbeef", h)
	}
	if _, err := f.NewObject(context.Background(), "nope.txt"); err != fs.ErrorObjectNotFound {
		t.Errorf("want ErrorObjectNotFound, got %v", err)
	}
}

func TestDataverseOpenFullRead(t *testing.T) {
	payload := []byte("hello world from the dataverse direct mode")
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 1, Filename: "hello.txt", FileSize: int64(len(payload))}},
	}
	srv := dvFixtureServer(t, testPID, files, map[int64][]byte{1: payload})
	f := newDvTestFs(t, srv.URL, testPID, "")
	obj, err := f.NewObject(context.Background(), "hello.txt")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	rc, err := obj.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if got, _ := io.ReadAll(rc); string(got) != string(payload) {
		t.Errorf("body: got %q want %q", got, payload)
	}
}

func TestDataverseOpenRange(t *testing.T) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 9, Filename: "bin.dat", FileSize: int64(len(payload))}},
	}
	srv := dvFixtureServer(t, testPID, files, map[int64][]byte{9: payload})
	f := newDvTestFs(t, srv.URL, testPID, "")
	obj, err := f.NewObject(context.Background(), "bin.dat")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	rc, err := obj.Open(context.Background(), &fs.RangeOption{Start: 100, End: 199})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 100 {
		t.Fatalf("range length: got %d want 100", len(got))
	}
	if got[0] != payload[100] || got[99] != payload[199] {
		t.Errorf("range bytes mismatch: got[0]=%d got[99]=%d", got[0], got[99])
	}
}

func TestDataverseResumesOnMidStreamFailure(t *testing.T) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 11, Filename: "long.bin", FileSize: int64(len(payload))}},
	}

	presignHits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DataverseVersionResponse{
			Status: "OK",
			Data:   api.DataverseDatasetVersion{LastUpdateTime: "2026-05-01T00:00:00Z", Files: files},
		})
	})
	mux.HandleFunc("/api/access/datafile/11", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://"+r.Host+"/presigned/11")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/presigned/11", func(w http.ResponseWriter, r *http.Request) {
		presignHits++
		var start int64
		if rh := r.Header.Get("Range"); rh != "" {
			_, _ = fmt.Sscanf(rh, "bytes=%d-", &start)
		}
		if presignHits == 1 {
			// First fetch: promise the full length, write part, then
			// hijack-close to simulate a mid-stream cut.
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload[:200])
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("ResponseWriter doesn't support hijack")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		remaining := payload[start:]
		if start > 0 {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(payload)-1, len(payload)))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(remaining)))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(remaining)))
			w.WriteHeader(http.StatusOK)
		}
		_, _ = w.Write(remaining)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := newDvTestFs(t, srv.URL, testPID, "")
	obj, err := f.NewObject(context.Background(), "long.bin")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	rc, err := obj.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = rc.Close() }()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll after resume: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("resumed body mismatch: got %d bytes", len(got))
	}
	if presignHits < 2 {
		t.Errorf("expected at least 2 presigned fetches (initial + resume); got %d", presignHits)
	}
}

// A transient server status on a read (here 503 + Retry-After) must surface
// as a retriable error so rclone's transfer-level retry handles it — byte
// reads bypass the pacer, so the status must be translated, not swallowed.
func TestDataverseReadRetriesTransientStatus(t *testing.T) {
	files := []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 8, Filename: "f.bin", FileSize: 4}},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DataverseVersionResponse{
			Status: "OK",
			Data:   api.DataverseDatasetVersion{LastUpdateTime: "2026-05-01T00:00:00Z", Files: files},
		})
	})
	mux.HandleFunc("/api/access/datafile/8", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		http.Error(w, "slow down", http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := newDvTestFs(t, srv.URL, testPID, "")
	obj, err := f.NewObject(context.Background(), "f.bin")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	if _, err := obj.Open(context.Background()); err == nil {
		t.Fatal("Open: want error on 503")
	} else if d, ok := pacer.IsRetryAfter(err); !ok || d != 2*time.Second {
		t.Errorf("503 + Retry-After should yield a 2s retry-after error; got ok=%v d=%v err=%v", ok, d, err)
	}
}

func TestParseByteRange(t *testing.T) {
	tests := []struct {
		in   string
		s, e int64
		ok   bool
	}{
		{"bytes=0-99", 0, 99, true},
		{"bytes=100-", 100, -1, true},
		{"bytes=200-1023", 200, 1023, true},
		{"bytes=", 0, 0, false},
		{"items=0-9", 0, 0, false},
		{"bytes=abc-def", 0, 0, false},
	}
	for _, tc := range tests {
		s, e, ok := parseByteRange(tc.in)
		if s != tc.s || e != tc.e || ok != tc.ok {
			t.Errorf("parseByteRange(%q): got (%d,%d,%v) want (%d,%d,%v)", tc.in, s, e, ok, tc.s, tc.e, tc.ok)
		}
	}
}

func ingestFile() api.DataverseFile {
	return api.DataverseFile{
		DirectoryLabel: "data",
		DataFile: api.DataverseDataFile{
			ID: 1, Filename: "sample.tab", ContentType: "text/tab-separated-values",
			FileSize: 15, MD5: "original-md5",
			OriginalFileName: "sample.csv", OriginalFileSize: 24, OriginalFileFormat: "text/csv",
		},
	}
}

func TestDataverseIngestArchival(t *testing.T) {
	srv := dvFixtureServer(t, testPID, []api.DataverseFile{ingestFile()}, nil)
	got, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"type": "doi", "host": srv.URL, "token": "secret", "dataset_pid": testPID,
		"ingest_format": "archival",
	})
	if err != nil {
		t.Fatalf("NewFs: %v", err)
	}
	entries := mustList(t, got.(*Fs), "data")
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Remote() != "data/sample.tab" {
		t.Errorf("name: got %q want data/sample.tab", entries[0].Remote())
	}
	obj := entries[0].(*Object)
	// filesize is the served (.tab) size after ingest, so archival reports it.
	if obj.Size() != 15 {
		t.Errorf("archival size should be the served filesize (15); got %d", obj.Size())
	}
	// The stored MD5 is the original upload's, not the .tab's, so it's suppressed.
	if h, _ := obj.Hash(context.Background(), hash.MD5); h != "" {
		t.Errorf("archival MD5 should be suppressed; got %q", h)
	}
}

func TestDataverseIngestOriginal(t *testing.T) {
	srv := dvFixtureServer(t, testPID, []api.DataverseFile{ingestFile()}, nil)
	f := newDvTestFs(t, srv.URL, testPID, "") // default ingest_format=original
	obj, err := f.NewObject(context.Background(), "data/sample.csv")
	if err != nil {
		t.Fatalf("NewObject by original name: %v", err)
	}
	if obj.Size() != 24 {
		t.Errorf("size: got %d want 24", obj.Size())
	}
	if h, _ := obj.Hash(context.Background(), hash.MD5); h != "original-md5" {
		t.Errorf("original MD5 should be the stored md5; got %q", h)
	}
}

func TestDataverseInvalidIngestFormat(t *testing.T) {
	_, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"type": "doi", "host": "https://example.invalid", "dataset_pid": testPID,
		"ingest_format": "garbage",
	})
	if err == nil || !strings.Contains(err.Error(), "ingest_format") {
		t.Errorf("expected ingest_format error, got %v", err)
	}
}

// A 403 on the read (restricted file, missing/insufficient token) is
// reported as an attributed error and NOT retried.
func TestDataverseForbiddenAttributed(t *testing.T) {
	files := []api.DataverseFile{
		{Restricted: true, DataFile: api.DataverseDataFile{ID: 7, Filename: "secret.txt", FileSize: 10}},
	}
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DataverseVersionResponse{
			Status: "OK",
			Data:   api.DataverseDatasetVersion{LastUpdateTime: "2026-05-01T00:00:00Z", Files: files},
		})
	})
	mux.HandleFunc("/api/access/datafile/7", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := newDvTestFs(t, srv.URL, testPID, "")
	obj, err := f.NewObject(context.Background(), "secret.txt")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	if _, err := obj.Open(context.Background()); err == nil {
		t.Fatal("Open: want error on 403")
	} else if !strings.Contains(err.Error(), "restricted") {
		t.Errorf("want restricted attribution, got %v", err)
	}
	if hits != 1 {
		t.Errorf("a 403 must not be retried; got %d access hits", hits)
	}
}

func TestDataverseWritesRejected(t *testing.T) {
	srv := dvFixtureServer(t, testPID, []api.DataverseFile{
		{DataFile: api.DataverseDataFile{ID: 1, Filename: "x.txt", FileSize: 1}},
	}, nil)
	f := newDvTestFs(t, srv.URL, testPID, "")
	ctx := context.Background()
	if err := f.Mkdir(ctx, "newdir"); err == nil {
		t.Error("Mkdir: want error")
	}
	if err := f.Rmdir(ctx, "newdir"); err == nil {
		t.Error("Rmdir: want error")
	}
	if _, err := f.Put(ctx, nil, nil); err == nil {
		t.Error("Put: want error")
	}
}

func TestDataverseFsString(t *testing.T) {
	srv := dvFixtureServer(t, testPID, nil, nil)
	f := newDvTestFs(t, srv.URL, testPID, "")
	if !strings.Contains(f.String(), testPID) {
		t.Errorf("String should mention the dataset PID in direct mode; got %q", f.String())
	}
	if !f.Hashes().Contains(hash.MD5) {
		t.Error("Hashes should include MD5")
	}
}

func TestDataverseBadAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()
	if _, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"type": "doi", "host": srv.URL, "token": "bad", "dataset_pid": testPID,
	}); err == nil {
		t.Fatal("want error on auth failure")
	}
}

func TestDataverseRequiresHostAndPID(t *testing.T) {
	// Only one of host / dataset_pid set is an error.
	if _, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"type": "doi", "host": "https://example.invalid",
	}); err == nil {
		t.Error("host without dataset_pid should error")
	}
	// Neither doi nor host+dataset_pid is an error.
	if _, err := NewFs(context.Background(), "test", "", configmap.Simple{
		"type": "doi",
	}); err == nil {
		t.Error("neither doi nor host+dataset_pid should error")
	}
}

// ---- /tree fast-path --------------------------------------------------------

// treeFixtureServer mimics a Dataverse that exposes the lazy /tree
// endpoint. levels maps a directory path to its items; the handler pages
// them two-at-a-time (regardless of the client's limit) so cursor
// following is exercised. The access endpoint serves bytes from contents,
// keyed by the request-URI tail.
func treeFixtureServer(t *testing.T, dsPID string, levels map[string][]api.TreeItem, contents map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest/tree", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("persistentId") != dsPID {
			http.Error(w, "unknown dataset", http.StatusNotFound)
			return
		}
		items := levels[r.URL.Query().Get("path")]
		const pageSize = 2
		start := 0
		if c := r.URL.Query().Get("cursor"); c != "" {
			_, _ = fmt.Sscanf(c, "%d", &start)
		}
		end := start + pageSize
		var next *string
		if end < len(items) {
			s := fmt.Sprintf("%d", end)
			next = &s
		} else {
			end = len(items)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.TreeResponse{
			Status: "OK",
			Data:   api.TreePage{Path: r.URL.Query().Get("path"), Items: items[start:end], NextCursor: next},
		})
	})

	mux.HandleFunc("/api/access/datafile/", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.RequestURI(), "/api/access/datafile/")
		data, ok := contents[key]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDataverseTreeListsAndPages(t *testing.T) {
	dsPID := "doi:10.5072/FK2/TREE"
	levels := map[string][]api.TreeItem{
		"": {
			{Type: "folder", Name: "data", Path: "data", Counts: &api.TreeCounts{Files: 3}},
			{Type: "file", Name: "readme.txt", Path: "readme.txt", ID: 1, Size: 11,
				ContentType: "text/plain", Access: "public",
				Checksum:    &api.TreeChecksum{Type: "MD5", Value: "md5-readme"},
				DownloadURL: "/api/access/datafile/1"},
		},
		"data": {
			{Type: "file", Name: "a.bin", Path: "data/a.bin", ID: 2, Size: 5, Access: "public", DownloadURL: "/api/access/datafile/2"},
			{Type: "file", Name: "b.bin", Path: "data/b.bin", ID: 3, Size: 6, Access: "public", DownloadURL: "/api/access/datafile/3"},
			{Type: "file", Name: "c.bin", Path: "data/c.bin", ID: 4, Size: 7, Access: "public", DownloadURL: "/api/access/datafile/4"},
		},
	}
	srv := treeFixtureServer(t, dsPID, levels, nil)
	f := newDvTestFs(t, srv.URL, dsPID, "")
	if !f.useTree {
		t.Fatal("expected useTree=true against a /tree-capable server")
	}

	got := remoteSet(mustList(t, f, ""))
	if want := map[string]bool{"data": true, "readme.txt": true}; !equalSets(got, want) {
		t.Errorf(`List(""): got %v want %v`, got, want)
	}

	// 3 files paged 2-at-a-time: the cursor must be followed to see all.
	got = remoteSet(mustList(t, f, "data"))
	if want := map[string]bool{"data/a.bin": true, "data/b.bin": true, "data/c.bin": true}; !equalSets(got, want) {
		t.Errorf("List(data): got %v want %v (cursor not followed?)", got, want)
	}

	obj, err := f.NewObject(context.Background(), "readme.txt")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	if obj.Size() != 11 {
		t.Errorf("Size: got %d want 11", obj.Size())
	}
	if h, _ := obj.Hash(context.Background(), hash.MD5); h != "md5-readme" {
		t.Errorf("Hash: got %q want md5-readme", h)
	}
}

func TestDataverseTreeUnknownDir(t *testing.T) {
	dsPID := "doi:10.5072/FK2/TREE"
	levels := map[string][]api.TreeItem{
		"": {{Type: "file", Name: "x.txt", Path: "x.txt", ID: 1, Size: 1, Access: "public", DownloadURL: "/api/access/datafile/1"}},
	}
	srv := treeFixtureServer(t, dsPID, levels, nil)
	f := newDvTestFs(t, srv.URL, dsPID, "")
	if _, err := f.List(context.Background(), "nope"); err != fs.ErrorDirNotFound {
		t.Errorf("want ErrorDirNotFound for unknown tree dir, got %v", err)
	}
}

func TestDataverseTreeOpenViaDownloadURL(t *testing.T) {
	dsPID := "doi:10.5072/FK2/TREE"
	payload := []byte("tree bytes!")
	levels := map[string][]api.TreeItem{
		"": {{Type: "file", Name: "hello.txt", Path: "hello.txt", ID: 9, Size: int64(len(payload)),
			Access: "public", DownloadURL: "/api/access/datafile/9"}},
	}
	srv := treeFixtureServer(t, dsPID, levels, map[string][]byte{"9": payload})
	f := newDvTestFs(t, srv.URL, dsPID, "")
	obj, err := f.NewObject(context.Background(), "hello.txt")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	rc, err := obj.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if got, _ := io.ReadAll(rc); string(got) != string(payload) {
		t.Errorf("body: got %q want %q", got, payload)
	}
}

// ingest_format=original (default) => originals=true on /tree. The access
// handler 400s unless format=original, so a green test proves the
// originals flag flowed end-to-end.
func TestDataverseTreeIngestOriginalsForwarded(t *testing.T) {
	dsPID := "doi:10.5072/FK2/TREE"
	original := []byte("ORIGINAL,csv,bytes\n1,2,3\n")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest/tree", func(w http.ResponseWriter, r *http.Request) {
		item := api.TreeItem{Type: "file", Name: "sample.tab", Path: "sample.tab", ID: 5,
			ContentType: "text/tab-separated-values", Access: "public"}
		if r.URL.Query().Get("originals") == "true" {
			item.Size = int64(len(original))
			item.Checksum = &api.TreeChecksum{Type: "MD5", Value: "orig-md5"}
			item.DownloadURL = "/api/access/datafile/5?format=original"
		} else {
			item.Size = 7
			item.DownloadURL = "/api/access/datafile/5"
		}
		var items []api.TreeItem
		if r.URL.Query().Get("path") == "" {
			items = []api.TreeItem{item}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.TreeResponse{Status: "OK", Data: api.TreePage{Items: items}})
	})
	mux.HandleFunc("/api/access/datafile/5", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "original" {
			http.Error(w, "want original", http.StatusBadRequest)
			return
		}
		_, _ = w.Write(original)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := newDvTestFs(t, srv.URL, dsPID, "")
	obj, err := f.NewObject(context.Background(), "sample.tab")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	if obj.Size() != int64(len(original)) {
		t.Errorf("Size: got %d want %d (original)", obj.Size(), len(original))
	}
	if h, _ := obj.Hash(context.Background(), hash.MD5); h != "orig-md5" {
		t.Errorf("Hash: got %q want orig-md5", h)
	}
	rc, err := obj.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if got, _ := io.ReadAll(rc); string(got) != string(original) {
		t.Errorf("body: got %q want original bytes", got)
	}
}

// In S3-direct mode the access endpoint 302s to a presigned URL on
// another host. The token must ride the Dataverse hop but NOT be
// forwarded to the storage host.
func TestDataverseReadStripsTokenOnCrossHostRedirect(t *testing.T) {
	payload := []byte("s3 object bytes")
	var tokenSeenByStorage string
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenSeenByStorage = r.Header.Get(api.AuthHeader)
		_, _ = w.Write(payload)
	}))
	defer storage.Close()

	files := []api.DataverseFile{{DataFile: api.DataverseDataFile{ID: 3, Filename: "f.bin", FileSize: int64(len(payload))}}}
	var tokenSeenByDataverse string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/datasets/:persistentId/versions/:latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.DataverseVersionResponse{
			Status: "OK",
			Data:   api.DataverseDatasetVersion{LastUpdateTime: "2026-05-01T00:00:00Z", Files: files},
		})
	})
	mux.HandleFunc("/api/access/datafile/3", func(w http.ResponseWriter, r *http.Request) {
		tokenSeenByDataverse = r.Header.Get(api.AuthHeader)
		http.Redirect(w, r, storage.URL+"/object", http.StatusFound)
	})
	dataverse := httptest.NewServer(mux)
	defer dataverse.Close()

	f := newDvTestFs(t, dataverse.URL, testPID, "")
	obj, err := f.NewObject(context.Background(), "f.bin")
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	rc, err := obj.Open(context.Background())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = rc.Close() }()
	if got, _ := io.ReadAll(rc); string(got) != string(payload) {
		t.Errorf("body: got %q want %q", got, payload)
	}
	if tokenSeenByDataverse != "secret" {
		t.Errorf("Dataverse hop should carry the token; got %q", tokenSeenByDataverse)
	}
	if tokenSeenByStorage != "" {
		t.Errorf("storage hop must NOT see the token; got %q", tokenSeenByStorage)
	}
}
