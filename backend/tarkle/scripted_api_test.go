package tarkle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/tarkle/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiCall is one request seen by a scriptServer
type apiCall struct {
	method, path, query, key string
	body                     map[string]any
	size                     int // bytes of the request body
}

func (c apiCall) str(k string) string { s, _ := c.body[k].(string); return s }

// scriptServer answers requests with answer and records them. Paths are
// given without /api/v1; storage URLs are under /store/.
type scriptServer struct {
	mu     sync.Mutex
	calls  []apiCall
	url    string
	header http.Header // extra headers sent with every answer
}

func newScriptServer(t *testing.T, extra map[string]string, answer func(s *scriptServer, c apiCall) (int, string)) (*Fs, *scriptServer) {
	s := &scriptServer{header: http.Header{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		c := apiCall{
			method: r.Method,
			path:   strings.TrimPrefix(r.URL.Path, "/api/v1"),
			query:  r.URL.RawQuery,
			key:    r.Header.Get("Idempotency-Key"),
			size:   len(raw),
		}
		_ = json.Unmarshal(raw, &c.body)
		s.mu.Lock()
		s.calls = append(s.calls, c)
		s.mu.Unlock()
		status, out := answer(s, c)
		for k, v := range s.header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(out))
	}))
	t.Cleanup(srv.Close)
	s.url = srv.URL
	cfg := testConfig(srv.URL, testWS)
	for k, v := range extra {
		cfg[k] = v
	}
	f, err := NewFs(context.Background(), "t", "", cfg)
	require.NoError(t, err)
	return f.(*Fs), s
}

// writes returns the method and path of every API request that is not a GET
func (s *scriptServer) writes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, c := range s.calls {
		if c.method != "GET" && !strings.HasPrefix(c.path, "/store/") {
			out = append(out, c.method+" "+c.path)
		}
	}
	return out
}

// find returns the calls with this method and path
func (s *scriptServer) find(method, path string) []apiCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []apiCall
	for _, c := range s.calls {
		if c.method == method && c.path == path {
			out = append(out, c)
		}
	}
	return out
}

func fileJSON(id, name string) string {
	return `{"id":"` + id + `","type":"file","name":"` + name + `","size":3,"parent_id":null,"modified_at":"2020-01-02T03:04:05.000Z"}`
}

func problem(status int, code string) (int, string) {
	return status, `{"error":{"code":"` + code + `","message":"m"}}`
}

// failReader fails the test if anything reads it
type failReader struct{ t *testing.T }

func (r failReader) Read(p []byte) (int, error) {
	r.t.Error("the source was read")
	return 0, io.EOF
}

// noHashInfo is a source that fails the test if asked for its hash
type noHashInfo struct {
	fs.ObjectInfo
	t *testing.T
}

func (i noHashInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	i.t.Error("the source was asked for its hash before the upload")
	return "", hash.ErrUnsupported
}

// countingReader counts the bytes read through it
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// singleUpload answers a single-part upload of name: start, the PUT and complete
func singleUpload(s *scriptServer, c apiCall, size int) (int, string, bool) {
	switch {
	case c.method == "POST" && c.path == "/uploads":
		return 201, `{"action":"upload","mode":"single","id":"u1","part_size":` + itoa(size) + `,"part_count":1,"uploads":[{"partNumber":1,"url":"` + s.url + `/store/put"}]}`, true
	case c.method == "PUT" && c.path == "/store/put":
		return 200, ``, true
	case c.method == "POST" && c.path == "/uploads/u1/complete":
		return 200, fileJSON("7", "f.txt"), true
	}
	return 0, "", false
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestCopyOverExistingKeepsItWhenCopyFails(t *testing.T) {
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		if c.method == "POST" && c.path == "/files/1/copy" {
			if c.str("name") == "f.txt" {
				return problem(409, "name_taken")
			}
			return problem(413, "storage_full")
		}
		return problem(404, "not_found")
	})
	src := &Object{fs: f, remote: "a.txt", id: "1", size: 3}
	_, err := f.Copy(context.Background(), src, "f.txt")
	require.Error(t, err)
	assert.Equal(t, []string{"POST /files/1/copy", "POST /files/1/copy"}, s.writes(), "nothing renamed or deleted")
}

