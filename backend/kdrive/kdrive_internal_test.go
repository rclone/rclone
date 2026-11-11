// Unit tests for the kdrive backend
package kdrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/kdrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckChunkSize checks chunk_size validation against the API limits
func TestCheckChunkSize(t *testing.T) {
	for _, test := range []struct {
		in      fs.SizeSuffix
		wantErr string
	}{
		{in: 20 * fs.Mebi, wantErr: ""},                    // default
		{in: 1000 * 1000, wantErr: ""},                     // API minimum
		{in: 1000 * 1000 * 1000, wantErr: ""},              // API maximum
		{in: 0, wantErr: "too small"},                      // panics without validation
		{in: 999 * 1000, wantErr: "too small"},             // below API minimum
		{in: 1000*1000*1000 + 1, wantErr: "too large"},     // above API maximum
		{in: 2 * 1000 * 1000 * 1000, wantErr: "too large"}, // way above API maximum
	} {
		t.Run(test.in.String(), func(t *testing.T) {
			err := checkChunkSize(test.in)
			if test.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.wantErr)
			}
		})
	}
}

// TestOpenChunkWriterClampsCallerChunkSize checks that a caller
// supplied chunk size hint below the API minimum is raised to it
func TestOpenChunkWriterClampsCallerChunkSize(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = fmt.Fprintln(w, `{"result":"success","data":{"token":"tok","upload_url":"http://unused"}}`)
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	f.dirCache = dircache.New("", "1", f)
	src := &mockObjectInfo{remote: "file.txt", size: 4, modTime: time.Unix(1700000000, 0)}

	info, _, err := f.OpenChunkWriter(context.Background(), "file.txt", src, &fs.ChunkOption{ChunkSize: 512 * 1024})
	require.NoError(t, err)
	assert.Equal(t, int64(minChunkSize), info.ChunkSize, "a chunk size hint below the API minimum must be raised to it")
}

// TestObjectSetMetaDataResetsStaleHash checks that setMetaData always
// replaces the hash (a chunked upload finish response carries none)
func TestObjectSetMetaDataResetsStaleHash(t *testing.T) {
	for _, test := range []struct {
		name          string
		staleHash     string
		staleNested   bool
		newHash       string
		wantHash      string
		wantIsNested  bool
		wantHashKnown bool
	}{
		{
			name:          "empty hash clears stale plain hash",
			staleHash:     "deadbeef",
			staleNested:   false,
			newHash:       "",
			wantHash:      "",
			wantIsNested:  false,
			wantHashKnown: false,
		},
		{
			name:          "empty hash clears stale nested hash",
			staleHash:     "deadbeef",
			staleNested:   true,
			newHash:       "",
			wantHash:      "",
			wantIsNested:  false,
			wantHashKnown: false,
		},
		{
			name:          "plain hash is set",
			staleHash:     "deadbeef",
			staleNested:   false,
			newHash:       "xxh3:12345678",
			wantHash:      "12345678",
			wantIsNested:  false,
			wantHashKnown: true,
		},
		{
			name:          "nested hash is set",
			staleHash:     "deadbeef",
			staleNested:   false,
			newHash:       "N:xxh3:87654321",
			wantHash:      "87654321",
			wantIsNested:  true,
			wantHashKnown: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			o := &Object{
				remote:      "file.txt",
				hasMetaData: true,
				xxh3:        test.staleHash,
				nestedHash:  test.staleNested,
			}
			info := &api.Item{
				ID:   42,
				Name: "file.txt",
				Type: "file",
				Hash: test.newHash,
			}

			err := o.setMetaData(info)
			require.NoError(t, err)
			assert.Equal(t, test.wantHash, o.xxh3, "hash after setMetaData")
			assert.Equal(t, test.wantIsNested, o.nestedHash, "nested flag after setMetaData")
			assert.True(t, o.hasMetaData)

			// Hash() must reflect the new state. Only checked when a
			// hash is known: with no hash Hash() would query the API.
			// Nested hashes are not comparable with plain XXH3, so
			// Hash() returns "" for them.
			if test.wantHashKnown {
				got, gotErr := o.Hash(context.Background(), hash.XXH3)
				require.NoError(t, gotErr)
				if test.wantIsNested {
					assert.Equal(t, "", got, "nested hash must not be returned as XXH3")
				} else {
					assert.Equal(t, test.wantHash, got)
				}
			}
		})
	}
}

