package s3

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/gofakes3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionalPutMissingIfMatch(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	ctx := conditionalContext(`"missing"`, "")

	_, err := cb.PutObject(ctx, "bucket", "missing.txt", map[string]string{}, strings.NewReader("x"), 1)
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrNoSuchKey), "got %v", err)

}

func TestConditionalCopyObject(t *testing.T) {
	ctx := context.Background()
	b, root := newTestBackend(t)
	cb := newConditionalBackend(b)

	const destination = "copy-dst.txt"
	const original = "destination before copy"
	_, err := b.PutObject(ctx, "bucket", destination, map[string]string{}, strings.NewReader(original), int64(len(original)))
	require.NoError(t, err)
	oldETag := objectETag(t, b, "bucket", destination)
	fileName := filepath.Join(root, "bucket", destination)

	t.Run("StaleIfMatch", func(t *testing.T) {
		conditionalCtx := conditionalContext(`"stale"`, "")
		_, err := cb.CopyObject(conditionalCtx, "bucket", "object.txt", "bucket", destination, map[string]string{})
		assert.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)

		got, readErr := os.ReadFile(fileName)
		require.NoError(t, readErr)
		assert.Equal(t, original, string(got))
	})

	t.Run("MatchingIfMatch", func(t *testing.T) {
		conditionalCtx := conditionalContext(oldETag, "")
		_, err := cb.CopyObject(conditionalCtx, "bucket", "object.txt", "bucket", destination, map[string]string{})
		require.NoError(t, err)

		got, readErr := os.ReadFile(fileName)
		require.NoError(t, readErr)
		assert.Equal(t, "normal object", string(got))
	})

	t.Run("IfNoneMatchCreateOnly", func(t *testing.T) {
		conditionalCtx := conditionalContext("", "*")
		_, err := cb.CopyObject(conditionalCtx, "bucket", "object.txt", "bucket", "copy-new.txt", map[string]string{})
		require.NoError(t, err)

		conditionalCtx = conditionalContext("", "*")
		_, err = cb.CopyObject(conditionalCtx, "bucket", "object.txt", "bucket", "copy-new.txt", map[string]string{})
		assert.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)

	})
}

func TestConditionalHTTP(t *testing.T) {
	b, root := newTestBackend(t)
	cb := newConditionalBackend(b)
	require.NoError(t, os.WriteFile(filepath.Join(root, "bucket", "source.txt"), []byte("copy source"), 0600))
	handler := withConditions(gofakes3.New(cb, gofakes3.WithHostBucket(false), gofakes3.WithoutVersioning()).Server())
	for _, test := range []struct {
		name, key, match, none, copySource string
		status                             int
		code                               string
	}{
		{"StalePut", "object.txt", `"stale"`, "", "", 412, "PreconditionFailed"},
		{"CreateOnlyPut", "object.txt", "", "*", "", 412, "PreconditionFailed"},
		{"MissingPut", "missing.txt", `"stale"`, "", "", 404, "NoSuchKey"},
		{"StaleCopy", "object.txt", `"stale"`, "", "/bucket/source.txt", 412, "PreconditionFailed"},
		{"CreateOnlyCopy", "object.txt", "", "*", "/bucket/source.txt", 412, "PreconditionFailed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/bucket/"+test.key, strings.NewReader("replacement"))
			req.Header.Set("Content-Length", "11")
			if test.match != "" {
				req.Header.Set("If-Match", test.match)
			}
			if test.none != "" {
				req.Header.Set("If-None-Match", test.none)
			}
			if test.copySource != "" {
				req.Header.Set("X-Amz-Copy-Source", test.copySource)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, test.status, rec.Code, rec.Body.String())
			var response struct {
				Code string
			}
			require.NoError(t, xml.Unmarshal(rec.Body.Bytes(), &response))
			assert.Equal(t, test.code, response.Code)
			assert.NotEmpty(t, rec.Header().Get("X-Amz-Request-Id"))
			got, err := os.ReadFile(filepath.Join(root, "bucket", "object.txt"))
			require.NoError(t, err)
			assert.Equal(t, "normal object", string(got))
		})
	}
}

func TestConditionalMultipart(t *testing.T) {
	b, root := newTestBackend(t)
	cb := newConditionalBackend(b)
	ctx := context.Background()
	etag := objectETag(t, b, "bucket", "object.txt")
	id, err := cb.CreateMultipartUpload(ctx, "bucket", "object.txt", map[string]string{})
	require.NoError(t, err)
	partETag, err := cb.UploadPart(ctx, "bucket", "object.txt", id, 1, 11, strings.NewReader("replacement"))
	require.NoError(t, err)
	var input gofakes3.CompleteMultipartUploadRequest
	require.NoError(t, xml.Unmarshal([]byte(`<CompleteMultipartUpload><Part><PartNumber>1</PartNumber><ETag>`+partETag+`</ETag></Part></CompleteMultipartUpload>`), &input))
	conditional := conditionalContext(`"stale"`, "")
	_, _, err = cb.CompleteMultipartUpload(conditional, "bucket", "object.txt", id, &input)
	require.ErrorIs(t, err, gofakes3.ErrPreconditionFailed)
	got, err := os.ReadFile(filepath.Join(root, "bucket", "object.txt"))
	require.NoError(t, err)
	assert.Equal(t, "normal object", string(got))
	conditional = conditionalContext(etag, "")
	_, _, err = cb.CompleteMultipartUpload(conditional, "bucket", "other.txt", id, &input)
	require.Equal(t, gofakes3.ErrNoSuchUpload, err)
	_, _, err = cb.CompleteMultipartUpload(conditional, "bucket", "object.txt", id, &input)
	require.NoError(t, err)
	got, err = os.ReadFile(filepath.Join(root, "bucket", "object.txt"))
	require.NoError(t, err)
	assert.Equal(t, "replacement", string(got))
	entries, err := os.ReadDir(filepath.Join(root, "bucket"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "object.txt", entries[0].Name())
}

func TestConditionalDeleteMultiDuplicate(t *testing.T) {
	b, _ := newTestBackend(t)
	cb := newConditionalBackend(b)
	result, err := cb.DeleteMulti(context.Background(), "bucket", "object.txt", "object.txt")
	require.NoError(t, err)
	assert.Len(t, result.Deleted, 2)
	assert.Empty(t, result.Error)
	assert.Empty(t, cb.locks.locks)
}

func TestKeyedRWMutexZeroValue(t *testing.T) {
	var locks keyedRWMutex
	unlock, err := locks.lock(context.Background(), "one", writeWeight)
	require.NoError(t, err)
	assert.Len(t, locks.locks, 1)
	unlock()
	assert.Empty(t, locks.locks)
	unlock, err = locks.lock(context.Background(), "one", 1)
	require.NoError(t, err)
	unlock()
	assert.Empty(t, locks.locks)
}