func TestCopyOverExistingSwapsIn(t *testing.T) {
	var temp string
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "POST" && c.path == "/files/1/copy":
			if c.str("name") == "f.txt" {
				return problem(409, "name_taken")
			}
			temp = c.str("name")
			return 201, fileJSON("9", temp)
		case c.method == "GET" && c.path == "/files":
			return 200, `{"entries":[` + fileJSON("5", "f.txt") + `,` + fileJSON("9", temp) + `],"next_cursor":null}`
		case c.method == "PATCH":
			return 200, fileJSON(strings.TrimPrefix(c.path, "/files/"), c.str("name"))
		case c.method == "DELETE":
			return 200, `{"deleted":true}`
		}
		return problem(404, "not_found")
	})
	src := &Object{fs: f, remote: "a.txt", id: "1", size: 3}
	dst, err := f.Copy(context.Background(), src, "f.txt")
	require.NoError(t, err)
	assert.Equal(t, "9", dst.(*Object).id)
	assert.Equal(t, []string{"POST /files/1/copy", "POST /files/1/copy", "PATCH /files/5", "PATCH /files/9", "DELETE /files/5"}, s.writes())
}

// replaceAnswers answers a folder listing with files and every PATCH with
// the renamed item, refusing renames of the ids in refuse
func replaceAnswers(files string, refuse map[string]bool) func(s *scriptServer, c apiCall) (int, string) {
	return func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "GET" && c.path == "/files":
			return 200, `{"entries":[` + files + `],"next_cursor":null}`
		case c.method == "PATCH":
			id := strings.TrimPrefix(c.path, "/files/")
			if refuse[id] {
				return problem(409, "name_taken")
			}
			return 200, fileJSON(id, c.str("name"))
		case c.method == "DELETE":
			return 200, `{"deleted":true}`
		}
		return problem(404, "not_found")
	}
}

func TestReplaceExistingRenamesAsideIgnoringCase(t *testing.T) {
	f, s := newScriptServer(t, nil, replaceAnswers(fileJSON("old", "medium.bin")+","+fileJSON("new", "MEDIUM (2).bin"), nil))
	item := &api.Item{ID: "new", Type: api.ItemTypeFile, Name: "MEDIUM (2).bin"}
	require.NoError(t, f.replaceExisting(context.Background(), "", "MEDIUM.bin", item))
	assert.Equal(t, []string{"PATCH /files/old", "PATCH /files/new", "DELETE /files/old"}, s.writes())
	assert.True(t, strings.HasPrefix(s.find("PATCH", "/files/old")[0].str("name"), "rclone-tmp-"), "the old file is renamed aside first")
	assert.Equal(t, "MEDIUM.bin", item.Name)
}

func TestReplaceExistingPutsOldFileBack(t *testing.T) {
	f, s := newScriptServer(t, nil, replaceAnswers(fileJSON("old", "f.txt")+","+fileJSON("new", "f (2).txt"), map[string]bool{"new": true}))
	item := &api.Item{ID: "new", Type: api.ItemTypeFile, Name: "f (2).txt"}
	require.Error(t, f.replaceExisting(context.Background(), "", "f.txt", item))
	assert.Equal(t, []string{"PATCH /files/old", "PATCH /files/new", "PATCH /files/old", "DELETE /files/new"}, s.writes())
	patches := s.find("PATCH", "/files/old")
	assert.Equal(t, "f.txt", patches[len(patches)-1].str("name"), "the old file gets its name back")
}

func TestReplaceExistingLeavesNoStrayCopy(t *testing.T) {
	f, s := newScriptServer(t, nil, replaceAnswers(fileJSON("new", "f (2).txt"), map[string]bool{"new": true}))
	item := &api.Item{ID: "new", Type: api.ItemTypeFile, Name: "f (2).txt"}
	require.Error(t, f.replaceExisting(context.Background(), "", "f.txt", item))
	assert.Equal(t, []string{"PATCH /files/new", "DELETE /files/new"}, s.writes(), "the new copy that couldn't take the name goes to the bin")
}

func TestWritesCarryOneIdempotencyKeyAcrossRetries(t *testing.T) {
	tries := 0
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "GET" && c.path == "/files":
			return 200, `{"entries":[],"next_cursor":null}`
		case c.method == "POST" && c.path == "/folders":
			tries++
			switch tries {
			case 1:
				return problem(503, "unavailable")
			case 2:
				return problem(409, "idempotency_in_progress")
			}
			return 201, `{"id":"d1","type":"folder","name":"sub"}`
		}
		return problem(404, "not_found")
	})
	require.NoError(t, f.Mkdir(context.Background(), "sub"))
	posts := s.find("POST", "/folders")
	require.Len(t, posts, 3, "a 503 and a 409 idempotency_in_progress are both retried")
	assert.NotEmpty(t, posts[0].key)
	assert.Equal(t, posts[0].key, posts[1].key, "a retry repeats the key")
	assert.Equal(t, posts[0].key, posts[2].key, "a retry repeats the key")
	for _, c := range s.find("GET", "/files") {
		assert.Empty(t, c.key, "a GET has no Idempotency-Key")
	}
}

