// Multipart upload tests for serve s3.

package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rclone/gofakes3"
	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pool"
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
// an atomic (PartialUploads=false) backing, so both flavours of remote go
// through the temporary-object-plus-rename path.
func newMultipartTestServerBacking(t *testing.T, backing string, disableStreaming bool) (*minio.Core, fs.Fs, string) {
	return newMultipartTestServerOpt(t, backing, disableStreaming, nil)
}

// newMultipartTestServerOpt is like newMultipartTestServerBacking but also
// applies tweak (if non-nil) to the server Options before starting it.
func newMultipartTestServerOpt(t *testing.T, backing string, disableStreaming bool, tweak func(*Options)) (*minio.Core, fs.Fs, string) {
	return newMultipartTestServerVFS(t, backing, disableStreaming, tweak, nil)
}

// newMultipartTestServerVFS is like newMultipartTestServerOpt but also
// overrides the VFS options (nil for the defaults) and disables the named
// features on the backing remote. A backing with features disabled must have
// a unique config string (e.g. a distinct description=) so an active VFS
// wrapping a fully-featured instance of the same remote isn't reused.
func newMultipartTestServerVFS(t *testing.T, backing string, disableStreaming bool, tweak func(*Options), vfsOpt *vfscommon.Options, disableFeatures ...string) (*minio.Core, fs.Fs, string) {
	fstest.Initialise()
	ctx := context.Background()
	if len(disableFeatures) > 0 {
		var ci *fs.ConfigInfo
		ctx, ci = fs.AddConfig(ctx)
		ci.DisableFeatures = disableFeatures
	}
	if backing == "" {
		backing = t.TempDir()
	}
	f, err := fs.NewFs(ctx, backing)
	require.NoError(t, err)
	// A unique bucket per server: every plain ":memory:" backing shares one
	// process-wide store, so a fixed name would leak objects between tests.
	bucket := fmt.Sprintf("test-%d", testBackingCounter.Add(1))
	require.NoError(t, f.Mkdir(ctx, bucket))
	if vfsOpt == nil {
		vfsOpt = &vfscommon.Opt
	}
	// The VFS is cached per remote (fs.ConfigString), so a shared ":memory:"
	// server reuses a VFS whose cached root listing predates the bucket just
	// created; forget it so the new bucket is visible.
	if root, err := vfs.New(ctx, f, vfsOpt).Root(); err == nil {
		root.ForgetAll()
	}

	keyid := random.String(16)
	keysec := random.String(16)
	opt := Opt
	opt.DisableMultipartStreaming = disableStreaming
	opt.AuthKey = []string{fmt.Sprintf("%s,%s", keyid, keysec)}
	opt.HTTP.ListenAddr = []string{endpoint}
	if tweak != nil {
		tweak(&opt)
	}
	w, err := newServer(ctx, f, &opt, vfsOpt, &proxy.Opt)
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

// TestMultipartRetryAfterStreamed checks that a part which is uploaded again
// after its first copy has already been streamed to the backend - as a client
// whose HTTP request timed out will do when it retries - is accepted
// idempotently and doesn't fail the CompleteMultipartUpload.
func TestMultipartRetryAfterStreamed(t *testing.T) {
	core, f, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "retry.bin"

	part1 := []byte(random.String(60 * 1024))
	part2 := []byte(random.String(40 * 1024))
	want := append(append([]byte(nil), part1...), part2...)

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	// Parts uploaded in order are streamed synchronously, so by the time
	// PutObjectPart returns the part is already in the backend stream.
	p1, err := core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(part1), int64(len(part1)), minio.PutObjectPartOptions{})
	require.NoError(t, err)
	p2, err := core.PutObjectPart(ctx, bucket, object, uploadID, 2, bytes.NewReader(part2), int64(len(part2)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	// Retry part 1 with identical content, as a client retrying after a
	// timeout does.
	p1retry, err := core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(part1), int64(len(part1)), minio.PutObjectPartOptions{})
	require.NoError(t, err)
	assert.Equal(t, p1.ETag, p1retry.ETag)

	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, []minio.CompletePart{
		{PartNumber: 1, ETag: p1.ETag},
		{PartNumber: 2, ETag: p2.ETag},
	}, minio.PutObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
}

// TestMultipartRetryDifferentContent checks that re-uploading a part with
// different content after the first copy has been streamed is rejected - the
// in-order stream can't replace data already sent to the backend.
func TestMultipartRetryDifferentContent(t *testing.T) {
	core, _, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "retry-different.bin"

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	part1 := []byte(random.String(60 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(part1), int64(len(part1)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	other := []byte(random.String(60 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(other), int64(len(other)), minio.PutObjectPartOptions{})
	require.Error(t, err)

	require.NoError(t, core.AbortMultipartUpload(ctx, bucket, object, uploadID))
}

// TestMultipartReplaceBuffered checks that re-uploading a part which has been
// received but not yet streamed (it is waiting for an earlier part) replaces
// the buffered copy, matching S3's last-write-wins semantics.
func TestMultipartReplaceBuffered(t *testing.T) {
	core, f, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "replace-buffered.bin"

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	// Part 2 arrives first so it is buffered awaiting part 1.
	old := []byte(random.String(40 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 2, bytes.NewReader(old), int64(len(old)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	// Upload part 2 again with different content - the buffered copy must be
	// replaced.
	part2 := []byte(random.String(40 * 1024))
	p2, err := core.PutObjectPart(ctx, bucket, object, uploadID, 2, bytes.NewReader(part2), int64(len(part2)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	part1 := []byte(random.String(60 * 1024))
	p1, err := core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(part1), int64(len(part1)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, []minio.CompletePart{
		{PartNumber: 1, ETag: p1.ETag},
		{PartNumber: 2, ETag: p2.ETag},
	}, minio.PutObjectOptions{})
	require.NoError(t, err)
	want := append(append([]byte(nil), part1...), part2...)
	assert.Equal(t, want, readObject(t, f, bucket, object))
}

// TestMultipartBufferLimit checks that a part arriving ahead of its turn
// blocks once the reorder buffer limit is reached, and is admitted once the
// missing part arrives and the buffer drains.
func TestMultipartBufferLimit(t *testing.T) {
	core, f, bucket := newMultipartTestServerOpt(t, "", false, func(opt *Options) {
		opt.MultipartStreamingBufferLimit = pool.BufferSize
	})
	ctx := context.Background()
	const object = "buffer-limit.bin"

	sizes := []int{40 * 1024, 40 * 1024, 40 * 1024}
	datas := make([][]byte, len(sizes))
	var want []byte
	for i, sz := range sizes {
		datas[i] = []byte(random.String(sz))
		want = append(want, datas[i]...)
	}

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	// Part 3 arrives first: it fits in the buffer so it is admitted even
	// though it must wait for its turn.
	p3, err := core.PutObjectPart(ctx, bucket, object, uploadID, 3, bytes.NewReader(datas[2]), int64(sizes[2]), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	// Part 2 would take the buffer over the limit, so it must block until
	// part 1 arrives and the stream drains.
	type partResult struct {
		part minio.ObjectPart
		err  error
	}
	done := make(chan partResult, 1)
	go func() {
		p, err := core.PutObjectPart(ctx, bucket, object, uploadID, 2, bytes.NewReader(datas[1]), int64(sizes[1]), minio.PutObjectPartOptions{})
		done <- partResult{p, err}
	}()
	select {
	case r := <-done:
		t.Fatalf("part 2 was not blocked by the buffer limit (err=%v)", r.err)
	case <-time.After(200 * time.Millisecond):
	}

	// Part 1 is the part the stream needs so it is admitted regardless of the
	// limit; streaming it frees the buffer and unblocks part 2.
	p1, err := core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(datas[0]), int64(sizes[0]), minio.PutObjectPartOptions{})
	require.NoError(t, err)
	var p2 minio.ObjectPart
	select {
	case r := <-done:
		require.NoError(t, r.err)
		p2 = r.part
	case <-time.After(10 * time.Second):
		t.Fatal("part 2 was never unblocked")
	}

	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, []minio.CompletePart{
		{PartNumber: 1, ETag: p1.ETag},
		{PartNumber: 2, ETag: p2.ETag},
		{PartNumber: 3, ETag: p3.ETag},
	}, minio.PutObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
}

// storeTestUpload records up as an in-flight upload of b, as
// CreateMultipartUpload does.
func storeTestUpload(t *testing.T, b *s3Backend, uploadID gofakes3.UploadID, up *multipartUpload) {
	tenant := b.tenant(context.Background())
	require.NoError(t, b.reserveUpload(tenant))
	require.NoError(t, b.addUpload(tenant, uploadID, up))
}

// stubSink is a multipartUpload sink which records how it was closed.
type stubSink struct {
	closed   bool
	abortErr error // the reason passed to CloseWithError
}

func (s *stubSink) Write(p []byte) (int, error) { return len(p), nil }

func (s *stubSink) Close() error {
	s.closed = true
	return nil
}

func (s *stubSink) CloseWithError(err error) error {
	s.closed = true
	s.abortErr = err
	return nil
}

// TestMultipartAbortDuringUploadPart aborts the upload between a buffered
// part's waitForTurn and its bufferPart - as happens when the abort arrives
// while the part body is still being received from the client - and checks
// that bufferPart fails cleanly instead of panicking on the torn-down upload.
func TestMultipartAbortDuringUploadPart(t *testing.T) {
	up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 0)
	sink := &stubSink{}
	up.fh = sink

	// An UploadPart in progress: the part is admitted, then the abort lands
	// while its body is still being received.
	contents := []byte("hello")
	turn, err := up.waitForTurn(context.Background(), 2, int64(len(contents)))
	require.NoError(t, err)
	require.Equal(t, turnBuffer, turn)
	require.NoError(t, up.abort())

	// The abort must abandon the write rather than committing it.
	assert.True(t, sink.closed)
	assert.Equal(t, errMultipartAborted, sink.abortErr)

	// The UploadPart resumes: it buffers the part and calls bufferPart.
	rw := multipart.NewRW()
	_, err = rw.Write(contents)
	require.NoError(t, err)
	md5Sum := md5.Sum(contents)
	err = up.bufferPart(context.Background(), 2, int64(len(contents)), md5Sum[:], rw)
	require.ErrorIs(t, err, gofakes3.ErrNoSuchUpload)

	// The reorder buffer reservation must have been returned
	up.mu.Lock()
	assert.Equal(t, int64(0), up.buffered)
	up.mu.Unlock()
}

// TestMultipartCloseAfterAbort checks that a close racing an abort reports
// the abort: gofakes3 can dispatch CompleteMultipartUpload and
// AbortMultipartUpload for the same uploadID concurrently, and a Complete
// that loses the race must not report success for data that was never
// committed.
func TestMultipartCloseAfterAbort(t *testing.T) {
	up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 0)
	up.fh = &stubSink{}
	require.NoError(t, up.abort())
	require.ErrorIs(t, up.close(), gofakes3.ErrNoSuchUpload)

	// close stays idempotent after a successful close.
	up = newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 0)
	up.fh = &stubSink{}
	require.NoError(t, up.close())
	require.NoError(t, up.close())
}

// TestMultipartCloseIncomplete checks that close refuses to commit while a
// part is still buffered awaiting an earlier one, leaving the upload open
// for more parts or an abort.
func TestMultipartCloseIncomplete(t *testing.T) {
	up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 0)
	sink := &stubSink{}
	up.fh = sink

	// Part 2 arrives ahead of part 1 so it is buffered, not streamed.
	contents := []byte("hello")
	rw := multipart.NewRW()
	_, err := rw.Write(contents)
	require.NoError(t, err)
	md5Sum := md5.Sum(contents)
	turn, err := up.waitForTurn(context.Background(), 2, int64(len(contents)))
	require.NoError(t, err)
	require.Equal(t, turnBuffer, turn)
	require.NoError(t, up.bufferPart(context.Background(), 2, int64(len(contents)), md5Sum[:], rw))

	require.ErrorIs(t, up.close(), gofakes3.ErrInvalidPart)
	assert.False(t, sink.closed)

	// The upload is still open so abort tears it down.
	require.NoError(t, up.abort())
	assert.True(t, sink.closed)
}

// failingSink is a sink whose Close fails and which cannot abandon writes,
// like a caching VFS handle whose synchronous write-back fails.
type failingSink struct{}

func (failingSink) Write(p []byte) (int, error) { return len(p), nil }
func (failingSink) Close() error                { return errBoom }

// TestMultipartAbortAlwaysSucceeds checks that AbortMultipartUpload reports
// success even when closing the sink fails: the upload is torn down either
// way, and an error reply would leave gofakes3's record of the upload alive
// with ours consumed, so every retried abort would 404 on a ghost upload.
func TestMultipartAbortAlwaysSucceeds(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	_vfs, err := b.s.getVFS(ctx)
	require.NoError(t, err)

	up := newMultipartUpload(bucket, "key", bucket+"/key", bucket+"/"+multipartUploadPrefix+"x", nil, 0)
	up.fh = failingSink{}
	up.vfs = _vfs
	const uploadID = gofakes3.UploadID("failing-close")
	storeTestUpload(t, b, uploadID, up)

	require.NoError(t, b.AbortMultipartUpload(ctx, bucket, "key", uploadID))

	// The upload record is consumed - a second abort is NoSuchUpload.
	require.ErrorIs(t, b.AbortMultipartUpload(ctx, bucket, "key", uploadID), gofakes3.ErrNoSuchUpload)
}

// TestMultipartCompleteRenameFailureKeepsUpload checks that a Complete
// failing after the commit (here: the rename of the temporary object) keeps
// the upload record, so the retried CompleteMultipartUpload which gofakes3
// allows on a backend error finds the upload instead of a NoSuchUpload.
func TestMultipartCompleteRenameFailureKeepsUpload(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	_vfs, err := b.s.getVFS(ctx)
	require.NoError(t, err)

	// A committed upload whose temporary object is missing: the rename
	// fails after the commit succeeded.
	up := newMultipartUpload(bucket, "key", bucket+"/key", bucket+"/"+multipartUploadPrefix+"missing", nil, 0)
	up.fh = &stubSink{}
	up.vfs = _vfs
	const uploadID = gofakes3.UploadID("rename-fails")
	storeTestUpload(t, b, uploadID, up)

	_, _, err = b.CompleteMultipartUpload(ctx, bucket, "key", uploadID, &gofakes3.CompleteMultipartUploadRequest{})
	require.Error(t, err)
	_, err = b.loadUpload(context.Background(), uploadID)
	require.NoError(t, err, "the upload record must survive a retryable Complete failure")
}

// TestMultipartOtherTenant checks that with an auth proxy one user
// (access key ID) can't upload parts to, complete or abort another
// user's multipart upload even if it knows the upload ID.
func TestMultipartOtherTenant(t *testing.T) {
	b, f, bucket := newPutTestBackend(t, "", nil)
	ctxA, ctxB := tenantCtx("tenantA"), tenantCtx("tenantB")
	const object = "victim.bin"

	uploadID, err := b.CreateMultipartUpload(ctxA, bucket, object, nil)
	require.NoError(t, err)

	data := []byte(random.String(1024))
	_, err = b.UploadPart(ctxB, bucket, object, uploadID, 1, int64(len(data)), bytes.NewReader(data))
	assert.ErrorIs(t, err, gofakes3.ErrNoSuchUpload, "UploadPart by another user")
	require.ErrorIs(t, b.AbortMultipartUpload(ctxB, bucket, object, uploadID), gofakes3.ErrNoSuchUpload, "Abort by another user")

	etag, err := b.UploadPart(ctxA, bucket, object, uploadID, 1, int64(len(data)), bytes.NewReader(data))
	require.NoError(t, err)
	input := &gofakes3.CompleteMultipartUploadRequest{Parts: []gofakes3.CompletedPart{{PartNumber: 1, ETag: etag}}}
	_, _, err = b.CompleteMultipartUpload(ctxB, bucket, object, uploadID, input)
	assert.ErrorIs(t, err, gofakes3.ErrNoSuchUpload, "Complete by another user")

	// The owner's upload is untouched by the other user's attempts.
	_, _, err = b.CompleteMultipartUpload(ctxA, bucket, object, uploadID, input)
	require.NoError(t, err)
	assert.Equal(t, data, readObject(t, f, bucket, object))
}

// TestMultipartVFSShutDownBetweenParts checks that a multipart upload
// survives the auth proxy shutting down the VFS it was started with, as
// the proxy does once the VFS has been unused in its cache for a while.
func TestMultipartVFSShutDownBetweenParts(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)
	const bucket, object = "bucket", "object"
	require.NoError(t, f.Mkdir(ctx, bucket))
	opt := Opt
	opt.HTTP.ListenAddr = []string{endpoint}
	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "/path/to/auth/proxy"
	w, err := newServer(ctx, f, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Shutdown() })
	b := w.backend

	// requestCtx returns the context of a request with a VFS from the
	// proxy's cache, which the proxy shuts down when it expires.
	requestCtx := func() (context.Context, *vfs.VFS) {
		VFS := vfs.New(ctx, f, &vfscommon.Opt)
		return context.WithValue(tenantCtx("alice"), ctxKeyID, VFS), VFS
	}

	ctx1, vfs1 := requestCtx()
	uploadID, err := b.CreateMultipartUpload(ctx1, bucket, object, nil)
	require.NoError(t, err)
	vfs1.Shutdown()

	ctx2, vfs2 := requestCtx()
	defer vfs2.Shutdown()
	data := []byte(random.String(1024))
	etag, err := b.UploadPart(ctx2, bucket, object, uploadID, 1, int64(len(data)), bytes.NewReader(data))
	require.NoError(t, err)
	input := &gofakes3.CompleteMultipartUploadRequest{Parts: []gofakes3.CompletedPart{{PartNumber: 1, ETag: etag}}}
	_, _, err = b.CompleteMultipartUpload(ctx2, bucket, object, uploadID, input)
	require.NoError(t, err)
	assert.Equal(t, data, readObject(t, f, bucket, object))
}

// TestMultipartReaper checks that an incomplete multipart upload abandoned
// by its client is aborted and cleaned up after --multipart-expiry, and that
// late operations on it fail with NoSuchUpload.
func TestMultipartReaper(t *testing.T) {
	core, f, bucket := newMultipartTestServerOpt(t, "", false, func(opt *Options) {
		opt.MultipartExpiry = fs.Duration(100 * time.Millisecond)
	})
	ctx := context.Background()
	const object = "abandoned.bin"

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)
	data := []byte(random.String(50 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	// FIXME gofakes3 lists nothing when the delimiter is empty
	uploads, err := core.ListMultipartUploads(ctx, bucket, "", "", "", "/", 1000)
	require.NoError(t, err)
	require.Len(t, uploads.Uploads, 1)

	// Wait for well over the expiry and the reaper interval, then the
	// upload must be gone, from listings too.
	time.Sleep(time.Second)
	uploads, err = core.ListMultipartUploads(ctx, bucket, "", "", "", "/", 1000)
	require.NoError(t, err)
	assert.Empty(t, uploads.Uploads)
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 2, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchUpload")
	err = core.AbortMultipartUpload(ctx, bucket, object, uploadID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchUpload")

	// Nothing is left at the key or as a temporary object.
	_, err = f.NewObject(ctx, path.Join(bucket, object))
	require.ErrorIs(t, err, fs.ErrorObjectNotFound)
	requireOnly(t, f, bucket)
}

// TestMultipartCompleteFailureForgotten checks that a completion which
// tears the upload down removes it from gofakes3's records too, so it
// isn't left in listings or memory.
func TestMultipartCompleteFailureForgotten(t *testing.T) {
	core, f, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "bad-complete.bin"

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)
	data := []byte(random.String(50 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	require.NoError(t, err)

	_, err = core.CompleteMultipartUpload(ctx, bucket, object, uploadID, []minio.CompletePart{{PartNumber: 1, ETag: `"00000000000000000000000000000000"`}}, minio.PutObjectOptions{})
	require.Error(t, err)

	// FIXME gofakes3 lists nothing when the delimiter is empty
	uploads, err := core.ListMultipartUploads(ctx, bucket, "", "", "", "/", 1000)
	require.NoError(t, err)
	assert.Empty(t, uploads.Uploads)
	requireOnly(t, f, bucket)
}

// TestMultipartReapExpiredUploads checks the reaper's rules directly: an
// idle upload past the expiry is aborted and cleaned up, one with a request
// in flight is left alone however stale its idle time, and a fresh one is
// kept.
func TestMultipartReapExpiredUploads(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	_vfs, err := b.s.getVFS(ctx)
	require.NoError(t, err)

	newUp := func(id string) *multipartUpload {
		up := newMultipartUpload(bucket, id, bucket+"/"+id, bucket+"/"+multipartUploadPrefix+id, nil, 0)
		up.fh = &stubSink{}
		up.vfs = _vfs
		storeTestUpload(t, b, gofakes3.UploadID(id), up)
		return up
	}

	const expiry = time.Hour
	now := time.Now()

	newUp("fresh")
	idle := newUp("idle")
	busy := newUp("busy")
	require.NoError(t, busy.startActivity())
	t.Cleanup(busy.endActivity)
	for _, up := range []*multipartUpload{idle, busy} {
		up.mu.Lock()
		up.lastUsed = now.Add(-2 * expiry)
		up.mu.Unlock()
	}

	b.reapExpiredUploads(now, expiry)

	_, err = b.loadUpload(context.Background(), "fresh")
	assert.NoError(t, err, "a fresh upload must not be reaped")
	_, err = b.loadUpload(context.Background(), "busy")
	assert.NoError(t, err, "an upload with a request in flight must not be reaped")
	_, err = b.loadUpload(context.Background(), "idle")
	assert.ErrorIs(t, err, gofakes3.ErrNoSuchUpload, "an idle upload past the expiry must be reaped")

	// The reaped upload was aborted, not committed.
	sink := idle.fh.(*stubSink)
	assert.True(t, sink.closed)
	assert.Equal(t, errMultipartAborted, sink.abortErr)
}

// cacheWritesVFSOpt returns VFS options with --vfs-cache-mode writes and the
// given write-back delay.
func cacheWritesVFSOpt(writeBack time.Duration) *vfscommon.Options {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeWrites
	vfsOpt.WriteBack = fs.Duration(writeBack)
	return &vfsOpt
}

// waitForContent waits for bucket/object on the backing Fs to hold want
// (e.g. after the VFS write-back delay).
func waitForContent(t *testing.T, f fs.Fs, bucket, object string, want []byte) {
	ctx := context.Background()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if o, err := f.NewObject(ctx, path.Join(bucket, object)); err == nil {
			if rc, err := o.Open(ctx); err == nil {
				got, err := io.ReadAll(rc)
				_ = rc.Close()
				if err == nil && bytes.Equal(got, want) {
					return
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("object %s/%s never reached the expected content on the backing remote", bucket, object)
}

// TestMultipartCacheModeWrites checks that with --vfs-cache-mode writes a
// multipart upload goes through the VFS cache and is written back to the
// backing remote, with no temporary object left behind. Also run with
// --disable-multipart-streaming, which only affects the streaming path - the
// cache needs no PutStream, so backends without one take this path instead
// of buffering in memory.
func TestMultipartCacheModeWrites(t *testing.T) {
	for _, tc := range []struct {
		name             string
		disableStreaming bool
	}{
		{"Streaming", false},
		{"NoStreaming", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, f, bucket := newMultipartTestServerVFS(t, "", tc.disableStreaming, nil, cacheWritesVFSOpt(100*time.Millisecond))
			const object = "cached.bin"
			want, err := multipartUploadParts(t, core, bucket, object, []int{120 * 1024, 100 * 1024, 53 * 1024})
			require.NoError(t, err)
			waitForContent(t, f, bucket, object, want)
			requireOnly(t, f, bucket, object)
		})
	}
}

// TestMultipartCacheModeMinimal checks that --vfs-cache-mode minimal takes
// the cached path just like writes - the parts are written through a
// read-write handle, which the VFS caches from minimal up - including with
// --disable-multipart-streaming set, which only affects the streaming path.
func TestMultipartCacheModeMinimal(t *testing.T) {
	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeMinimal
	vfsOpt.WriteBack = fs.Duration(100 * time.Millisecond)
	core, f, bucket := newMultipartTestServerVFS(t, "", true, nil, &vfsOpt)
	const object = "cached-minimal.bin"

	want, err := multipartUploadParts(t, core, bucket, object, []int{120 * 1024, 100 * 1024, 53 * 1024})
	require.NoError(t, err)
	waitForContent(t, f, bucket, object, want)
	requireOnly(t, f, bucket, object)
}

// TestMultipartCacheModeWritesAbort checks that with --vfs-cache-mode writes
// an aborted upload is discarded from the cache: nothing reaches the backing
// remote and an existing object at the key survives.
func TestMultipartCacheModeWritesAbort(t *testing.T) {
	core, f, bucket := newMultipartTestServerVFS(t, "", false, nil, cacheWritesVFSOpt(100*time.Millisecond))
	ctx := context.Background()
	const object = "cached-abort.bin"

	existing := []byte(random.String(100))
	_, err := core.PutObject(ctx, bucket, object, bytes.NewReader(existing), int64(len(existing)), "", "", minio.PutObjectOptions{})
	require.NoError(t, err)
	waitForContent(t, f, bucket, object, existing)

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)
	data := []byte(random.String(50 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	require.NoError(t, err)
	require.NoError(t, core.AbortMultipartUpload(ctx, bucket, object, uploadID))

	// Wait out several write-back intervals: the aborted upload must not be
	// written back, neither over the object nor as a temporary object.
	time.Sleep(time.Second)
	assert.Equal(t, existing, readObject(t, f, bucket, object))
	requireOnly(t, f, bucket, object)
}

// TestMultipartCacheModeWritesSupersedesPut checks that a multipart upload
// completed while an earlier PUT to the same key is still in the write-back
// window ends up with the multipart data: both writes go through the same
// cache item, so the earlier PUT's write-back cannot land on top of the
// newer multipart object.
func TestMultipartCacheModeWritesSupersedesPut(t *testing.T) {
	core, f, bucket := newMultipartTestServerVFS(t, "", false, nil, cacheWritesVFSOpt(500*time.Millisecond))
	ctx := context.Background()
	const object = "supersede.bin"

	// PUT an object; it sits in the cache awaiting write-back.
	old := []byte(random.String(100))
	_, err := core.PutObject(ctx, bucket, object, bytes.NewReader(old), int64(len(old)), "", "", minio.PutObjectOptions{})
	require.NoError(t, err)

	// Immediately replace it with a multipart upload to the same key.
	want, err := multipartUploadParts(t, core, bucket, object, []int{60 * 1024, 40 * 1024})
	require.NoError(t, err)

	// After all write-backs settle the multipart data must have won.
	waitForContent(t, f, bucket, object, want)
	time.Sleep(time.Second)
	assert.Equal(t, want, readObject(t, f, bucket, object))
	requireOnly(t, f, bucket, object)
}

// TestMultipartNoPutStream checks that a multipart upload to a remote
// without streaming upload support works with the default cache mode: the
// parts are spooled to a temporary file on local disk and uploaded with a
// known size, rather than being buffered in memory.
func TestMultipartNoPutStream(t *testing.T) {
	core, f, bucket := newMultipartTestServerVFS(t, "", false, nil, nil, "PutStream")
	require.Nil(t, f.Features().PutStream)
	const object = "no-putstream.bin"

	want, err := multipartUploadParts(t, core, bucket, object, []int{120 * 1024, 100 * 1024, 53 * 1024})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
	requireOnly(t, f, bucket, object)
}

// TestMultipartNoServerSideMove checks multipart uploads to an atomic remote
// with no server-side move or copy, where the parts stream straight to the
// final object: an upload round-trips, and an aborted upload leaves an
// existing object at the key untouched.
func TestMultipartNoServerSideMove(t *testing.T) {
	// The distinct description= gives this remote its own config string so
	// it doesn't share a VFS with the fully-featured ":memory:" servers.
	core, f, bucket := newMultipartTestServerVFS(t, ":memory,description=no-server-side-move:", false, nil, nil, "Copy")
	require.False(t, operations.CanServerSideMove(f))
	ctx := context.Background()
	const object = "direct.bin"

	existing := []byte(random.String(100))
	_, err := core.PutObject(ctx, bucket, object, bytes.NewReader(existing), int64(len(existing)), "", "", minio.PutObjectOptions{})
	require.NoError(t, err)

	// An aborted upload must leave the existing object untouched.
	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)
	data := []byte(random.String(50 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	require.NoError(t, err)
	require.NoError(t, core.AbortMultipartUpload(ctx, bucket, object, uploadID))
	assert.Equal(t, existing, readObject(t, f, bucket, object))
	requireOnly(t, f, bucket, object)

	// A completed upload replaces it.
	want, err := multipartUploadParts(t, core, bucket, object, []int{60 * 1024, 40 * 1024})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
	requireOnly(t, f, bucket, object)
}

// TestMultipartNoServerSideMovePartialUploads checks that a multipart upload
// to a remote where partial uploads are visible and which has no server-side
// move or copy round-trips: the parts are written straight to the final
// object rather than being buffered in memory.
func TestMultipartNoServerSideMovePartialUploads(t *testing.T) {
	core, f, bucket := newMultipartTestServerVFS(t, "", false, nil, nil, "Move", "Copy")
	require.False(t, operations.CanServerSideMove(f))
	require.True(t, f.Features().PartialUploads)
	const object = "direct-partial.bin"

	want, err := multipartUploadParts(t, core, bucket, object, []int{60 * 1024, 40 * 1024})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
	requireOnly(t, f, bucket, object)
}

// TestMultipartCacheModeWritesNoServerSideMove checks that with
// --vfs-cache-mode writes a remote with no server-side move or copy is still
// written through the cache rather than falling back to buffering the upload
// in memory, and pins the documented trade-offs of that path: the parts go
// into the cache under the final key, so the in-flight upload is visible
// there, and an aborted upload cannot be abandoned once in the cache, so its
// partial data is written back as if it were a completed object.
func TestMultipartCacheModeWritesNoServerSideMove(t *testing.T) {
	// The distinct description= gives this remote its own config string so
	// it doesn't share a VFS with the fully-featured ":memory:" servers.
	core, f, bucket := newMultipartTestServerVFS(t, ":memory,description=no-move-cache:", false, nil, cacheWritesVFSOpt(100*time.Millisecond), "Copy")
	require.False(t, operations.CanServerSideMove(f))
	ctx := context.Background()

	// A round trip goes through the cache under the final key.
	const object = "cached-direct.bin"
	want, err := multipartUploadParts(t, core, bucket, object, []int{120 * 1024, 100 * 1024, 53 * 1024})
	require.NoError(t, err)
	waitForContent(t, f, bucket, object, want)

	// The parts are written to the cache, not buffered in memory, so the
	// in-flight upload is visible at the key.
	const object2 = "cached-direct-inflight.bin"
	uploadID, err := core.NewMultipartUpload(ctx, bucket, object2, minio.PutObjectOptions{})
	require.NoError(t, err)
	data := []byte(random.String(50 * 1024))
	_, err = core.PutObjectPart(ctx, bucket, object2, uploadID, 1, bytes.NewReader(data), int64(len(data)), minio.PutObjectPartOptions{})
	require.NoError(t, err)
	_, err = core.StatObject(ctx, bucket, object2, minio.StatObjectOptions{})
	assert.NoError(t, err, "in-flight upload should be visible at the key")

	// An aborted upload's partial data is committed to the cache and
	// written back.
	require.NoError(t, core.AbortMultipartUpload(ctx, bucket, object2, uploadID))
	waitForContent(t, f, bucket, object2, data)
	requireOnly(t, f, bucket, object, object2)
}

// TestTempObjectsHiddenFromListings checks that the reserved .rclone_temp_
// prefix, and the multipart prefix used before it was reserved, are hidden
// from S3 listings while remaining visible to rclone itself for cleanup.
func TestTempObjectsHiddenFromListings(t *testing.T) {
	core, f, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()

	names := []string{
		"visible.bin",
		tempObjectPrefix + "anything",
		multipartUploadPrefix + "leftover",
		putObjectPrefix + "leftover",
		legacyMultipartUploadPrefix + "leftover",
	}
	for _, name := range names {
		data := []byte("x")
		src := object.NewStaticObjectInfo(path.Join(bucket, name), time.Now(), int64(len(data)), true, nil, f)
		_, err := f.Put(ctx, bytes.NewReader(data), src)
		require.NoError(t, err)
	}
	// All the objects really exist on the backing remote...
	requireOnly(t, f, bucket, names...)

	// ...but only the visible one appears in an S3 listing.
	result, err := core.ListObjects(bucket, "", "", "", 1000)
	require.NoError(t, err)
	var keys []string
	for _, o := range result.Contents {
		keys = append(keys, o.Key)
	}
	assert.Equal(t, []string{"visible.bin"}, keys)
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

// poolProbeReader records the pool's in-use buffer count the first time it is
// read - after UploadPart has created its buffer but before any body bytes have
// been delivered - then reports a short body by returning io.EOF.
type poolProbeReader struct {
	baseline int
	recorded int
	read     bool
}

func (r *poolProbeReader) Read(p []byte) (int, error) {
	if !r.read {
		r.read = true
		r.recorded = pool.Global().InUse() - r.baseline
	}
	return 0, io.EOF
}

// TestUploadPartNoReserveBeforeBody checks that UploadPart does not preallocate
// pool memory proportional to the client-declared Content-Length before any
// body bytes have been received. A part declaring a large size but sending no
// body must not reserve pages up front, so an unverified header cannot exhaust
// process memory.
func TestUploadPartNoReserveBeforeBody(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, "object", nil)
	require.NoError(t, err)

	const declared = int64(64 << 20) // 64 MiB declared by the client
	wantPages := int(declared / int64(pool.BufferSize))

	reader := &poolProbeReader{baseline: pool.Global().InUse()}
	_, err = b.UploadPart(ctx, bucket, "object", uploadID, 1, declared, reader)
	// No body bytes arrive, so the part is rejected as incomplete.
	require.ErrorIs(t, err, gofakes3.ErrIncompleteBody)

	// The fixed path allocates nothing before the first read, so recorded is 0;
	// the vulnerable path preallocated wantPages (64). The pool is process-wide,
	// so recorded could pick up a few unrelated in-use buffers, but never the
	// 64-page reservation the bug produced - the margin distinguishes them.
	require.True(t, reader.read, "the body must have been read")
	require.Less(t, reader.recorded, wantPages,
		"UploadPart preallocated pool pages from the declared Content-Length before any body bytes arrived")
}

// TestWaitForTurnRejectsBogusSize checks that the reorder-buffer admission
// rejects a negative client-declared part length, and that a huge declared
// length is never admitted to the buffer - it must wait to be streamed - nor
// can it overflow the running total.
func TestWaitForTurnRejectsBogusSize(t *testing.T) {
	up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 1<<20)
	ctx := context.Background()

	// A part length can never be negative.
	_, err := up.waitForTurn(ctx, 1, -1)
	require.ErrorIs(t, err, gofakes3.ErrInvalidArgument)

	// The next part is streamed, whatever its size, without touching the
	// buffer.
	turn, err := up.waitForTurn(ctx, 1, math.MaxInt64)
	require.NoError(t, err)
	assert.Equal(t, turnStream, turn)
	up.mu.Lock()
	assert.Equal(t, int64(0), up.buffered)
	up.mu.Unlock()

	// A huge out-of-order part must wait for its turn, not be admitted.
	admitted := make(chan struct{})
	go func() {
		_, _ = up.waitForTurn(ctx, 2, math.MaxInt64)
		close(admitted)
	}()
	select {
	case <-admitted:
		t.Fatal("huge out-of-order part admitted past the buffer limit")
	case <-time.After(50 * time.Millisecond):
	}

	// Wake the blocked goroutine so it doesn't leak.
	up.mu.Lock()
	up.closed = true
	up.broadcast()
	up.mu.Unlock()
	<-admitted
}

// TestWaitForTurnRejectsOverflowingReservation checks that huge declared
// lengths sent by concurrent requests for the next part can't overflow the
// running totals negative and so admit out-of-order parts past the buffer
// limits, whether the upload's own or the server's.
func TestWaitForTurnRejectsOverflowingReservation(t *testing.T) {
	const huge = 5_000_000_000_000_000_000
	for _, tc := range []struct {
		name        string
		bufferLimit int64
		budget      *bufferBudget
	}{
		{"UploadLimit", 1 << 20, nil},
		{"ServerLimit", 0, newBufferBudget(1 << 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, tc.bufferLimit)
			up.budget = tc.budget
			ctx := context.Background()

			// The first request for the next part streams it.
			turn, err := up.waitForTurn(ctx, 1, huge)
			require.NoError(t, err)
			assert.Equal(t, turnStream, turn)

			// Concurrent requests for the same part and one out of order
			// must all wait rather than be charged to the buffer.
			admitted := make(chan struct{}, 3)
			for _, part := range []struct {
				number int
				size   int64
			}{{1, huge}, {1, huge}, {3, 8 << 20}} {
				go func() {
					_, _ = up.waitForTurn(ctx, part.number, part.size)
					admitted <- struct{}{}
				}()
			}
			select {
			case <-admitted:
				t.Fatal("part admitted past the buffer limit")
			case <-time.After(50 * time.Millisecond):
			}
			up.mu.Lock()
			assert.Equal(t, int64(0), up.buffered)
			up.mu.Unlock()

			// Wake the blocked goroutines so they don't leak.
			up.mu.Lock()
			up.closed = true
			up.broadcast()
			up.mu.Unlock()
			for range 3 {
				<-admitted
			}
		})
	}
}

// TestMultipartBufferLimitCountsPages checks that the reorder buffer limit
// is charged in the whole pool pages a buffered part occupies, not the part's
// length, so a client can't pin a page per byte by sending tiny parts out of
// order.
func TestMultipartBufferLimitCountsPages(t *testing.T) {
	b, f, bucket := newPutTestBackend(t, "", nil)
	b.s.opt.MultipartStreamingBufferLimit = 4 * pool.BufferSize
	ctx := context.Background()
	const object = "tiny-parts.bin"

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, object, nil)
	require.NoError(t, err)

	// Parts 2 to 5 fill the buffer's 4 pages with one byte each.
	upload := func(partNumber int) (string, error) {
		body := []byte{byte('0' + partNumber)}
		return b.UploadPart(ctx, bucket, object, uploadID, partNumber, 1, bytes.NewReader(body))
	}
	etags := make([]string, 7)
	for partNumber := 2; partNumber <= 5; partNumber++ {
		etags[partNumber], err = upload(partNumber)
		require.NoError(t, err)
	}

	// Part 6 needs a fifth page so must wait for the buffer to drain.
	done := make(chan error, 1)
	go func() {
		var err error
		etags[6], err = upload(6)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("part 6 was not blocked by the buffer limit (err=%v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	// Part 1 lets the buffered parts stream, freeing the pages for part 6.
	etags[1], err = upload(1)
	require.NoError(t, err)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("part 6 was never unblocked")
	}

	parts := make([]gofakes3.CompletedPart, 0, 6)
	for partNumber := 1; partNumber <= 6; partNumber++ {
		parts = append(parts, gofakes3.CompletedPart{PartNumber: partNumber, ETag: etags[partNumber]})
	}
	_, _, err = b.CompleteMultipartUpload(ctx, bucket, object, uploadID, &gofakes3.CompleteMultipartUploadRequest{Parts: parts})
	require.NoError(t, err)
	assert.Equal(t, []byte("123456"), readObject(t, f, bucket, object))
}

// poolWatchReader is a part body which records the most pool pages in use,
// above those in use when it was made, while it is being read.
type poolWatchReader struct {
	io.Reader
	baseline int
	maxPages int
}

func newPoolWatchReader(data []byte) *poolWatchReader {
	return &poolWatchReader{Reader: bytes.NewReader(data), baseline: pool.Global().InUse()}
}

func (r *poolWatchReader) Read(p []byte) (int, error) {
	r.maxPages = max(r.maxPages, pool.Global().InUse()-r.baseline)
	n, err := r.Reader.Read(p)
	r.maxPages = max(r.maxPages, pool.Global().InUse()-r.baseline)
	return n, err
}

// TestMultipartRetryAfterStreamedNotBuffered checks that a re-upload of a
// part which has already been streamed is only hashed to compare it with the
// original, not buffered in memory.
func TestMultipartRetryAfterStreamedNotBuffered(t *testing.T) {
	b, f, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	const object = "retry-not-buffered.bin"
	const partSize = 8 * pool.BufferSize

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, object, nil)
	require.NoError(t, err)
	part1 := []byte(random.String(partSize))
	etag, err := b.UploadPart(ctx, bucket, object, uploadID, 1, partSize, bytes.NewReader(part1))
	require.NoError(t, err)

	body := newPoolWatchReader(part1)
	retryETag, err := b.UploadPart(ctx, bucket, object, uploadID, 1, partSize, body)
	require.NoError(t, err)
	assert.Equal(t, etag, retryETag)
	assert.Less(t, body.maxPages, partSize/pool.BufferSize/2, "the retried part was buffered in memory")

	// A different re-upload is still rejected.
	other := []byte(random.String(partSize))
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, partSize, bytes.NewReader(other))
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrNotImplemented), "want NotImplemented, got %v", err)

	_, _, err = b.CompleteMultipartUpload(ctx, bucket, object, uploadID, &gofakes3.CompleteMultipartUploadRequest{
		Parts: []gofakes3.CompletedPart{{PartNumber: 1, ETag: etag}},
	})
	require.NoError(t, err)
	assert.Equal(t, part1, readObject(t, f, bucket, object))
}

// TestMultipartBufferWaitGivesUp checks that a part waiting for room in the
// reorder buffer gives up when its request is cancelled, and tells the client
// to slow down if it has waited too long, so a waiting request can't be held
// open forever.
func TestMultipartBufferWaitGivesUp(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	b.s.opt.MultipartStreamingBufferLimit = pool.BufferSize
	oldTimeout := multipartWaitTimeout
	multipartWaitTimeout = 100 * time.Millisecond
	t.Cleanup(func() { multipartWaitTimeout = oldTimeout })
	ctx := context.Background()
	const object = "wait.bin"

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, object, nil)
	require.NoError(t, err)

	// Part 2 fills the buffer, so part 3 must wait.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 2, 1, bytes.NewReader([]byte("2")))
	require.NoError(t, err)

	cancelCtx, cancel := context.WithCancel(ctx)
	time.AfterFunc(10*time.Millisecond, cancel)
	_, err = b.UploadPart(cancelCtx, bucket, object, uploadID, 3, 1, bytes.NewReader([]byte("3")))
	require.ErrorIs(t, err, context.Canceled)

	_, err = b.UploadPart(ctx, bucket, object, uploadID, 3, 1, bytes.NewReader([]byte("3")))
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrSlowDown), "want SlowDown, got %v", err)

	require.NoError(t, b.AbortMultipartUpload(ctx, bucket, object, uploadID))
}

// bufferSink is a multipartUpload sink which collects what is written to it.
type bufferSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *bufferSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *bufferSink) Close() error { return nil }

func (s *bufferSink) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.buf.Bytes())
}

