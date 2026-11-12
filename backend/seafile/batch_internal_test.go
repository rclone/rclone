package seafile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/seafile/api"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/batcher"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

// uploadRequest is an upload request received by the test server
type uploadRequest struct {
	relativePath string
	files        map[string]string // name to content
	names        []string          // names in the order received
}

// testServer is a minimal Seafile upload API
type testServer struct {
	*httptest.Server
	mu         sync.Mutex
	uploads    []uploadRequest
	failStatus int    // if set, every upload fails with this status
	refuseName string // if set, an upload containing this name fails with 400
}

func newTestServer(t *testing.T) *testServer {
	s := &testServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api2/repos/{library}/upload-link/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(s.URL + "/upload-api/" + r.PathValue("library"))
	})
	mux.HandleFunc("POST /upload-api/{library}", s.upload)
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func (s *testServer) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req := uploadRequest{
		relativePath: r.FormValue("relative_path"),
		files:        map[string]string{},
	}
	var details []api.FileDetail
	for _, fh := range r.MultipartForm.File["file"] {
		in, err := fh.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data, _ := io.ReadAll(in)
		_ = in.Close()
		// The Go fileserver of Seafile stores the names in Unicode NFC
		name := norm.NFC.String(fh.Filename)
		req.files[name] = string(data)
		req.names = append(req.names, name)
		details = append(details, api.FileDetail{ID: "id-" + name, Name: name, Size: int64(len(data))})
	}
	s.mu.Lock()
	s.uploads = append(s.uploads, req)
	failStatus, refuseName := s.failStatus, s.refuseName
	s.mu.Unlock()
	if failStatus != 0 {
		http.Error(w, `{"error": "failed"}`, failStatus)
		return
	}
	if _, found := req.files[refuseName]; found && refuseName != "" {
		http.Error(w, `{"error": "invalid fileName"}`, http.StatusBadRequest)
		return
	}
	// The C fileserver of Seafile returns the files in reverse order
	slices.Reverse(details)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(details)
}

func (s *testServer) getUploads() []uploadRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]uploadRequest(nil), s.uploads...)
}

// newTestFs makes an Fs talking to the test server with the batch options given
func newTestFs(ctx context.Context, t *testing.T, s *testServer, mode string, size int) *Fs {
	f := &Fs{
		name: "test",
		opt: Options{
			Enc: encoder.EncodeZero | encoder.EncodeSlash | encoder.EncodeDot,
		},
		srv:   rest.NewClient(s.Client()).SetRoot(s.URL + "/"),
		pacer: getPacer(ctx, s.URL),
	}
	opt := defaultBatcherOptions
	opt.Mode = mode
	opt.Size = size
	opt.Timeout = time.Minute // batches are only sent when full or on shutdown
	var err error
	f.batcher, err = batcher.New(ctx, f, f.commitBatch, opt)
	require.NoError(t, err)
	t.Cleanup(f.batcher.Shutdown)
	return f
}

func newBatchItems(paths ...string) []*batchItem {
	items := make([]*batchItem, len(paths))
	for i, p := range paths {
		items[i] = &batchItem{libraryID: "lib", filePath: p, data: []byte("data of " + p)}
	}
	return items
}

func TestCommitBatchOneRequestPerDirectory(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	f := newTestFs(ctx, t, s, "off", 0)

	items := newBatchItems("a/2.txt", "b/1.txt", "a/1.txt", "3.txt", "a/2.txt", "b/2.txt")
	results := make([]*api.FileDetail, len(items))
	errs := make([]error, len(items))
	require.NoError(t, f.commitBatch(ctx, items, results, errs))

	for i, item := range items {
		require.NoError(t, errs[i], item.filePath)
		require.NotNil(t, results[i], item.filePath)
		assert.Equal(t, path.Base(item.filePath), results[i].Name, item.filePath)
		assert.Equal(t, int64(len(item.data)), results[i].Size, item.filePath)
	}
	uploads := s.getUploads()
	require.Len(t, uploads, 4)
	// the second a/2.txt goes into a request of its own
	assert.Equal(t, "a/", uploads[0].relativePath)
	assert.Equal(t, []string{"2.txt", "1.txt"}, uploads[0].names)
	assert.Equal(t, "data of a/2.txt", uploads[0].files["2.txt"])
	assert.Equal(t, "b/", uploads[1].relativePath)
	assert.Equal(t, []string{"1.txt", "2.txt"}, uploads[1].names)
	assert.Equal(t, "", uploads[2].relativePath)
	assert.Equal(t, []string{"3.txt"}, uploads[2].names)
	assert.Equal(t, "a/", uploads[3].relativePath)
	assert.Equal(t, []string{"2.txt"}, uploads[3].names)
}

