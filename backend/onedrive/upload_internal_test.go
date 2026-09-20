package onedrive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fragmentServer is a fake OneDrive upload session which records the
// fragments it receives. fail is called for each PUT before the body is
// accepted and may write an error response, returning true to reject it.
type fragmentServer struct {
	t         *testing.T
	size      int64
	mu        sync.Mutex
	received  []byte // bytes accepted so far
	fragments int    // number of accepted fragments
	fail      func(w http.ResponseWriter, attempt int) bool
	attempts  int
}

func (s *fragmentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == "POST" && r.URL.Path == "/root/createUploadSession":
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"uploadUrl":"http://%s/upload","nextExpectedRanges":["0-"]}`, r.Host)
	case r.Method == "GET" && r.URL.Path == "/upload":
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"nextExpectedRanges":["%d-%d"]}`, len(s.received), s.size-1)
	case r.Method == "PUT" && r.URL.Path == "/upload":
		body, err := io.ReadAll(r.Body)
		require.NoError(s.t, err)
		s.attempts++
		if s.fail != nil && s.fail(w, s.attempts) {
			return
		}
		var start, end, total int64
		_, err = fmt.Sscanf(r.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total)
		require.NoError(s.t, err)
		assert.Equal(s.t, int64(len(s.received)), start, "fragment sent out of order")
		assert.Equal(s.t, int64(len(body)), end-start+1, "Content-Range disagrees with body length")
		assert.Equal(s.t, s.size, total)
		s.received = append(s.received, body...)
		s.fragments++
		w.Header().Set("Content-Type", "application/json")
		if int64(len(s.received)) == s.size {
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"id":"fake-id","name":"remote","size":%d,"file":{}}`, s.size)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprintf(w, `{"nextExpectedRanges":["%d-%d"]}`, len(s.received), s.size-1)
	default:
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

// newUploadTest returns an Object wired to a fragmentServer which
// uploads content in chunks of chunkSize.
func newUploadTest(t *testing.T, content []byte, chunkSize int) (*Object, *fragmentServer, *httptest.Server) {
	s := &fragmentServer{t: t, size: int64(len(content))}
	server := httptest.NewServer(s)
	t.Cleanup(server.Close)
	client := rest.NewClient(server.Client()).SetRoot(server.URL)
	f := &Fs{
		ci:     fs.GetConfig(context.Background()),
		srv:    client,
		unAuth: client,
		pacer:  fs.NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}
	f.opt.ChunkSize = fs.SizeSuffix(chunkSize)
	f.opt.UploadCutoff = fs.SizeSuffix(chunkSize)
	return &Object{fs: f}, s, server
}

// TestUploadMultipartSkip checks that a 416 response makes the upload
// read the server's position and re-send only the unreceived tail of
// the fragment.
func TestUploadMultipartSkip(t *testing.T) {
	content := bytes.Repeat([]byte("onedrive"), 1024)
	chunkSize := len(content) / 2
	skip := chunkSize / 4
	o, s, _ := newUploadTest(t, content, chunkSize)
	s.fail = func(w http.ResponseWriter, attempt int) bool {
		if attempt == 1 {
			// Pretend the first part of the fragment was received
			// before the connection broke
			s.received = append(s.received, content[:skip]...)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "invalidRange", "message": "bad range"}})
			return true
		}
		return false
	}
	src := object.NewStaticObjectInfo("remote", time.Now(), int64(len(content)), true, nil, nil)
	info, err := o.uploadMultipart(context.Background(), bytes.NewReader(content), src)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, content, s.received)
	assert.Equal(t, 2, s.fragments)
	assert.Equal(t, 3, s.attempts, "expected one 416, a partial re-send and the final fragment")
}