// newBufferSinkUpload starts a multipart upload on b whose parts are
// collected in the returned sink instead of being written to the VFS, so the
// memory used by the VFS upload doesn't hide the memory used by serve s3.
func newBufferSinkUpload(t *testing.T, b *s3Backend, bucket, object string) (gofakes3.UploadID, *bufferSink) {
	uploadID, err := b.CreateMultipartUpload(context.Background(), bucket, object, nil)
	require.NoError(t, err)
	up, err := b.loadUpload(context.Background(), uploadID)
	require.NoError(t, err)
	if up.fh != nil {
		// This fails the VFS upload, returning errBoom
		_ = up.fh.(interface{ CloseWithError(error) error }).CloseWithError(errBoom)
	}
	sink := &bufferSink{}
	up.fh = sink
	t.Cleanup(func() { _ = b.AbortMultipartUpload(context.Background(), bucket, object, uploadID) })
	return uploadID, sink
}

// TestMultipartInOrderPartNotBuffered checks that the part the stream needs
// is written straight to the sink rather than buffered in memory, however
// big it is compared with the buffer limit.
func TestMultipartInOrderPartNotBuffered(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	b.s.opt.MultipartStreamingBufferLimit = pool.BufferSize
	ctx := context.Background()
	const object = "in-order.bin"
	const partSize = 8 * pool.BufferSize

	uploadID, sink := newBufferSinkUpload(t, b, bucket, object)
	data := []byte(random.String(partSize))
	body := newPoolWatchReader(data)
	_, err := b.UploadPart(ctx, bucket, object, uploadID, 1, partSize, body)
	require.NoError(t, err)
	assert.Equal(t, 0, body.maxPages, "the in-order part was buffered in memory")
	assert.Equal(t, data, sink.Bytes())
}