func TestCommitBatchRefusedFileGetsItsOwnError(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	s.refuseName = "bad.txt"
	f := newTestFs(ctx, t, s, "off", 0)

	items := newBatchItems("dir/1.txt", "dir/bad.txt", "dir/2.txt")
	results := make([]*api.FileDetail, len(items))
	errs := make([]error, len(items))
	require.NoError(t, f.commitBatch(ctx, items, results, errs))

	assert.NoError(t, errs[0])
	assert.Error(t, errs[1])
	assert.Nil(t, results[1])
	assert.NoError(t, errs[2])
	// one batch request, then one request per file
	assert.Len(t, s.getUploads(), 1+len(items))
}

func TestCommitBatchServerErrorFailsEveryFile(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	s.failStatus = http.StatusInternalServerError
	f := newTestFs(ctx, t, s, "off", 0)

	items := newBatchItems("dir/1.txt", "dir/2.txt")
	results := make([]*api.FileDetail, len(items))
	errs := make([]error, len(items))
	require.NoError(t, f.commitBatch(ctx, items, results, errs))

	for i := range items {
		require.Error(t, errs[i])
		assert.True(t, fserrors.IsRetryError(errs[i]), "error should be retried by the caller: %v", errs[i])
		assert.Nil(t, results[i])
	}
	// a server error isn't about a single file so the files aren't tried one by one
	assert.Len(t, s.getUploads(), 1)
}

func updateObjects(ctx context.Context, f *Fs, contents map[string]string) map[string]error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs = map[string]error{}
	)
	for remote, content := range contents {
		wg.Go(func() {
			o := &Object{fs: f, libraryID: "lib", remote: remote, pathInLibrary: remote}
			src := object.NewStaticObjectInfo(remote, time.Now(), int64(len(content)), true, nil, nil)
			err := o.Update(ctx, strings.NewReader(content), src)
			if err == nil && o.size != int64(len(content)) {
				err = fmt.Errorf("size is %d, want %d", o.size, len(content))
			}
			mu.Lock()
			errs[remote] = err
			mu.Unlock()
		})
	}
	wg.Wait()
	return errs
}

func TestUpdateSyncBatch(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	const n = 5
	f := newTestFs(ctx, t, s, "sync", n)

	contents := map[string]string{}
	for i := range n {
		contents[fmt.Sprintf("dir/file%d.txt", i)] = strings.Repeat("x", i)
	}
	for remote, err := range updateObjects(ctx, f, contents) {
		assert.NoError(t, err, remote)
	}
	uploads := s.getUploads()
	require.Len(t, uploads, 1)
	assert.Len(t, uploads[0].files, n)
	assert.Equal(t, "xxx", uploads[0].files["file3.txt"])
}

func TestUpdateAsyncBatchIsSentOnShutdown(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	f := newTestFs(ctx, t, s, "async", 100)

	errs := updateObjects(ctx, f, map[string]string{"1.txt": "one", "2.txt": "two"})
	for remote, err := range errs {
		assert.NoError(t, err, remote)
	}
	assert.Len(t, s.getUploads(), 0)
	require.NoError(t, f.Shutdown(ctx))
	uploads := s.getUploads()
	require.Len(t, uploads, 1)
	assert.Equal(t, map[string]string{"1.txt": "one", "2.txt": "two"}, uploads[0].files)
}

func TestUpdateLargeFileIsNotBatched(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	f := newTestFs(ctx, t, s, "sync", 2)

	content := bytes.Repeat([]byte("x"), int(batchMaxFileSize)+1)
	o := &Object{fs: f, libraryID: "lib", remote: "big.bin", pathInLibrary: "big.bin"}
	src := object.NewStaticObjectInfo("big.bin", time.Now(), int64(len(content)), true, nil, nil)
	// a batched file would wait for a second file or the timeout of a minute
	done := make(chan error, 1)
	go func() { done <- o.Update(ctx, bytes.NewReader(content), src) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("large file was held back for a batch")
	}
	uploads := s.getUploads()
	require.Len(t, uploads, 1)
	assert.Len(t, uploads[0].files["big.bin"], len(content))
}

func TestUpdateBatchedShortInput(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	f := newTestFs(ctx, t, s, "sync", 2)

	o := &Object{fs: f, libraryID: "lib", remote: "short.txt", pathInLibrary: "short.txt"}
	src := object.NewStaticObjectInfo("short.txt", time.Now(), 10, true, nil, nil)
	require.Error(t, o.Update(ctx, strings.NewReader("short"), src))
	assert.Len(t, s.getUploads(), 0)
}

func TestCommitBatchMatchesNormalizedNames(t *testing.T) {
	ctx := context.Background()
	s := newTestServer(t)
	f := newTestFs(ctx, t, s, "off", 0)

	nfd := norm.NFD.String("dir/caf\u00e9.txt")
	items := newBatchItems(nfd, "dir/z.txt")
	results := make([]*api.FileDetail, len(items))
	errs := make([]error, len(items))
	require.NoError(t, f.commitBatch(ctx, items, results, errs))
	for i := range items {
		require.NoError(t, errs[i])
		assert.Equal(t, int64(len(items[i].data)), results[i].Size)
	}
}
