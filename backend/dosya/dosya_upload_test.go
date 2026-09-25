package dosya

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rtFunc adapts a function into an http.RoundTripper so the upload path
// can be driven without a network. Unlike a real server it always reads
// the whole request body and answers, which is exactly the misbehaviour
// (accepting a short body as a complete upload) these tests guard against.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func newFakeFs(rt http.RoundTripper) *Fs {
	ctx := context.Background()
	f := &Fs{
		name: "test",
		opt:  Options{WorkspaceID: "ws_test"},
		pacer: fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(time.Millisecond),
			pacer.MaxSleep(time.Millisecond),
			pacer.DecayConstant(decayConstant),
			pacer.AttackConstant(attackConstant),
		)),
	}
	f.rest = rest.NewClient(&http.Client{Transport: rt}).SetRoot("https://fake.invalid").SetErrorHandler(errorHandler)
	return f
}

// fakeUploadServer records what an upload sends and lets a test inject
// failures per request
type fakeUploadServer struct {
	t          *testing.T
	mu         sync.Mutex
	resumable  string           // JSON for the init "resumable" field, "" for a single PUT
	puts       [][]byte         // bodies of PUT /api/upload/<id>
	parts      map[int][][]byte // bodies of PUT /api/upload/<id>/part/<n>, per part, per attempt
	completes  int
	failFirst  map[string]bool // path -> answer the first request with a 502
	seenBefore map[string]bool
}

func newFakeUploadServer(t *testing.T, resumable string) *fakeUploadServer {
	return &fakeUploadServer{
		t:          t,
		resumable:  resumable,
		parts:      map[int][][]byte{},
		failFirst:  map[string]bool{},
		seenBefore: map[string]bool{},
	}
}

func (s *fakeUploadServer) roundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body []byte
	if r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		require.NoError(s.t, err)
	}
	path := r.URL.Path
	// Bodies are recorded for every attempt, including the ones answered
	// with a 502, so tests can see exactly what each attempt carried.
	fail := s.failFirst[path] && !s.seenBefore[path]
	s.seenBefore[path] = true
	bad := jsonResp(http.StatusBadGateway, `{"ok":false,"error":"bad gateway"}`)
	switch {
	case r.Method == "POST" && path == "/api/upload/init":
		if s.resumable != "" {
			return jsonResp(http.StatusCreated, `{"ok":true,"session_id":"upl_1","resumable":`+s.resumable+`}`), nil
		}
		return jsonResp(http.StatusCreated, `{"ok":true,"session_id":"upl_1"}`), nil
	case r.Method == "PUT" && path == "/api/upload/upl_1":
		s.puts = append(s.puts, body)
		if fail {
			return bad, nil
		}
		return jsonResp(http.StatusCreated, fmt.Sprintf(`{"ok":true,"file":{"id":"fil_1","name":"file.txt","size_bytes":%d}}`, len(body))), nil
	case r.Method == "PUT" && strings.HasPrefix(path, "/api/upload/upl_1/part/"):
		var n int
		_, err := fmt.Sscanf(strings.TrimPrefix(path, "/api/upload/upl_1/part/"), "%d", &n)
		require.NoError(s.t, err)
		s.parts[n] = append(s.parts[n], body)
		if fail {
			return bad, nil
		}
		return jsonResp(http.StatusOK, fmt.Sprintf(`{"ok":true,"part_number":%d,"bytes_uploaded":%d}`, n, len(body))), nil
	case r.Method == "POST" && path == "/api/upload/upl_1/complete":
		s.completes++
		return jsonResp(http.StatusOK, `{"ok":true,"file":{"id":"fil_1","name":"file.txt","size_bytes":10}}`), nil
	}
	s.t.Fatalf("unexpected request %s %s", r.Method, path)
	return nil, nil
}