// failAfterReader returns data and then fails with err.
func failAfterReader(data []byte, err error) io.Reader {
	return io.MultiReader(bytes.NewReader(data), iotestErrReader{err})
}

type iotestErrReader struct{ err error }

func (r iotestErrReader) Read([]byte) (int, error) { return 0, r.err }

// TestMultipartResumeFailedPart checks that when the body of a part being
// streamed fails part way, a retry of the part with the same contents
// carries on from where it stopped, while one with different contents is
// rejected without disturbing the stream.
func TestMultipartResumeFailedPart(t *testing.T) {
	b, f, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	const object = "resume.bin"

	part1 := []byte(random.String(300 * 1024))
	part2 := []byte(random.String(100 * 1024))
	want := append(append([]byte(nil), part1...), part2...)

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, object, nil)
	require.NoError(t, err)

	// The first attempt at part 1 fails after 123457 bytes reach the sink.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), failAfterReader(part1[:123457], errBoom))
	require.ErrorIs(t, err, errBoom)

	// A retry which fails before the resume point changes nothing.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), failAfterReader(part1[:1000], errBoom))
	require.ErrorIs(t, err, errBoom)

	// A retry with different contents is rejected.
	other := append([]byte(nil), part1...)
	other[100] ^= 1
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(other)), bytes.NewReader(other))
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrNotImplemented), "want NotImplemented, got %v", err)

	// As is a retry shorter than what is already in the stream.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, 1000, bytes.NewReader(part1[:1000]))
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrNotImplemented), "want NotImplemented, got %v", err)

	// A second failure part way through extends what is in the stream.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), failAfterReader(part1[:200000], errBoom))
	require.ErrorIs(t, err, errBoom)

	// The whole retry completes the part.
	p1, err := b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), bytes.NewReader(part1))
	require.NoError(t, err)
	p2, err := b.UploadPart(ctx, bucket, object, uploadID, 2, int64(len(part2)), bytes.NewReader(part2))
	require.NoError(t, err)

	_, _, err = b.CompleteMultipartUpload(ctx, bucket, object, uploadID, &gofakes3.CompleteMultipartUploadRequest{
		Parts: []gofakes3.CompletedPart{{PartNumber: 1, ETag: p1}, {PartNumber: 2, ETag: p2}},
	})
	require.NoError(t, err)
	assert.Equal(t, want, readObject(t, f, bucket, object))
}