func TestUpdateIdenticalContentDoesNotReadSource(t *testing.T) {
	f, s := newScriptServer(t, map[string]string{"hash_before_upload": "true"}, func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "POST" && c.path == "/uploads":
			return 200, `{"action":"skip","existing":{"id":"1","name":"f.txt"}}`
		case c.method == "GET" && c.path == "/files/1":
			return 200, fileJSON("1", "f.txt")
		case c.method == "PATCH" && c.path == "/files/1": // the source's modification time is set on the identical file
			return 200, fileJSON("1", "f.txt")
		}
		return problem(404, "not_found")
	})
	sum := strings.Repeat("ab", 32)
	o := &Object{fs: f, remote: "f.txt"}
	src := object.NewStaticObjectInfo("f.txt", time.Now(), 3, true, map[hash.Type]string{hash.SHA256: sum}, nil)
	require.NoError(t, o.Update(context.Background(), failReader{t}, src))
	assert.Equal(t, "1", o.id)
	starts := s.find("POST", "/uploads")
	require.Len(t, starts, 1)
	assert.Equal(t, sum, starts[0].str("sha256"), "the SHA-256 is sent when the upload starts, so the server can skip")
}

func TestSkipChunkSizeIsBounded(t *testing.T) {
	f, _ := newScriptServer(t, map[string]string{"hash_before_upload": "true"}, func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "POST" && c.path == "/uploads":
			return 200, `{"action":"skip","existing":{"id":"1","name":"f.txt"}}`
		case c.method == "GET" && c.path == "/files/1":
			return 200, fileJSON("1", "f.txt")
		}
		return problem(404, "not_found")
	})
	src := object.NewStaticObjectInfo("f.txt", time.Now(), 10<<30, true, map[hash.Type]string{hash.SHA256: strings.Repeat("ab", 32)}, nil)
	info, _, err := f.OpenChunkWriter(context.Background(), "f.txt", src)
	require.NoError(t, err)
	assert.Equal(t, int64(skipChunkSize), info.ChunkSize, "a skipped 10 GiB file is not buffered whole")
}

func TestUpdateReadsSourceOnceAndSendsItsHash(t *testing.T) {
	data := []byte("hello tarkle")
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		if st, out, ok := singleUpload(s, c, len(data)); ok {
			return st, out
		}
		return problem(404, "not_found")
	})
	o := &Object{fs: f, remote: "f.txt"}
	in := &countingReader{r: bytes.NewReader(data)}
	src := noHashInfo{ObjectInfo: object.NewStaticObjectInfo("f.txt", time.Now(), int64(len(data)), true, nil, nil), t: t}
	require.NoError(t, o.Update(context.Background(), in, src))
	assert.Equal(t, len(data), in.n, "the source is read exactly once")
	want := sha256.Sum256(data)
	completes := s.find("POST", "/uploads/u1/complete")
	require.Len(t, completes, 1)
	assert.Equal(t, hex.EncodeToString(want[:]), completes[0].str("sha256"))
	assert.Empty(t, s.find("POST", "/uploads")[0].str("sha256"), "nothing is hashed before the upload by default")
}

func TestRateLimitWaitsForTheNextWindow(t *testing.T) {
	r := newRateLimit()
	h := http.Header{}
	h.Set("RateLimit-Remaining", "1")
	h.Set("RateLimit-Reset", "0")
	now := time.Now()
	r.update(h, now)
	r.resetAt = now.Add(150 * time.Millisecond)
	start := time.Now()
	require.NoError(t, r.wait(context.Background())) // the last call of the window
	assert.Less(t, time.Since(start), 100*time.Millisecond)
	require.NoError(t, r.wait(context.Background())) // must wait for the next window
	assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
}

func TestRateLimitReadsBothHeaderForms(t *testing.T) {
	now := time.Now()
	r := newRateLimit()
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "7")
	h.Set("X-RateLimit-Reset", itoa(int(now.Add(30*time.Second).Unix())))
	r.update(h, now)
	assert.Equal(t, 7, r.remaining)
	assert.WithinDuration(t, now.Add(30*time.Second), r.resetAt, time.Second)
	h = http.Header{}
	h.Set("RateLimit-Remaining", "9") // a later answer of the same window can't raise it
	h.Set("RateLimit-Reset", "30")
	r.update(h, now)
	assert.Equal(t, 7, r.remaining)
}

func TestCallsFollowTheServersWindow(t *testing.T) {
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		return 200, `{"storage":{"used_bytes":1}}`
	})
	s.header.Set("RateLimit-Remaining", "0")
	s.header.Set("RateLimit-Reset", "1")
	_, err := f.About(context.Background())
	require.NoError(t, err)
	start := time.Now()
	_, err = f.About(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, time.Since(start), 800*time.Millisecond, "the second call waits for the next window")
}

