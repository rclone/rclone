// Multipart upload tests for serve s3.

package s3

import (
	"bytes"
	"context"
	"crypto/md5"
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
		opt.MultipartStreamingBufferLimit = 50 * 1024
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

	// Part 3 arrives first: the buffer is empty so it is admitted even though
	// it must wait for its turn.
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

// TestMultipartAbortDuringUploadPart aborts the upload between a part's
// waitForTurn and its streamPart - as happens when the abort arrives while
// the part body is still being received from the client - and checks that
// streamPart fails cleanly instead of panicking on the torn-down upload.
func TestMultipartAbortDuringUploadPart(t *testing.T) {
	up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 0)
	sink := &stubSink{}
	up.fh = sink

	// An UploadPart in progress: the part is admitted, then the abort lands
	// while its body is still being received.
	contents := []byte("hello")
	require.NoError(t, up.waitForTurn(1, int64(len(contents))))
	require.NoError(t, up.abort())

	// The abort must abandon the write rather than committing it.
	assert.True(t, sink.closed)
	assert.Equal(t, errMultipartAborted, sink.abortErr)

	// The UploadPart resumes: it buffers the part and calls streamPart.
	rw := multipart.NewRW()
	_, err := rw.Write(contents)
	require.NoError(t, err)
	md5Sum := md5.Sum(contents)
	err = up.streamPart(1, int64(len(contents)), md5Sum[:], rw)
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
	require.NoError(t, up.waitForTurn(2, int64(len(contents))))
	require.NoError(t, up.streamPart(2, int64(len(contents)), md5Sum[:], rw))

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
	b.multipartUploads.Store(uploadID, up)

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
	b.multipartUploads.Store(uploadID, up)

	_, _, err = b.CompleteMultipartUpload(ctx, bucket, "key", uploadID, &gofakes3.CompleteMultipartUploadRequest{})
	require.Error(t, err)
	_, err = b.loadUpload(uploadID)
	require.NoError(t, err, "the upload record must survive a retryable Complete failure")
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

	// Wait for well over the expiry and the reaper interval, then the
	// upload must be gone.
	time.Sleep(time.Second)
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
		b.multipartUploads.Store(gofakes3.UploadID(id), up)
		return up
	}

	const expiry = time.Hour
	now := time.Now()

	newUp("fresh")
	idle := newUp("idle")
	busy := newUp("busy")
	busy.startActivity()
	for _, up := range []*multipartUpload{idle, busy} {
		up.mu.Lock()
		up.lastUsed = now.Add(-2 * expiry)
		up.mu.Unlock()
	}

	b.reapExpiredUploads(now, expiry)

	_, err = b.loadUpload("fresh")
	assert.NoError(t, err, "a fresh upload must not be reaped")
	_, err = b.loadUpload("busy")
	assert.NoError(t, err, "an upload with a request in flight must not be reaped")
	_, err = b.loadUpload("idle")
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
// rejects a negative client-declared part length and that a huge declared
// length cannot overflow the running total so as to admit a further part past
// the buffer limit.
func TestWaitForTurnRejectsBogusSize(t *testing.T) {
	up := newMultipartUpload("bucket", "key", "bucket/key", "bucket/key", nil, 1<<20)

	// A part length can never be negative.
	require.ErrorIs(t, up.waitForTurn(1, -1), gofakes3.ErrInvalidArgument)

	// A huge out-of-order part is admitted once because the buffer is empty,
	// driving buffered near the top of the int64 range.
	require.NoError(t, up.waitForTurn(2, math.MaxInt64))

	// A further out-of-order part must wait, not be wrongly admitted by an
	// overflow of buffered+size.
	admitted := make(chan struct{})
	go func() {
		_ = up.waitForTurn(3, math.MaxInt64)
		close(admitted)
	}()
	select {
	case <-admitted:
		t.Fatal("out-of-order part admitted past the buffer limit via overflow")
	case <-time.After(50 * time.Millisecond):
	}

	// Wake the blocked goroutine so it doesn't leak.
	up.mu.Lock()
	up.closed = true
	up.cond.Broadcast()
	up.mu.Unlock()
	<-admitted
}