// TestMultipartCompleteWithPartialPart checks that an upload whose stream
// holds part of a failed part can't be completed without it.
func TestMultipartCompleteWithPartialPart(t *testing.T) {
	b, f, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	const object = "partial.bin"

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, object, nil)
	require.NoError(t, err)
	part1 := []byte(random.String(200 * 1024))
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), failAfterReader(part1[:150000], errBoom))
	require.ErrorIs(t, err, errBoom)

	_, _, err = b.CompleteMultipartUpload(ctx, bucket, object, uploadID, &gofakes3.CompleteMultipartUploadRequest{})
	require.ErrorIs(t, err, gofakes3.ErrInvalidPart)
	_, err = f.NewObject(ctx, path.Join(bucket, object))
	require.ErrorIs(t, err, fs.ErrorObjectNotFound)
}

// TestMultipartConcurrentDuplicateParts checks that further requests for the
// part being streamed wait, without buffering it, for the first to finish,
// then are checked against it.
func TestMultipartConcurrentDuplicateParts(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	const object = "duplicates.bin"
	const partSize = 8 * pool.BufferSize

	uploadID, sink := newBufferSinkUpload(t, b, bucket, object)
	data := []byte(random.String(partSize))

	// The first request stalls half way through its body.
	pr, pw := io.Pipe()
	first := make(chan error, 1)
	var firstETag string
	go func() {
		var err error
		firstETag, err = b.UploadPart(ctx, bucket, object, uploadID, 1, partSize, pr)
		first <- err
	}()
	_, err := pw.Write(data[:partSize/2])
	require.NoError(t, err)

	type result struct {
		etag string
		err  error
	}
	const duplicates = 4
	bodies := make([]*poolWatchReader, duplicates)
	results := make(chan result, duplicates)
	for i := range bodies {
		bodies[i] = newPoolWatchReader(data)
		go func(body io.Reader) {
			etag, err := b.UploadPart(ctx, bucket, object, uploadID, 1, partSize, body)
			results <- result{etag, err}
		}(bodies[i])
	}
	select {
	case r := <-results:
		t.Fatalf("a duplicate finished while the first request was streaming (err=%v)", r.err)
	case <-time.After(200 * time.Millisecond):
	}

	_, err = pw.Write(data[partSize/2:])
	require.NoError(t, err)
	// The body must end, as a request body does, so that the part is
	// read to its end.
	require.NoError(t, pw.Close())
	require.NoError(t, <-first)
	for range duplicates {
		r := <-results
		require.NoError(t, r.err)
		assert.Equal(t, firstETag, r.etag)
	}
	for _, body := range bodies {
		assert.Equal(t, 0, body.maxPages, "a duplicate part was buffered in memory")
	}
	assert.Equal(t, data, sink.Bytes())
}

