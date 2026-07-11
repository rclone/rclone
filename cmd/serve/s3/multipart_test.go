// Multipart upload tests for serve s3.

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testBackingCounter hands out unique backing roots across test servers.
var testBackingCounter atomic.Int64

// newMultipartTestServer starts a serve s3 server backed by a fresh local temp
// directory and returns a low-level minio Core client (for explicit control of
// the multipart parts), the backing Fs and the bucket name. The server and
// client are torn down via t.Cleanup.
func newMultipartTestServer(t *testing.T, disableStreaming bool) (*minio.Core, fs.Fs, string) {
	return newMultipartTestServerBacking(t, "", disableStreaming)
}

// newMultipartTestServerBacking is like newMultipartTestServer but backed by
// the named remote (a fresh local temp directory if empty). ":memory:" gives
// an atomic (PartialUploads=false) backing, so the streamed-straight-to-the-
// destination path is exercised as well as the temporary-object path that
// local (PartialUploads=true) uses.
func newMultipartTestServerBacking(t *testing.T, backing string, disableStreaming bool) (*minio.Core, fs.Fs, string) {
	fstest.Initialise()
	ctx := context.Background()
	if backing == "" {
		backing = t.TempDir()
	}
	f, err := fs.NewFs(ctx, backing)
	require.NoError(t, err)
	// A unique bucket per server: every plain ":memory:" backing shares one
	// process-wide store, so a fixed name would leak objects between tests.
	bucket := fmt.Sprintf("test-%d", testBackingCounter.Add(1))
	require.NoError(t, f.Mkdir(ctx, bucket))
	// The VFS is cached per remote (fs.ConfigString), so a shared ":memory:"
	// server reuses a VFS whose cached root listing predates the bucket just
	// created; forget it so the new bucket is visible.
	if root, err := vfs.New(ctx, f, &vfscommon.Opt).Root(); err == nil {
		root.ForgetAll()
	}

	keyid := random.String(16)
	keysec := random.String(16)
	opt := Opt
	opt.DisableMultipartStreaming = disableStreaming
	opt.AuthKey = []string{fmt.Sprintf("%s,%s", keyid, keysec)}
	opt.HTTP.ListenAddr = []string{endpoint}
	w, err := newServer(ctx, f, &opt, &vfscommon.Opt, &proxy.Opt)
	require.NoError(t, err)
	go func() { _ = w.Serve() }()
	t.Cleanup(func() { _ = w.Shutdown() })

	u, err := url.Parse(w.server.URLs()[0])
	require.NoError(t, err)
	core, err := minio.NewCore(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(keyid, keysec, ""),
		Secure: false,
	})
	require.NoError(t, err)
	return core, f, bucket
}

// readObject reads bucket/object back from the backing Fs.
func readObject(t *testing.T, f fs.Fs, bucket, object string) []byte {
	ctx := context.Background()
	o, err := f.NewObject(ctx, path.Join(bucket, object))
	require.NoError(t, err)
	rc, err := o.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	return got
}

// multipartUploadParts uploads object to bucket as a multipart upload with the
// given (in-order) part sizes and returns the assembled contents plus the
// first error encountered.
func multipartUploadParts(t *testing.T, core *minio.Core, bucket, object string, partSizes []int) ([]byte, error) {
	ctx := context.Background()
	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	if err != nil {
		return nil, err
	}
	var want []byte
	var parts []minio.CompletePart
	for i, sz := range partSizes {
		data := []byte(random.String(sz))
		want = append(want, data...)
		p, err := core.PutObjectPart(ctx, bucket, object, uploadID, i+1, bytes.NewReader(data), int64(sz), minio.PutObjectPartOptions{})
		if err != nil {
			_ = core.AbortMultipartUpload(ctx, bucket, object, uploadID)
			return want, err
		}
		parts = append(parts, minio.CompletePart{PartNumber: i + 1, ETag: p.ETag})
	}
	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts, minio.PutObjectOptions{})
	return want, err
}

// TestMultipartNonUniform checks that a multipart upload whose parts are NOT a
// uniform size round-trips correctly, both with the default streaming path and
// with the in-memory fallback (--disable-multipart-streaming).
func TestMultipartNonUniform(t *testing.T) {
	// Non-uniform parts, last one smaller.
	partSizes := []int{120 * 1024, 100 * 1024, 53 * 1024}
	const object = "non-uniform.bin"

	for _, tc := range []struct {
		name             string
		disableStreaming bool
	}{
		{"Streaming", false},
		{"InMemory", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, f, bucket := newMultipartTestServer(t, tc.disableStreaming)
			want, err := multipartUploadParts(t, core, bucket, object, partSizes)
			require.NoError(t, err)
			assert.Equal(t, want, readObject(t, f, bucket, object))
		})
	}
}

