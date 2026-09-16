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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parallelUploadServer answers the multipart endpoints without holding a
// lock across a request, so parts really do overlap, and records what the
// ordering rules below are about: how many parts were in flight at once, and
// whether any part started before part 1 had finished.
//
// Part 1 matters because the API creates the R2 multipart upload (and, for a
// new file, its id and key) on the first part it sees. Parts that race ahead
// of part 1 would each create their own upload, and only one of those would
// be completed.
type parallelUploadServer struct {
	t          *testing.T
	resumable  string
	partDelay  time.Duration
	failPart   int // answer this part with a 400, 0 for none
	mu         sync.Mutex
	inFlight   int
	maxFlight  int
	part1Done  bool
	earlyParts []int // parts that started before part 1 finished
	parts      map[int]string
	completes  int
}

func (s *parallelUploadServer) roundTrip(r *http.Request) (*http.Response, error) {
	var body []byte
	if r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		require.NoError(s.t, err)
	}
	path := r.URL.Path
	switch {
	case r.Method == "POST" && path == "/api/upload/init":
		return jsonResp(http.StatusCreated, `{"ok":true,"session_id":"upl_1","resumable":`+s.resumable+`}`), nil
	case r.Method == "PUT" && strings.HasPrefix(path, "/api/upload/upl_1/part/"):
		var n int
		_, err := fmt.Sscanf(strings.TrimPrefix(path, "/api/upload/upl_1/part/"), "%d", &n)
		require.NoError(s.t, err)

		s.mu.Lock()
		if n != 1 && !s.part1Done {
			s.earlyParts = append(s.earlyParts, n)
		}
		s.inFlight++
		s.maxFlight = max(s.maxFlight, s.inFlight)
		s.mu.Unlock()

		time.Sleep(s.partDelay)

		s.mu.Lock()
		s.inFlight--
		if n == 1 {
			s.part1Done = true
		}
		s.parts[n] = string(body)
		fail := n == s.failPart
		s.mu.Unlock()

		if fail {
			return jsonResp(http.StatusBadRequest, `{"ok":false,"error":"part refused"}`), nil
		}
		return jsonResp(http.StatusCreated, fmt.Sprintf(`{"ok":true,"part_number":%d,"etag":"e%d"}`, n, n)), nil
	case r.Method == "POST" && path == "/api/upload/upl_1/complete":
		s.mu.Lock()
		s.completes++
		s.mu.Unlock()
		return jsonResp(http.StatusOK, `{"ok":true,"file":{"id":"fil_1","name":"file.txt","size_bytes":20}}`), nil
	}
	s.t.Fatalf("unexpected request %s %s", r.Method, path)
	return nil, nil
}

func newParallelServer(t *testing.T, resumable string) *parallelUploadServer {
	return &parallelUploadServer{t: t, resumable: resumable, partDelay: 50 * time.Millisecond, parts: map[int]string{}}
}

const tenParts = `{"part_size":2,"total_parts":10}`

func TestUploadMultipartSendsPartsConcurrently(t *testing.T) {
	srv := newParallelServer(t, tenParts)
	f := newFakeFs(rtFunc(srv.roundTrip))
	f.opt.UploadConcurrency = 4
	content := "aabbccddeeffgghhiijj"

	_, err := f.uploadFile(context.Background(), strings.NewReader(content), "file.txt", 20, time.Time{}, "", nil)
	require.NoError(t, err)

	assert.Equal(t, 4, srv.maxFlight, "parts after the first should use the whole concurrency")
	assert.Empty(t, srv.earlyParts, "no part may start before part 1 has finished")
	assert.Equal(t, 1, srv.completes)
	for i := 1; i <= 10; i++ {
		assert.Equal(t, content[2*(i-1):2*i], srv.parts[i], "part %d carries its own bytes", i)
	}
}

func TestUploadMultipartConcurrencyOneIsSequential(t *testing.T) {
	srv := newParallelServer(t, tenParts)
	f := newFakeFs(rtFunc(srv.roundTrip))
	f.opt.UploadConcurrency = 1

	_, err := f.uploadFile(context.Background(), strings.NewReader("aabbccddeeffgghhiijj"), "file.txt", 20, time.Time{}, "", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, srv.maxFlight)
	assert.Equal(t, 1, srv.completes)
}

func TestUploadMultipartFailedPartIsNotCompleted(t *testing.T) {
	srv := newParallelServer(t, tenParts)
	srv.failPart = 6
	f := newFakeFs(rtFunc(srv.roundTrip))
	f.opt.UploadConcurrency = 4

	_, err := f.uploadFile(context.Background(), strings.NewReader("aabbccddeeffgghhiijj"), "file.txt", 20, time.Time{}, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "part 6")
	assert.Equal(t, 0, srv.completes, "an upload with a failed part must not be completed")
}

func TestUploadMultipartParallelRejectsShortSource(t *testing.T) {
	// 20 bytes declared, 15 supplied: the short read is found by the
	// reader, whichever parts are already in flight, and nothing completes.
	srv := newParallelServer(t, tenParts)
	f := newFakeFs(rtFunc(srv.roundTrip))
	f.opt.UploadConcurrency = 4

	_, err := f.uploadFile(context.Background(), strings.NewReader("aabbccddeeffggh"), "file.txt", 20, time.Time{}, "", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, 0, srv.completes)
	assert.Empty(t, srv.parts[8], "a part the source never filled must not be sent")
}