// TestMultipartPartFailsAfterWholeBody checks that a part whose body is
// delivered in full but is then rejected - as gofakes3's Content-MD5 check
// does, after every byte has reached the backend - fails the whole upload,
// rather than leaving it in a state no retry of the part can escape.
func TestMultipartPartFailsAfterWholeBody(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	const object = "bad-digest.bin"

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, object, nil)
	require.NoError(t, err)
	part1 := []byte(random.String(100 * 1024))

	// The whole body arrives, then the integrity check rejects it.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)),
		failAfterReader(part1, gofakes3.ErrBadDigest))
	require.Error(t, err)

	// The upload is gone, so the client starts again rather than retrying
	// a part which could never be accepted.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), bytes.NewReader(part1))
	require.ErrorIs(t, err, gofakes3.ErrNoSuchUpload)
	_, err = b.loadUpload(context.Background(), uploadID)
	require.ErrorIs(t, err, gofakes3.ErrNoSuchUpload)
}

// flakySink is a sink which fails the write which would take it past
// failAt bytes, taking part of it, and accepts everything after that.
type flakySink struct {
	bufferSink
	failAt int
	failed bool
}

func (s *flakySink) Write(p []byte) (int, error) {
	if s.failed || s.buf.Len()+len(p) <= s.failAt {
		return s.bufferSink.Write(p)
	}
	s.failed = true
	n, _ := s.bufferSink.Write(p[:s.failAt-s.buf.Len()])
	return n, errBoom
}