// TestMultipartOutOfOrder uploads the parts concurrently and out of order,
// exercising the reorder buffer and the in-order pump handoff.
func TestMultipartOutOfOrder(t *testing.T) {
	core, f, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "out-of-order.bin"

	sizes := []int{70 * 1024, 90 * 1024, 50 * 1024, 33 * 1024}
	datas := make([][]byte, len(sizes))
	var want []byte
	for i, sz := range sizes {
		datas[i] = []byte(random.String(sz))
		want = append(want, datas[i]...)
	}

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	parts := make([]minio.CompletePart, len(sizes))
	errs := make([]error, len(sizes))
	var wg sync.WaitGroup
	for _, i := range []int{2, 0, 3, 1} { // shuffled upload order
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p, err := core.PutObjectPart(ctx, bucket, object, uploadID, i+1, bytes.NewReader(datas[i]), int64(sizes[i]), minio.PutObjectPartOptions{})
			errs[i] = err
			parts[i] = minio.CompletePart{PartNumber: i + 1, ETag: p.ETag}
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts, minio.PutObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
}

// TestMultipartNonContiguous checks that a multipart upload with a gap in the
// part numbers (which the in-order stream can't place) is rejected.
func TestMultipartNonContiguous(t *testing.T) {
	core, _, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "gap.bin"

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	var parts []minio.CompletePart
	for _, pn := range []int{1, 2, 4} { // part 3 missing
		data := []byte(random.String(40 * 1024))
		p, err := core.PutObjectPart(ctx, bucket, object, uploadID, pn, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
		require.NoError(t, err)
		parts = append(parts, minio.CompletePart{PartNumber: pn, ETag: p.ETag})
	}
	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, parts, minio.PutObjectOptions{})
	require.Error(t, err)
}

// requireOnly asserts that the bucket contains only the expected
// objects, in particular no leftover temporary multipart objects.
func requireOnly(t *testing.T, f fs.Fs, bucket string, want ...string) {
	entries, err := f.List(context.Background(), bucket)
	require.NoError(t, err)
	var got []string
	for _, entry := range entries {
		got = append(got, path.Base(entry.Remote()))
	}
	assert.ElementsMatch(t, want, got)
}

// testRemotes to exercise all the code branches
var testRemotes = []struct {
	name    string
	backing string
}{
	{"Local", ""},          // PartialUploads=true
	{"Memory", ":memory:"}, // PartialUploads=false
}

// TestMultipartAbort checks that aborting an upload tears down the streamed
// PutStream so neither the object nor its temporary object is left behind.
func TestMultipartAbort(t *testing.T) {
	for _, tc := range testRemotes {
		t.Run(tc.name, func(t *testing.T) {
			core, f, bucket := newMultipartTestServerBacking(t, tc.backing, false)
			ctx := context.Background()
			const object = "aborted.bin"

			uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
			require.NoError(t, err)
			data := []byte(random.String(50 * 1024))
			_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
			require.NoError(t, err)
			require.NoError(t, core.AbortMultipartUpload(ctx, bucket, object, uploadID))

			_, err = f.NewObject(ctx, path.Join(bucket, object))
			require.ErrorIs(t, err, fs.ErrorObjectNotFound)
			requireOnly(t, f, bucket)
		})
	}
}

// TestMultipartAbortPreservesExisting checks that aborting an upload to a name
// that already holds an object leaves the existing object untouched - the
// streamed upload must be atomic, not overwrite the destination as it goes.
func TestMultipartAbortPreservesExisting(t *testing.T) {
	for _, tc := range testRemotes {
		t.Run(tc.name, func(t *testing.T) {
			core, f, bucket := newMultipartTestServerBacking(t, tc.backing, false)
			ctx := context.Background()
			const object = "existing.bin"

			// Put an object the normal (non-multipart) way.
			existing := []byte(random.String(100))
			_, err := core.PutObject(ctx, bucket, object, bytes.NewReader(existing), int64(len(existing)), "", "", minio.PutObjectOptions{})
			require.NoError(t, err)

			// Start a multipart upload to the same name, upload a part, then abort.
			uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
			require.NoError(t, err)
			data := []byte(random.String(50 * 1024))
			_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
			require.NoError(t, err)
			require.NoError(t, core.AbortMultipartUpload(ctx, bucket, object, uploadID))

			// The original object must survive, and no temporary object be left behind.
			assert.Equal(t, existing, readObject(t, f, bucket, object))
			requireOnly(t, f, bucket, object)
		})
	}
}

// TestMultipartOverwrite checks that a completed multipart upload atomically
// replaces an existing object of the same name.
func TestMultipartOverwrite(t *testing.T) {
	for _, tc := range testRemotes {
		t.Run(tc.name, func(t *testing.T) {
			core, f, bucket := newMultipartTestServerBacking(t, tc.backing, false)
			ctx := context.Background()
			const object = "overwrite.bin"

			existing := []byte(random.String(100))
			_, err := core.PutObject(ctx, bucket, object, bytes.NewReader(existing), int64(len(existing)), "", "", minio.PutObjectOptions{})
			require.NoError(t, err)

			want, err := multipartUploadParts(t, core, bucket, object, []int{60 * 1024, 40 * 1024})
			require.NoError(t, err)

			assert.Equal(t, want, readObject(t, f, bucket, object))
			requireOnly(t, f, bucket, object)
		})
	}
}