func TestListRFallsBackWhenTooLarge(t *testing.T) {
	f, _ := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		if c.path != "/files" {
			return problem(404, "not_found")
		}
		switch {
		case strings.Contains(c.query, "recursive=true"):
			return problem(413, "too_large")
		case strings.Contains(c.query, "folder=d1"):
			return 200, `{"entries":[` + fileJSON("2", "g.txt") + `],"next_cursor":null}`
		case strings.Contains(c.query, "type=folder"):
			return 200, `{"entries":[{"id":"d1","type":"folder","name":"sub"}],"next_cursor":null}`
		default:
			return 200, `{"entries":[{"id":"d1","type":"folder","name":"sub"},` + fileJSON("1", "f.txt") + `],"next_cursor":null}`
		}
	})
	var got []string
	err := f.ListR(context.Background(), "", func(entries fs.DirEntries) error {
		for _, e := range entries {
			got = append(got, e.Remote())
		}
		return nil
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"sub", "f.txt", "sub/g.txt"}, got)
}

func TestHopelessUploadsAreNotRetried(t *testing.T) {
	for code, fatal := range map[string]bool{"file_too_large": false, "storage_limit_exceeded": false, "storage_full": true} {
		t.Run(code, func(t *testing.T) {
			f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
				if c.method == "POST" && c.path == "/uploads" {
					return problem(413, code)
				}
				return problem(404, "not_found")
			})
			o := &Object{fs: f, remote: "f.txt"}
			src := object.NewStaticObjectInfo("f.txt", time.Now(), 3, true, nil, nil)
			err := o.Update(context.Background(), bytes.NewReader([]byte("abc")), src)
			require.Error(t, err)
			assert.Len(t, s.find("POST", "/uploads"), 1, "asked once")
			if fatal {
				assert.True(t, fserrors.IsFatalError(err), "storage_full stops the run")
			} else {
				assert.True(t, fserrors.IsNoRetryError(err), "%s fails this file without retries", code)
			}
		})
	}
}

func TestKeepVersionsFalseUploadsSeparately(t *testing.T) {
	f, s := newScriptServer(t, map[string]string{"keep_versions": "false"}, func(s *scriptServer, c apiCall) (int, string) {
		if st, out, ok := singleUpload(s, c, 3); ok {
			return st, out
		}
		return problem(404, "not_found")
	})
	_, err := f.startUpload(context.Background(), "f.txt", "", 3, "text/plain", time.Time{}, "")
	require.NoError(t, err)
	assert.Equal(t, "separate", s.find("POST", "/uploads")[0].str("on_conflict"))
}

func TestErrorDecodesBothFormats(t *testing.T) {
	for name, body := range map[string]string{
		"problem details": `{"type":"https://tarkle.tech/docs/api/errors/name_taken","title":"Name taken","status":409,"detail":"A file named x exists","code":"name_taken","details":{"choices":["version"]}}`,
		"older envelope":  `{"error":{"code":"name_taken","message":"A file named x exists","details":{"choices":["version"]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp := &http.Response{StatusCode: 409, Body: io.NopCloser(strings.NewReader(body))}
			e := apiError(errorHandler(resp))
			require.NotNil(t, e)
			assert.Equal(t, "name_taken", e.Code())
			assert.Equal(t, "A file named x exists", e.Message())
			assert.JSONEq(t, `{"choices":["version"]}`, string(e.Details()))
			assert.True(t, choiceOffered(e, "version"))
		})
	}
}

func TestCloseResendsMissingPartsFromTheSource(t *testing.T) {
	completes := 0
	f, s := newScriptServer(t, nil, func(s *scriptServer, c apiCall) (int, string) {
		switch {
		case c.method == "PUT" && c.path == "/store/put":
			return 200, ``
		case c.method == "POST" && c.path == "/uploads/u1/complete":
			completes++
			if completes == 1 {
				return 409, `{"error":{"code":"incomplete","message":"m","details":{"missing_parts":[2]}}}`
			}
			return 200, fileJSON("7", "f.txt")
		}
		return problem(404, "not_found")
	})
	src := object.NewMemoryObject("f.txt", time.Now(), []byte("abcdef"))
	w := &chunkWriter{
		f:    f,
		src:  src,
		leaf: "f.txt",
		id:   "u1",
		plan: partPlan{size: 6, partSize: 3, count: 2},
		urls: map[int]string{1: s.url + "/store/put", 2: s.url + "/store/put"},
	}
	require.NoError(t, w.Close(context.Background()))
	puts := s.find("PUT", "/store/put")
	require.Len(t, puts, 1, "only the missing part is sent again")
	assert.Equal(t, 3, puts[0].size, "part 2 is the last 3 bytes")
	assert.Equal(t, 2, completes)
	assert.Equal(t, "7", w.entry.ID)
}