// TestMultipartPumpFailure checks that an upload is failed when a part
// which was buffered, and so already acknowledged to the client, can't be
// written to the sink, rather than a resend of it being appended to what
// the failed write left there.
func TestMultipartPumpFailure(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()
	const object = "pump-failure.bin"

	uploadID, _ := newBufferSinkUpload(t, b, bucket, object)
	up, err := b.loadUpload(ctx, uploadID)
	require.NoError(t, err)
	part1 := []byte(random.String(1024))
	part2 := []byte(random.String(1024))
	up.fh = &flakySink{failAt: len(part1) + len(part2)/2}

	// Part 2 arrives first so is buffered, then fails part way into the
	// sink when part 1 arrives and it is pumped.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 2, int64(len(part2)), bytes.NewReader(part2))
	require.NoError(t, err)
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 1, int64(len(part1)), bytes.NewReader(part1))
	require.ErrorIs(t, err, errBoom)

	// The upload is gone rather than accepting part 2 again.
	_, err = b.UploadPart(ctx, bucket, object, uploadID, 2, int64(len(part2)), bytes.NewReader(part2))
	require.ErrorIs(t, err, gofakes3.ErrNoSuchUpload)
}

// TestMultipartPartBadMD5 checks that a part whose body doesn't match the
// Content-MD5 the client declared is rejected, and that the upload it
// corrupted is failed rather than left for the client to retry.
func TestMultipartPartBadMD5(t *testing.T) {
	core, f, bucket := newMultipartTestServer(t, false)
	ctx := context.Background()
	const object = "bad-md5.bin"

	uploadID, err := core.NewMultipartUpload(ctx, bucket, object, minio.PutObjectOptions{})
	require.NoError(t, err)

	part1 := []byte(random.String(60 * 1024))
	wrongMD5 := md5.Sum([]byte("not the part"))
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(part1), int64(len(part1)),
		minio.PutObjectPartOptions{Md5Base64: base64.StdEncoding.EncodeToString(wrongMD5[:])})
	require.Error(t, err)

	// Nothing was committed at the key and the upload is gone.
	_, err = f.NewObject(ctx, path.Join(bucket, object))
	require.ErrorIs(t, err, fs.ErrorObjectNotFound)
	_, err = core.PutObjectPart(ctx, bucket, object, uploadID, 1, bytes.NewReader(part1), int64(len(part1)), minio.PutObjectPartOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NoSuchUpload")
	requireOnly(t, f, bucket)
}

