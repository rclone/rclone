package dosya

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/dosya/api"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFsAdvertisesSHA256AndSecondPrecision(t *testing.T) {
	f := &Fs{}
	assert.Equal(t, time.Second, f.Precision())
	assert.True(t, f.Hashes().Contains(hash.SHA256), "SHA-256 must be advertised")
	// Exactly SHA-256, nothing else.
	assert.Equal(t, hash.NewHashSet(hash.SHA256), f.Hashes())
}

func TestObjectHashReturnsContentHash(t *testing.T) {
	o := &Object{file: api.FileItem{ContentHash: "abc123"}}

	got, err := o.Hash(context.Background(), hash.SHA256)
	require.NoError(t, err)
	assert.Equal(t, "abc123", got)

	// A multipart object with no server-side hash yields "" and no error,
	// exactly as an S3 multipart object does.
	empty := &Object{file: api.FileItem{ContentHash: ""}}
	got, err = empty.Hash(context.Background(), hash.SHA256)
	require.NoError(t, err)
	assert.Equal(t, "", got)

	// Any other algorithm is unsupported.
	_, err = o.Hash(context.Background(), hash.MD5)
	assert.ErrorIs(t, err, hash.ErrUnsupported)
}

func TestObjectModTimeUsesUpdatedAt(t *testing.T) {
	mt := time.Date(2003, 2, 3, 4, 5, 6, 0, time.UTC)
	o := &Object{file: api.FileItem{UpdatedAt: float64(mt.Unix()), CreatedAt: 1}}
	assert.True(t, mt.Equal(o.ModTime(context.Background())))
}

func TestModTimeHeaders(t *testing.T) {
	assert.Nil(t, modTimeHeaders(time.Time{}), "a zero time sends no header")

	mt := time.Date(2003, 2, 3, 4, 5, 6, 499, time.UTC)
	h := modTimeHeaders(mt)
	require.NotNil(t, h)
	assert.Equal(t, strconv.FormatInt(mt.Unix(), 10), h[sourceMTimeHeader])
}

func TestFileFromUploadKeepsModTimeAndHash(t *testing.T) {
	resp := &api.UploadCompleteResponse{}
	resp.File.ID = "fil_1"
	resp.File.Name = "x.txt"
	resp.File.SizeBytes = 10
	resp.File.CreatedAt = 1_000
	resp.File.ContentHash = "deadbeef"

	mt := time.Date(2004, 3, 3, 4, 5, 6, 0, time.UTC)
	item := fileFromUpload(resp, mt)
	assert.Equal(t, float64(mt.Unix()), item.UpdatedAt, "modtime is what we sent, not the create time")
	assert.Equal(t, "deadbeef", item.ContentHash)
	assert.Equal(t, float64(1_000), item.CreatedAt)

	// A zero modtime falls back to the create time.
	item = fileFromUpload(resp, time.Time{})
	assert.Equal(t, float64(1_000), item.UpdatedAt)
}

// TestUploadSendsSourceMTimeHeader drives a real single-PUT upload through a
// fake transport and asserts the source-mtime header actually rides on the PUT.
func TestUploadSendsSourceMTimeHeader(t *testing.T) {
	var putMTime string
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.Body != nil {
			_, _ = io.ReadAll(r.Body) // drain so the upload's counting reader advances
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/upload/init":
			return jsonResp(http.StatusCreated, `{"ok":true,"session_id":"upl_1"}`), nil
		case r.Method == "PUT" && r.URL.Path == "/api/upload/upl_1":
			putMTime = r.Header.Get(sourceMTimeHeader)
			return jsonResp(http.StatusCreated, `{"ok":true,"file":{"id":"fil_1","name":"file.txt","size_bytes":5,"content_hash":"h"}}`), nil
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		return nil, nil
	})
	f := newFakeFs(rt)

	mt := time.Date(2003, 2, 3, 4, 5, 6, 0, time.UTC)
	resp, err := f.uploadFile(context.Background(), strings.NewReader("hello"), "file.txt", 5, mt, "", nil)
	require.NoError(t, err)
	assert.Equal(t, "h", resp.File.ContentHash)
	assert.Equal(t, strconv.FormatInt(mt.Unix(), 10), putMTime, "the PUT must carry the source-mtime header")
}

// TestMultipartCompleteSendsSourceMTimeHeader asserts the header also rides on
// the completion request for a multipart upload (the server stores the modtime
// there, not on the individual parts).
func TestMultipartCompleteSendsSourceMTimeHeader(t *testing.T) {
	var completeMTime string
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		if r.Body != nil {
			_, _ = io.ReadAll(r.Body)
		}
		switch {
		case r.URL.Path == "/api/upload/init":
			return jsonResp(http.StatusCreated, `{"ok":true,"session_id":"upl_1","resumable":{"part_size":4,"total_parts":3}}`), nil
		case strings.HasPrefix(r.URL.Path, "/api/upload/upl_1/part/"):
			return jsonResp(http.StatusOK, `{"ok":true}`), nil
		case r.URL.Path == "/api/upload/upl_1/complete":
			completeMTime = r.Header.Get(sourceMTimeHeader)
			return jsonResp(http.StatusOK, `{"ok":true,"file":{"id":"fil_1","name":"file.txt","size_bytes":10}}`), nil
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		return nil, nil
	})
	f := newFakeFs(rt)

	mt := time.Date(2003, 2, 3, 4, 5, 6, 0, time.UTC)
	_, err := f.uploadFile(context.Background(), strings.NewReader("0123456789"), "file.txt", 10, mt, "", nil)
	require.NoError(t, err)
	assert.Equal(t, strconv.FormatInt(mt.Unix(), 10), completeMTime, "the complete call must carry the source-mtime header")
}

// TestCopyReturnsSourceModTimeAndHash guards the fingerprint of the object a
// server-side Copy returns: it must report the SOURCE's modtime and hash (the
// copy is byte-identical and the API preserves both), not the copy's wall
// clock. Reading "now" here made rclone's in-memory object disagree with the
// one read back from the remote and failed FsCopy's fingerprint check.
func TestCopyReturnsSourceModTimeAndHash(t *testing.T) {
	rt := rtFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/files":
			// NewObject's existence probe on the destination: nothing there.
			return jsonResp(http.StatusOK, `{"ok":true,"files":[],"folders":[]}`), nil
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/copy"):
			return jsonResp(http.StatusOK, `{"ok":true,"file_id":"fil_copy","name":"f-copy.txt"}`), nil
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		return nil, nil
	})
	f := newFakeFs(rt)
	f.dirCache = dircache.New("", rootID, f)

	mt := time.Date(2001, 2, 3, 4, 5, 10, 0, time.UTC)
	ct := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	src := &Object{fs: f, remote: "orig.txt", file: api.FileItem{
		ID: "fil_src", Name: "orig.txt", SizeBytes: 100, MimeType: "text/plain",
		CreatedAt: float64(ct.Unix()), UpdatedAt: float64(mt.Unix()), ContentHash: "srchash",
	}}

	dst, err := f.Copy(context.Background(), src, "f-copy.txt")
	require.NoError(t, err)

	assert.True(t, mt.Equal(dst.ModTime(context.Background())), "copy keeps the source modtime")
	assert.Equal(t, int64(100), dst.Size())
	got, err := dst.Hash(context.Background(), hash.SHA256)
	require.NoError(t, err)
	assert.Equal(t, "srchash", got, "copy keeps the source hash")
}