func TestUploadSmallSendsWholeBody(t *testing.T) {
	srv := newFakeUploadServer(t, "")
	f := newFakeFs(rtFunc(srv.roundTrip))
	content := strings.Repeat("a", 100)

	resp, err := f.uploadFile(context.Background(), strings.NewReader(content), "file.txt", 100, time.Time{}, "", nil)
	require.NoError(t, err)
	assert.Equal(t, "fil_1", resp.File.ID)
	require.Len(t, srv.puts, 1)
	assert.Equal(t, content, string(srv.puts[0]))
}

func TestUploadSmallRejectsShortSource(t *testing.T) {
	// The source declares 100 bytes but only supplies 60. A server which
	// accepts that as a complete upload must not turn into a success.
	srv := newFakeUploadServer(t, "")
	f := newFakeFs(rtFunc(srv.roundTrip))

	_, err := f.uploadFile(context.Background(), strings.NewReader(strings.Repeat("a", 60)), "file.txt", 100, time.Time{}, "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Contains(t, err.Error(), "expected 100 bytes in input, but got 60")
}

func TestUploadSmallDoesNotResendDrainedBody(t *testing.T) {
	// A retried PUT would resend an already-consumed reader, i.e. an
	// empty body, which the server would store as an empty file. The
	// single PUT therefore must not be retried at the low level.
	srv := newFakeUploadServer(t, "")
	srv.failFirst["/api/upload/upl_1"] = true
	f := newFakeFs(rtFunc(srv.roundTrip))

	content := strings.Repeat("a", 100)
	_, err := f.uploadFile(context.Background(), strings.NewReader(content), "file.txt", 100, time.Time{}, "", nil)
	require.Error(t, err)
	require.Len(t, srv.puts, 1, "the PUT must not be retried with a drained body")
	assert.Equal(t, content, string(srv.puts[0]))
}

const threeParts = `{"part_size":4,"total_parts":3}`

func TestUploadMultipartAssemblesAllBytes(t *testing.T) {
	srv := newFakeUploadServer(t, threeParts)
	f := newFakeFs(rtFunc(srv.roundTrip))
	content := "0123456789"

	resp, err := f.uploadFile(context.Background(), strings.NewReader(content), "file.txt", 10, time.Time{}, "", nil)
	require.NoError(t, err)
	assert.Equal(t, "fil_1", resp.File.ID)
	assert.Equal(t, 1, srv.completes)
	require.Len(t, srv.parts, 3)
	assert.Equal(t, "0123", string(srv.parts[1][0]))
	assert.Equal(t, "4567", string(srv.parts[2][0]))
	assert.Equal(t, "89", string(srv.parts[3][0]))
}

func TestUploadMultipartRejectsShortSource(t *testing.T) {
	// 10 bytes declared, 8 supplied: the last part must never be sent
	// and the upload must not be completed.
	srv := newFakeUploadServer(t, threeParts)
	f := newFakeFs(rtFunc(srv.roundTrip))

	_, err := f.uploadFile(context.Background(), strings.NewReader("01234567"), "file.txt", 10, time.Time{}, "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Contains(t, err.Error(), "expected 10 bytes in input, but got 8")
	assert.Empty(t, srv.parts[3], "short final part must not be uploaded")
	assert.Equal(t, 0, srv.completes, "upload must not be completed")
}

func TestUploadMultipartRetriesPartWithFullBody(t *testing.T) {
	// A part which fails transiently must be resent with all its bytes,
	// not with whatever is left of a consumed reader.
	srv := newFakeUploadServer(t, threeParts)
	srv.failFirst["/api/upload/upl_1/part/2"] = true
	f := newFakeFs(rtFunc(srv.roundTrip))

	_, err := f.uploadFile(context.Background(), strings.NewReader("0123456789"), "file.txt", 10, time.Time{}, "", nil)
	require.NoError(t, err)
	require.Len(t, srv.parts[2], 2, "part 2 should have been sent twice")
	assert.Equal(t, "4567", string(srv.parts[2][0]))
	assert.Equal(t, "4567", string(srv.parts[2][1]))
	assert.Equal(t, "89", string(srv.parts[3][0]))
	assert.Equal(t, 1, srv.completes)
}