// TestLimitedBody checks that a part body is read to its end, so that an
// error reported there is not missed, and that a body longer than declared
// is rejected.
func TestLimitedBody(t *testing.T) {
	for _, test := range []struct {
		name string
		body io.Reader
		want error
	}{
		{"EndError", failAfterReader([]byte("hello"), gofakes3.ErrBadDigest), gofakes3.ErrBadDigest},
		{"TooLong", bytes.NewReader([]byte("hello world")), gofakes3.ErrIncompleteBody},
		{"Exact", bytes.NewReader([]byte("hello")), nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			n, err := io.Copy(io.Discard, newLimitedBody(test.body, 5))
			assert.Equal(t, int64(5), n)
			if test.want == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, test.want)
			}
		})
	}
}

// TestMultipartBufferTotal checks that --multipart-streaming-buffer-total
// limits the memory buffered across all the uploads of a user but not
// those of other users, and that an aborted upload's buffered parts are
// returned to it.
func TestMultipartBufferTotal(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	b.s.opt.MultipartStreamingBufferTotal = 2 * pool.BufferSize
	ctx := context.Background()

	uploadAs := func(ctx context.Context, object string, uploadID gofakes3.UploadID, partNumber int) (string, error) {
		body := []byte{byte('0' + partNumber)}
		return b.UploadPart(ctx, bucket, object, uploadID, partNumber, 1, bytes.NewReader(body))
	}
	upload := func(object string, uploadID gofakes3.UploadID, partNumber int) (string, error) {
		return uploadAs(ctx, object, uploadID, partNumber)
	}

	// Upload A buffers two out-of-order parts, using up the total.
	idA, err := b.CreateMultipartUpload(ctx, bucket, "a", nil)
	require.NoError(t, err)
	for partNumber := 2; partNumber <= 3; partNumber++ {
		_, err = upload("a", idA, partNumber)
		require.NoError(t, err)
	}

	// Another user's out-of-order part doesn't wait.
	ctxOther := tenantCtx("other")
	idOther, err := b.CreateMultipartUpload(ctxOther, bucket, "other", nil)
	require.NoError(t, err)
	_, err = uploadAs(ctxOther, "other", idOther, 2)
	require.NoError(t, err)
	require.NoError(t, b.AbortMultipartUpload(ctxOther, bucket, "other", idOther))

	// Upload B's out-of-order part must wait although its own buffer is
	// empty.
	idB, err := b.CreateMultipartUpload(ctx, bucket, "b", nil)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		_, err := upload("b", idB, 2)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("upload b's part was not blocked by the buffer total (err=%v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	// Aborting upload A frees its buffered parts.
	require.NoError(t, b.AbortMultipartUpload(ctx, bucket, "a", idA))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("upload b's part was never unblocked")
	}
	require.NoError(t, b.AbortMultipartUpload(ctx, bucket, "b", idB))

	budget := b.tenant(ctx).budget
	budget.mu.Lock()
	assert.Equal(t, int64(0), budget.used, "buffer total not all returned")
	budget.mu.Unlock()
}