// newTestFs creates an Fs wired to a test server with a plain client
// (no OAuth) and a fast pacer
func newTestFs(t *testing.T, endpoint string) *Fs {
	t.Helper()
	return &Fs{
		name: "TestKdrive",
		root: "",
		opt:  Options{DriveID: "1234", Endpoint: endpoint},
		srv:  rest.NewClient(&http.Client{}).SetRoot(endpoint),
		pacer: fs.NewPacer(context.Background(),
			pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(2*time.Millisecond), pacer.DecayConstant(2))),
	}
}

// TestWriteChunkDetectsResultError checks that a 200 with result:error
// is reported as an error
func TestWriteChunkDetectsResultError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"chunk_error","description":"chunk upload failed"}}`)
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	u := &uploadSession{
		f:           f,
		token:       "tok",
		uploadURL:   ts.URL,
		fileSize:    4,
		chunkSize:   4,
		chunkHashes: make(map[int64]string),
	}

	_, err := u.WriteChunk(context.Background(), 0, bytes.NewReader([]byte("test")))
	require.Error(t, err, "result:error with HTTP 200 must not be treated as success")
	assert.Contains(t, err.Error(), "chunk upload failed")
}

// TestWriteChunkRetriesAndRewinds checks that transient failures are
// retried with the chunk re-sent from the start of the reader
func TestWriteChunkRetriesAndRewinds(t *testing.T) {
	var mu sync.Mutex
	var attempts int
	var bodies []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		attempts++
		bodies = append(bodies, string(body))
		current := attempts
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if current == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"internal_error","description":"transient"}}`)
			return
		}
		_, _ = fmt.Fprintln(w, `{"result":"success","data":{"chunk_number":1,"status":"OK"}}`)
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	u := &uploadSession{
		f:           f,
		token:       "tok",
		uploadURL:   ts.URL,
		fileSize:    4,
		chunkSize:   4,
		chunkHashes: make(map[int64]string),
	}

	written, err := u.WriteChunk(context.Background(), 0, bytes.NewReader([]byte("test")))
	require.NoError(t, err)
	assert.Equal(t, int64(4), written)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, bodies, 2, "chunk upload must be retried once after a 5xx")
	assert.Equal(t, "test", bodies[0], "first attempt sends the chunk")
	assert.Equal(t, "test", bodies[1], "retry must re-send the chunk from the start of the reader")
}

// kdriveMockItem returns a JSON body for a single item result
func kdriveMockItem(id int, name string) string {
	return fmt.Sprintf(`{"result":"success","data":{"id":%d,"name":%q,"type":"file","path":%q,"last_modified_at":1700000000}}`, id, name, "/"+name)
}

// mockObjectInfo is a minimal fs.ObjectInfo for testing
type mockObjectInfo struct {
	remote  string
	size    int64
	modTime time.Time
	hash    string
}

func (m *mockObjectInfo) Fs() fs.Info                         { return nil }
func (m *mockObjectInfo) String() string                      { return m.remote }
func (m *mockObjectInfo) Remote() string                      { return m.remote }
func (m *mockObjectInfo) Size() int64                         { return m.size }
func (m *mockObjectInfo) ModTime(_ context.Context) time.Time { return m.modTime }
func (m *mockObjectInfo) Storable() bool                      { return true }
func (m *mockObjectInfo) MimeType(_ context.Context) string   { return "application/octet-stream" }

func (m *mockObjectInfo) Hash(_ context.Context, t hash.Type) (string, error) {
	if m.hash == "" || t != hash.XXH3 {
		return "", hash.ErrUnsupported
	}
	return m.hash, nil
}