// TestMultipartSinkOpenedOnFirstPart checks that starting a multipart upload
// doesn't open its VFS upload, so an upload with no parts holds no VFS or
// backend resources, and that an upload completed without parts still
// creates an empty object.
func TestMultipartSinkOpenedOnFirstPart(t *testing.T) {
	b, f, bucket := newPutTestBackend(t, "", nil)
	ctx := context.Background()

	sinkOpen := func(uploadID gofakes3.UploadID) bool {
		up, err := b.loadUpload(context.Background(), uploadID)
		require.NoError(t, err)
		up.mu.Lock()
		defer up.mu.Unlock()
		return up.fh != nil
	}

	uploadID, err := b.CreateMultipartUpload(ctx, bucket, "parts.bin", nil)
	require.NoError(t, err)
	assert.False(t, sinkOpen(uploadID), "the sink was opened before any part arrived")

	// A buffered part doesn't need the sink either.
	part2 := []byte("world")
	p2, err := b.UploadPart(ctx, bucket, "parts.bin", uploadID, 2, int64(len(part2)), bytes.NewReader(part2))
	require.NoError(t, err)
	assert.False(t, sinkOpen(uploadID), "the sink was opened for a buffered part")

	part1 := []byte("hello ")
	p1, err := b.UploadPart(ctx, bucket, "parts.bin", uploadID, 1, int64(len(part1)), bytes.NewReader(part1))
	require.NoError(t, err)
	assert.True(t, sinkOpen(uploadID))
	_, _, err = b.CompleteMultipartUpload(ctx, bucket, "parts.bin", uploadID, &gofakes3.CompleteMultipartUploadRequest{
		Parts: []gofakes3.CompletedPart{{PartNumber: 1, ETag: p1}, {PartNumber: 2, ETag: p2}},
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("hello world"), readObject(t, f, bucket, "parts.bin"))

	// An upload aborted without parts leaves nothing behind.
	uploadID, err = b.CreateMultipartUpload(ctx, bucket, "aborted.bin", nil)
	require.NoError(t, err)
	require.NoError(t, b.AbortMultipartUpload(ctx, bucket, "aborted.bin", uploadID))

	// An upload completed without parts makes an empty object.
	uploadID, err = b.CreateMultipartUpload(ctx, bucket, "empty.bin", nil)
	require.NoError(t, err)
	_, _, err = b.CompleteMultipartUpload(ctx, bucket, "empty.bin", uploadID, &gofakes3.CompleteMultipartUploadRequest{})
	require.NoError(t, err)
	assert.Equal(t, []byte{}, readObject(t, f, bucket, "empty.bin"))

	requireOnly(t, f, bucket, "parts.bin", "empty.bin")
}

// TestMultipartMaxUploads checks that a user can have no more than
// --multipart-max-uploads multipart uploads in progress at once, that
// other users can still start theirs, and that finished uploads make room
// for more.
func TestMultipartMaxUploads(t *testing.T) {
	b, _, bucket := newPutTestBackend(t, "", nil)
	b.s.opt.MultipartMaxUploads = 2
	ctx := context.Background()

	id1, err := b.CreateMultipartUpload(ctx, bucket, "1", nil)
	require.NoError(t, err)
	id2, err := b.CreateMultipartUpload(ctx, bucket, "2", nil)
	require.NoError(t, err)
	_, err = b.CreateMultipartUpload(ctx, bucket, "3", nil)
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrSlowDown), "want SlowDown, got %v", err)

	// Another user isn't limited by them.
	ctxOther := tenantCtx("other")
	idOther, err := b.CreateMultipartUpload(ctxOther, bucket, "other", nil)
	require.NoError(t, err)
	require.NoError(t, b.AbortMultipartUpload(ctxOther, bucket, "other", idOther))

	// Aborting an upload makes room.
	require.NoError(t, b.AbortMultipartUpload(ctx, bucket, "1", id1))
	id3, err := b.CreateMultipartUpload(ctx, bucket, "3", nil)
	require.NoError(t, err)
	_, err = b.CreateMultipartUpload(ctx, bucket, "4", nil)
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrSlowDown), "want SlowDown, got %v", err)

	// So does completing one.
	_, _, err = b.CompleteMultipartUpload(ctx, bucket, "2", id2, &gofakes3.CompleteMultipartUploadRequest{})
	require.NoError(t, err)
	_, err = b.CreateMultipartUpload(ctx, bucket, "4", nil)
	require.NoError(t, err)

	// A refused upload leaves nothing behind on the remote.
	_, err = b.CreateMultipartUpload(ctx, bucket, "dir/deep/4", nil)
	assert.True(t, gofakes3.HasErrorCode(err, gofakes3.ErrSlowDown), "want SlowDown, got %v", err)
	_vfs, err := b.s.getVFS(ctx)
	require.NoError(t, err)
	_, err = _vfs.Stat(bucket + "/dir")
	assert.ErrorIs(t, err, vfs.ENOENT, "a refused upload created directories")

	// And so does expiring one.
	up, err := b.loadUpload(context.Background(), id3)
	require.NoError(t, err)
	up.mu.Lock()
	up.lastUsed = time.Now().Add(-2 * time.Hour)
	up.mu.Unlock()
	b.reapExpiredUploads(time.Now(), time.Hour)
	_, err = b.CreateMultipartUpload(ctx, bucket, "5", nil)
	require.NoError(t, err)
}