// TestUpdateDirectNoRetryOnServerError checks that a failed single
// part upload is not retried here: the input must not be buffered, the
// higher layers retry the whole transfer
func TestUpdateDirectNoRetryOnServerError(t *testing.T) {
	var attempts int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"internal_error","description":"transient"}}`)
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	o := &Object{fs: f, remote: "file.txt"}
	src := &mockObjectInfo{remote: "file.txt", size: 4, modTime: time.Unix(1700000000, 0), hash: "12345678"}

	err := o.updateDirect(context.Background(), bytes.NewReader([]byte("test")), "1", "file.txt", src)
	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts), "single part uploads must not be retried here")
}

// TestUpdateDirect checks the upload parameters and metadata of a
// successful single part upload, and that total_chunk_hash is only
// sent when the source hash is known
func TestUpdateDirect(t *testing.T) {
	for _, test := range []struct {
		name          string
		srcHash       string
		wantTotalHash string
	}{
		{name: "hash known", srcHash: "12345678", wantTotalHash: "xxh3:12345678"},
		{name: "hash unknown", srcHash: "", wantTotalHash: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var gotTotalHash, gotBody string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				gotTotalHash = r.URL.Query().Get("total_chunk_hash")
				body, _ := io.ReadAll(r.Body)
				gotBody = string(body)
				_, _ = fmt.Fprintln(w, `{"result":"success","data":{"id":55,"name":"file.txt","type":"file","path":"/file.txt","hash":"xxh3:99998888","last_modified_at":1700000000}}`)
			}))
			defer ts.Close()

			f := newTestFs(t, ts.URL)
			o := &Object{fs: f, remote: "file.txt"}
			src := &mockObjectInfo{remote: "file.txt", size: 4, modTime: time.Unix(1700000000, 0), hash: test.srcHash}

			err := o.updateDirect(context.Background(), bytes.NewReader([]byte("test")), "1", "file.txt", src)
			require.NoError(t, err)
			assert.Equal(t, "test", gotBody, "the content must be streamed as-is")
			assert.Equal(t, test.wantTotalHash, gotTotalHash, "total_chunk_hash")
			assert.Equal(t, "55", o.id)
			gotHash, hashErr := o.Hash(context.Background(), hash.XXH3)
			require.NoError(t, hashErr)
			assert.Equal(t, "99998888", gotHash)
		})
	}
}

// TestNewFsRequiresDriveID checks that NewFs fails fast when drive_id
// is missing
func TestNewFsRequiresDriveID(t *testing.T) {
	m := configmap.Simple{"chunk_size": "20Mi"}
	_, err := NewFs(context.Background(), "kdrive", "", m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "drive_id")
}

// TestUploadSessionStringMasksToken checks that the upload session
// String does not leak the session token in debug logs
func TestUploadSessionStringMasksToken(t *testing.T) {
	u := &uploadSession{token: "very-secret-uuid"}
	assert.NotContains(t, u.String(), "very-secret-uuid")
}

// TestListNonRecursiveSingleRoundTrip checks that a non recursive
// listing needs a single API request: without depth=unlimited the API
// returns direct children only, so neither the parent item lookup nor
// a child filter is needed
func TestListNonRecursiveSingleRoundTrip(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/files/1/files"):
			// Direct children only, as returned without depth=unlimited
			_, _ = fmt.Fprintln(w, `{"result":"success","data":[`+
				`{"id":10,"name":"sub","type":"dir","path":"/sub"},`+
				`{"id":11,"name":"file.txt","type":"file","path":"/file.txt","hash":"xxh3:12345678","last_modified_at":1700000000}],"has_more":false}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"unexpected","description":"unexpected request"}}`)
		}
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	f.dirCache = dircache.New("", "1", f)

	entries, err := f.List(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&requests), "non recursive listing must make a single API request")

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Remote())
	}
	assert.ElementsMatch(t, []string{"sub", "file.txt"}, names)
}

// TestMoveConflictReturnsError checks that moving onto an occupied
// destination reports the API conflict instead of resolving it: rclone
// removes the destination itself before calling Move
func TestMoveConflictReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/files/111/move/1"):
			_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"conflict_error","description":"destination already exists"}}`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"unexpected","description":"unexpected request"}}`)
		}
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	f.dirCache = dircache.New("", "1", f)
	srcObj := &Object{fs: f, remote: "src.txt", id: "111", hasMetaData: true}

	_, err := f.Move(context.Background(), srcObj, "file2")
	require.Error(t, err, "a move onto an occupied destination must fail")
	assert.Contains(t, err.Error(), "conflict", "the API conflict must be reported")
}

// TestMoveDetectsForeignObjectAtDestination checks that Move does not
// return a file re-created by another writer at the destination
func TestMoveDetectsForeignObjectAtDestination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/files/111/move/1"):
			// The move API reports success...
			_, _ = fmt.Fprintln(w, `{"result":"success","data":{"cancel_id":"abc","valid_until":1700000000}}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/files/1/name"):
			// ...but the file at the destination is not the moved
			// file: another writer re-created it
			_, _ = fmt.Fprintln(w, kdriveMockItem(999, "file2"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"unexpected","description":"unexpected request"}}`)
		}
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	f.dirCache = dircache.New("", "1", f)
	srcObj := &Object{fs: f, remote: "src.txt", id: "111", hasMetaData: true}

	_, err := f.Move(context.Background(), srcObj, "file2")
	require.Error(t, err, "Move must not report success when the destination is not the moved file")
}

// TestMoveSuccess checks the nominal move: the object returned is the
// moved file (same ID as the source).
func TestMoveSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(io.Discard, r.Body)
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/files/111/move/1"):
			_, _ = fmt.Fprintln(w, `{"result":"success","data":{"cancel_id":"abc","valid_until":1700000000}}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/files/1/name"):
			// After the move, the moved file is at the destination
			_, _ = fmt.Fprintln(w, kdriveMockItem(111, "file2"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			_, _ = fmt.Fprintln(w, `{"result":"error","error":{"code":"unexpected","description":"unexpected request"}}`)
		}
	}))
	defer ts.Close()

	f := newTestFs(t, ts.URL)
	f.dirCache = dircache.New("", "1", f)
	srcObj := &Object{fs: f, remote: "src.txt", id: "111", hasMetaData: true}

	dstObj, err := f.Move(context.Background(), srcObj, "file2")
	require.NoError(t, err)
	moved, ok := dstObj.(*Object)
	require.True(t, ok)
	assert.Equal(t, "111", moved.id, "the returned object must be the moved file")
}

// TestFormatModTime checks that pre-1970 times format as negative
// unix timestamps instead of wrapping through a uint64 cast
func TestFormatModTime(t *testing.T) {
	for _, test := range []struct {
		in   time.Time
		want string
	}{
		{in: time.Unix(1700000000, 0), want: "1700000000"},
		{in: time.Unix(0, 0), want: "0"},
		{in: time.Unix(-1577923200, 0), want: "-1577923200"}, // Sat Jan 2 1965
	} {
		t.Run(test.want, func(t *testing.T) {
			assert.Equal(t, test.want, formatModTime(test.in))
		})
	}
}

// TestObjectSetMetaDataIsDir checks that directories are rejected
func TestObjectSetMetaDataIsDir(t *testing.T) {
	o := &Object{remote: "dir"}
	info := &api.Item{ID: 42, Name: "dir", Type: "dir"}
	err := o.setMetaData(info)
	assert.Equal(t, fs.ErrorIsDir, err)
}

// TestShouldRetryLockError checks that transient locks held by server
// side copies are retried while locks held by applications and other
// errors are not
func TestShouldRetryLockError(t *testing.T) {
	for _, test := range []struct {
		name      string
		body      string
		status    int
		wantRetry bool
	}{
		{
			name:      "lock held by internal copy",
			body:      `{"result":"error","error":{"code":"lock_error","description":"There is a lock on this file, locked by internal_copy","context":{"values":["internal_copy"]}}}`,
			status:    http.StatusForbidden,
			wantRetry: true,
		},
		{
			name:      "lock held by an application",
			body:      `{"result":"error","error":{"code":"lock_error","description":"There is a lock on this file, locked by onlyoffice","context":{"values":["onlyoffice"]}}}`,
			status:    http.StatusForbidden,
			wantRetry: false,
		},
		{
			name:      "lock without context",
			body:      `{"result":"error","error":{"code":"lock_error","description":"There is a lock on this file, locked by internal_copy"}}`,
			status:    http.StatusForbidden,
			wantRetry: false,
		},
		{
			name:      "other api error",
			body:      `{"result":"error","error":{"code":"object_not_found","description":"File not found"}}`,
			status:    http.StatusNotFound,
			wantRetry: false,
		},
		{
			name:      "server error",
			status:    http.StatusServiceUnavailable,
			wantRetry: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var apiErr error
			if test.body != "" {
				e := new(api.ResultStatus)
				require.NoError(t, json.Unmarshal([]byte(test.body), e))
				apiErr = e
			}
			resp := &http.Response{StatusCode: test.status, Header: http.Header{}}
			got, gotErr := shouldRetry(context.Background(), resp, apiErr)
			assert.Equal(t, test.wantRetry, got)
			assert.Equal(t, apiErr, gotErr)
		})
	}
}
