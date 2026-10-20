// Multipart upload support for serve s3.
//
// Multipart uploads received by serve s3 are written, in part-number order,
// through the VFS, exactly like a plain PutObject: to a temporary object
// which is renamed into place on completion, so the object at the key only
// ever changes atomically on success. With the default --vfs-cache-mode off
// the parts stream through the VFS into a single upload to the remote; with
// --vfs-cache-mode writes or above they are buffered in the VFS cache and
// uploaded by its write-back. This implements the gofakes3.MultipartBackend
// interface on s3Backend.
//
// When streaming is disabled (--disable-multipart-streaming),
// ErrMultipartUploadNotSupported is returned so that gofakes3 falls back to
// buffering the parts in memory.

package s3

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/gofakes3"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pool"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// multipartUploadPrefix is prepended to the leaf name of the temporary object
// a streamed multipart upload is written to before it is moved into place.
const multipartUploadPrefix = tempObjectPrefix + "multipart_"

// multipartWaitTimeout is how long a part waits for room in the reorder
// buffer before the client is told to slow down and retry it.
var multipartWaitTimeout = time.Minute

// multipartUpload tracks one in-flight S3 multipart upload. The parts are
// written, in part-number order, into fh - a VFS file handle which either
// streams straight through to the remote (the default) or is backed by the
// VFS cache (when the VFS is caching writes).
type multipartUpload struct {
	bucket, key string
	fp          string // final object path
	streamFp    string // path the parts are written to (fp when the remote has no server-side move or copy)
	meta        map[string]string

	fh     io.WriteCloser // sink the in-order parts are written to, opened by sink (guarded by mu)
	vfs    *vfs.VFS       // the VFS fh is created on, used for all later operations
	tenant *tenant        // the user the upload belongs to

	mu        sync.Mutex
	changed   chan struct{}  // closed when buffered shrinks, nextPart advances or the upload closes
	partMD5s  map[int][]byte // raw MD5 sums per part (for the final S3 multipart ETag)
	partSizes map[int]int64  // observed part sizes
	closed    bool
	aborted   bool      // closed by abort, so nothing was committed
	active    int       // requests in flight for this upload, protecting it from the reaper
	lastUsed  time.Time // when the last request for this upload finished

	nextPart    int              // next part number to stream (1-based)
	streamBuf   map[int]*pool.RW // parts received ahead of nextPart, awaiting their turn
	pumping     bool             // a goroutine is currently writing to the sink
	partialSize int64            // bytes of part nextPart in the sink from an attempt which failed part way
	poisoned    bool             // the stream holds data the upload can never complete with
	partialMD5  []byte           // MD5 state after those partialSize bytes (encoding.BinaryMarshaler)
	buffered    int64            // bytes of parts admitted but not yet streamed or released
	bufferLimit int64            // max buffered before parts ahead of nextPart must wait (<= 0 for no limit)
	budget      *bufferBudget    // limits buffered across all the user's uploads (nil for no limit)
}

// bufferBudget limits the memory used by the reorder buffers of all the
// multipart uploads of a user.
type bufferBudget struct {
	mu      sync.Mutex
	limit   int64         // <= 0 for no limit
	used    int64         // bytes reserved
	changed chan struct{} // closed when used shrinks
}

// newBufferBudget makes a bufferBudget of limit bytes, <= 0 for no limit.
func newBufferBudget(limit int64) *bufferBudget {
	return &bufferBudget{
		limit:   limit,
		changed: make(chan struct{}),
	}
}

// tryReserve reserves n bytes if they fit in the budget. If not it returns
// a channel which is closed when some are released. A nil budget has no
// limit.
func (bb *bufferBudget) tryReserve(n int64) (ok bool, changed <-chan struct{}) {
	if bb == nil {
		return true, nil
	}
	bb.mu.Lock()
	defer bb.mu.Unlock()
	if bb.limit > 0 && n > bb.limit-bb.used {
		return false, bb.changed
	}
	bb.used += n
	return true, nil
}

// release returns n bytes reserved with tryReserve.
func (bb *bufferBudget) release(n int64) {
	if bb == nil || n == 0 {
		return
	}
	bb.mu.Lock()
	defer bb.mu.Unlock()
	bb.used -= n
	close(bb.changed)
	bb.changed = make(chan struct{})
}

// newMultipartUpload allocates an upload struct.
func newMultipartUpload(bucket, key, fp, streamFp string, meta map[string]string, bufferLimit int64) *multipartUpload {
	up := &multipartUpload{
		bucket:      bucket,
		key:         key,
		fp:          fp,
		streamFp:    streamFp,
		meta:        meta,
		partMD5s:    map[int][]byte{},
		partSizes:   map[int]int64{},
		nextPart:    1,
		streamBuf:   map[int]*pool.RW{},
		bufferLimit: bufferLimit,
		lastUsed:    time.Now(),
		changed:     make(chan struct{}),
	}
	return up
}

// broadcast wakes everything waiting on up.changed. Call with up.mu held.
func (up *multipartUpload) broadcast() {
	close(up.changed)
	up.changed = make(chan struct{})
}

// unbuffer returns charge bytes of buffer reserved by waitForTurn to the
// upload's and the user's budgets. Call with up.mu held.
func (up *multipartUpload) unbuffer(charge int64) {
	up.buffered -= charge
	up.budget.release(charge)
	up.broadcast()
}

// startActivity marks the upload as having a request in flight, holding
// its VFS until the matching endActivity. It returns ErrNoSuchUpload if
// the upload has already gone.
func (up *multipartUpload) startActivity() error {
	if !up.vfs.Hold() {
		return gofakes3.ErrNoSuchUpload
	}
	up.mu.Lock()
	up.active++
	up.mu.Unlock()
	return nil
}

// endActivity marks the request done and restarts the upload's idle time.
func (up *multipartUpload) endActivity() {
	up.mu.Lock()
	up.active--
	up.lastUsed = time.Now()
	up.mu.Unlock()
	up.vfs.Shutdown()
}

// loadUpload looks up an in-flight upload of the user making the request
// in ctx by ID.
func (b *s3Backend) loadUpload(ctx context.Context, uploadID gofakes3.UploadID) (*multipartUpload, error) {
	v, ok := b.tenant(ctx).uploads.Load(uploadID)
	if !ok {
		return nil, gofakes3.ErrNoSuchUpload
	}
	return v.(*multipartUpload), nil
}

// reserveUpload reserves a place for a new upload of t, failing with
// gofakes3.ErrSlowDown if t already has --multipart-max-uploads in
// progress. The place is given back by releaseUpload or deleteUpload.
func (b *s3Backend) reserveUpload(t *tenant) error {
	n := t.nUploads.Add(1)
	if maxUploads := int64(b.s.opt.MultipartMaxUploads); maxUploads > 0 && n > maxUploads {
		t.nUploads.Add(-1)
		b.warnMaxUploadsOnce.Do(func() {
			fs.Logf(nil, "serve s3: telling a client to slow down as it has --multipart-max-uploads %d multipart uploads in progress", maxUploads)
		})
		return gofakes3.ErrSlowDown
	}
	return nil
}

// releaseUpload gives back a place reserved for an upload of t which was
// never recorded with addUpload.
func (b *s3Backend) releaseUpload(t *tenant) {
	t.nUploads.Add(-1)
}

// addUpload records a new in-flight upload of t, which must hold a place
// reserved by reserveUpload.
//
// The upload holds its VFS until deleteUpload, as an auth proxy would
// otherwise shut it down once unused for a while, even between the
// requests of an upload.
func (b *s3Backend) addUpload(t *tenant, uploadID gofakes3.UploadID, up *multipartUpload) error {
	if !up.vfs.Hold() {
		return errors.New("serve s3: VFS shut down while creating multipart upload")
	}
	up.tenant = t
	t.uploads.Store(uploadID, up)
	return nil
}

// deleteUpload removes the record of an in-flight upload, if present.
func (b *s3Backend) deleteUpload(uploadID gofakes3.UploadID, up *multipartUpload) {
	if _, ok := up.tenant.uploads.LoadAndDelete(uploadID); ok {
		up.vfs.Shutdown()
		up.tenant.nUploads.Add(-1)
	}
}

// CreateMultipartUpload begins a new multipart upload.
//
// The parts are written, in part-number order, through the VFS to a temporary
// object which is renamed into place on completion. With the default
// --vfs-cache-mode off the write streams through to the remote as the parts
// arrive; with --vfs-cache-mode writes or above it lands in the VFS cache and
// is uploaded by the write-back. Either way an aborted or failed upload never
// makes a partial object visible at the final path or disturbs a pre-existing
// one.
//
// On a remote with no server-side move or copy the parts are written straight
// to the final object instead, trading some atomicity for never buffering in
// memory: the in-flight upload is visible at the key, and a failed or aborted
// upload can leave partial data there when the remote doesn't upload atomically
// or the VFS is caching writes (a write to the cache can't be abandoned).
//
// With --disable-multipart-streaming, ErrMultipartUploadNotSupported is
// returned so that gofakes3 falls back to buffering the whole upload in
// memory; a one-off NOTICE warns about the memory use. A caching VFS needs
// no streaming support, so it ignores the flag.
//
// The object the parts are written to is only created when the first part
// arrives, so an upload with no parts holds no file handle or backend
// upload, only a reference to its VFS.
// No user can have more than --multipart-max-uploads in progress: after that
// gofakes3.ErrSlowDown is returned before anything is created on the
// remote.
func (b *s3Backend) CreateMultipartUpload(ctx context.Context, bucketName, objectName string, meta map[string]string) (gofakes3.UploadID, error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return "", err
	}
	if _, err := _vfs.Stat(bucketName); err != nil {
		return "", gofakes3.BucketNotFound(bucketName)
	}

	if b.s.opt.DisableMultipartStreaming && _vfs.Opt.CacheMode < vfscommon.CacheModeMinimal {
		b.warnInMemoryOnce.Do(func() {
			fs.Logf(nil, "serve s3: buffering multipart uploads in memory because --disable-multipart-streaming is set - this may use a lot of memory")
		})
		return "", gofakes3.ErrMultipartUploadNotSupported
	}

	fp, err := bucketObjectPath(bucketName, objectName)
	if err != nil {
		return "", err
	}
	// Reserve a place before doing anything with the remote, so a refused
	// upload leaves nothing behind.
	t := b.tenant(ctx)
	if err := b.reserveUpload(t); err != nil {
		return "", err
	}

	objectDir := path.Dir(fp)
	if objectDir != "." {
		if err := mkdirRecursive(objectDir, _vfs); err != nil {
			b.releaseUpload(t)
			return "", err
		}
	}

	uploadID := gofakes3.UploadID(uuid.New().String())
	streamFp := fp
	// Write to a temporary object moved into place on completion if the
	// remote supports server-side move (if not write directly to the final
	// object). Unlike a plain PutObject all remotes use a temporary object.
	// S3 semantics say the key must not show it until it completes. This is
	// at the cost of a server-side copy and delete where the remote has no
	// server side move.
	if operations.CanServerSideMove(_vfs.Fs()) {
		streamFp = path.Join(objectDir, multipartUploadPrefix+string(uploadID))
	}

	up := newMultipartUpload(bucketName, objectName, fp, streamFp, meta, int64(b.s.opt.MultipartStreamingBufferLimit))
	up.budget = t.budget
	up.vfs = _vfs

	if err := b.addUpload(t, uploadID, up); err != nil {
		b.releaseUpload(t)
		return "", err
	}
	return uploadID, nil
}

// UploadPart writes a single part from the S3 client into the streaming upload.
//
// The next part the stream needs is written straight into the sink as it is
// received. A part which arrives ahead of its turn is buffered in memory until
// the stream reaches it, and a re-upload of a part already streamed is just
// hashed to check it matches.
func (b *s3Backend) UploadPart(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, partNumber int, contentLength int64, body io.Reader) (string, error) {
	up, err := b.loadUpload(ctx, uploadID)
	if err != nil {
		return "", err
	}
	if err := up.startActivity(); err != nil {
		return "", err
	}
	defer up.endActivity()

	// The sink must never get more than the declared length, and the body
	// must be read to its end so that an error it only reports there - as
	// the Content-MD5 check does - is not missed.
	body = newLimitedBody(body, contentLength)

	turn, err := up.waitForTurn(ctx, partNumber, contentLength)
	if err != nil {
		return "", err
	}
	switch turn {
	case turnVerify:
		md5Sum, size, _ := up.streamedPart(partNumber)
		return verifyStreamedPart(partNumber, md5Sum, size, contentLength, body)
	case turnStream:
		md5Sum, err := up.streamDirect(partNumber, contentLength, body)
		if err != nil {
			if up.isPoisoned() {
				// The stream holds data the upload can never complete
				// with, so tear it down rather than let the client retry
				// a part which can't be accepted.
				b.failUpload(uploadID, up)
			}
			return "", err
		}
		return fmt.Sprintf("%q", hex.EncodeToString(md5Sum)), nil
	}

	// Buffer the part in a pool-backed RW so we can MD5 it (for the ETag) and
	// stream it once it is this part's turn. The RW grows a page at a time as
	// the body is read.
	rw := multipart.NewRW()
	hasher := md5.New()
	n, err := io.Copy(rw, io.TeeReader(body, hasher))
	if err != nil {
		_ = rw.Close()
		up.release(contentLength)
		return "", err
	}
	if n != contentLength {
		_ = rw.Close()
		up.release(contentLength)
		return "", gofakes3.ErrIncompleteBody
	}
	md5Sum := hasher.Sum(nil)
	etag := fmt.Sprintf("%q", hex.EncodeToString(md5Sum))

	if err := up.bufferPart(ctx, partNumber, n, md5Sum, rw); err != nil {
		if up.isPoisoned() {
			b.failUpload(uploadID, up)
		}
		return "", err
	}
	return etag, nil
}

// limitedBody reads the declared length of a part body and no more, then
// checks the body really ends there. The final read of the underlying body
// is what reports an error such as a Content-MD5 mismatch, so it must
// happen even though its data is not wanted.
type limitedBody struct {
	body      io.Reader
	remaining int64
}

// newLimitedBody returns body limited to size bytes.
func newLimitedBody(body io.Reader, size int64) *limitedBody {
	return &limitedBody{body: body, remaining: max(size, 0)}
}

// maxEmptyReads is how many reads returning nothing are tolerated before a
// body is declared stuck, as io.Copy does.
const maxEmptyReads = 100

func (r *limitedBody) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		// The declared length has been read, so all that is left is to
		// find out how the body ends.
		var b [1]byte
		for range maxEmptyReads {
			n, err := r.body.Read(b[:])
			if n > 0 {
				return 0, gofakes3.ErrIncompleteBody
			}
			if err != nil {
				return 0, err
			}
		}
		return 0, io.ErrNoProgress
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.body.Read(p)
	r.remaining -= int64(n)
	return n, err
}

// streamedPart returns the MD5 sum and size of partNumber if it has already
// been streamed to the sink.
func (up *multipartUpload) streamedPart(partNumber int) (md5Sum []byte, size int64, ok bool) {
	up.mu.Lock()
	defer up.mu.Unlock()
	if partNumber >= up.nextPart {
		return nil, 0, false
	}
	return up.partMD5s[partNumber], up.partSizes[partNumber], true
}

// verifyStreamedPart reads the body of a re-upload of partNumber, which has
// already been streamed with md5Sum and size, returning its ETag if it is
// identical and an error if not, since the stream can't be rewritten.
func verifyStreamedPart(partNumber int, md5Sum []byte, size int64, contentLength int64, body io.Reader) (string, error) {
	hasher := md5.New()
	n, err := io.Copy(hasher, body)
	if err != nil {
		return "", err
	}
	if n != contentLength {
		return "", gofakes3.ErrIncompleteBody
	}
	if n != size || !bytes.Equal(hasher.Sum(nil), md5Sum) {
		return "", errPartAlreadyStreamed(partNumber)
	}
	return fmt.Sprintf("%q", hex.EncodeToString(md5Sum)), nil
}

// errPartAlreadyStreamed is returned for an attempt to replace partNumber
// with different contents after it has been streamed.
func errPartAlreadyStreamed(partNumber int) error {
	return gofakes3.ErrorMessagef(gofakes3.ErrNotImplemented, "part %d has already been streamed to the backend and cannot be replaced with different contents", partNumber)
}

// bufferCharge returns the reorder buffer memory a part of size bytes
// takes: its pool.RW holds it in whole pool pages, however small it is.
func bufferCharge(size int64) int64 {
	if size > math.MaxInt64-pool.BufferSize {
		return math.MaxInt64
	}
	return (size + pool.BufferSize - 1) / pool.BufferSize * pool.BufferSize
}

// partTurn says how an admitted part is to be received.
type partTurn int

const (
	// turnBuffer means buffer the part until the stream reaches it; its
	// bufferCharge has been reserved.
	turnBuffer partTurn = iota
	// turnStream means stream the part straight into the sink with
	// streamDirect: the caller now owns the sink.
	turnStream
	// turnVerify means the part has already been streamed, so the
	// re-upload can only be checked against it.
	turnVerify
)

// waitForTurn blocks until a part of size bytes can be received, and says
// how.
//
// The next part the stream needs is streamed straight into the sink, once
// no other request is writing to it, so it needs no buffer. A part ahead of
// it must wait until its bufferCharge fits within both the upload's reorder
// buffer limit and the user's budget for all their uploads, which it then
// reserves, bounding the memory uploads can consume when clients send parts
// faster than the backend drains them. A part too big for the limits waits
// until it is the next part. Reserved bytes are returned
// with release, or by the pump as the part is streamed.
//
// A part which waits for longer than multipartWaitTimeout gets
// gofakes3.ErrSlowDown, asking the client to retry it later, so a waiting
// request isn't held open indefinitely. The wait also ends if ctx is
// cancelled.
//
// size is the client-declared part length and is not trusted: a negative value
// is rejected, and the admission test is written so a huge value can't overflow
// the running total and wrongly admit further parts past the limit.
func (up *multipartUpload) waitForTurn(ctx context.Context, partNumber int, size int64) (partTurn, error) {
	if size < 0 {
		return 0, gofakes3.ErrInvalidArgument
	}
	charge := bufferCharge(size)
	timer := time.NewTimer(multipartWaitTimeout)
	defer timer.Stop()
	for {
		up.mu.Lock()
		if up.closed {
			up.mu.Unlock()
			return 0, gofakes3.ErrNoSuchUpload
		}
		switch {
		case partNumber < up.nextPart:
			up.mu.Unlock()
			return turnVerify, nil
		case partNumber == up.nextPart && !up.pumping:
			up.pumping = true
			up.mu.Unlock()
			return turnStream, nil
		}
		var budgetChanged <-chan struct{}
		if partNumber > up.nextPart && (up.bufferLimit <= 0 || charge <= up.bufferLimit-up.buffered) {
			var ok bool
			if ok, budgetChanged = up.budget.tryReserve(charge); ok {
				up.buffered += charge
				up.mu.Unlock()
				return turnBuffer, nil
			}
		}
		changed := up.changed
		up.mu.Unlock()
		select {
		case <-changed:
		case <-budgetChanged:
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timer.C:
			return 0, gofakes3.ErrSlowDown
		}
	}
}

// release returns the reservation waitForTurn made for a part of size bytes
// to the reorder buffer budget and wakes any parts waiting for room.
func (up *multipartUpload) release(size int64) {
	up.mu.Lock()
	up.unbuffer(bufferCharge(size))
	up.mu.Unlock()
}

// streamDirect streams body, the size bytes of partNumber, straight into
// the sink, returning its MD5 sum. partNumber must be the next part the
// stream needs and the caller must own the sink (up.pumping), which is
// passed on to pump any buffered parts which follow, or given up.
//
// If the body fails part way, what reached the sink can't be taken back, so
// the bytes streamed and the MD5 state after them are recorded instead. The
// next attempt at the part - typically the client's retry - must start with
// the same bytes, which are only hashed to check them, and then carries on
// from where the failed one stopped.
func (up *multipartUpload) streamDirect(partNumber int, size int64, body io.Reader) (md5Sum []byte, err error) {
	up.mu.Lock()
	resumeAt, resumeState := up.partialSize, up.partialMD5
	up.mu.Unlock()

	hasher := md5.New()
	if resumeAt > size {
		return nil, up.stopStreaming(errPartAlreadyStreamed(partNumber))
	}
	if resumeAt > 0 {
		if _, err := io.CopyN(hasher, body, resumeAt); err != nil {
			if err == io.EOF {
				err = gofakes3.ErrIncompleteBody
			}
			return nil, up.stopStreaming(err)
		}
		if state, _ := hasher.(encoding.BinaryMarshaler).MarshalBinary(); !bytes.Equal(state, resumeState) {
			return nil, up.stopStreaming(errPartAlreadyStreamed(partNumber))
		}
	}

	fh, err := up.sink()
	if err != nil {
		return nil, up.stopStreaming(err)
	}
	n, sinkErr, bodyErr := copyToSink(fh, hasher, body)
	if n > 0 {
		state, _ := hasher.(encoding.BinaryMarshaler).MarshalBinary()
		up.mu.Lock()
		up.partialSize, up.partialMD5 = resumeAt+n, state
		up.mu.Unlock()
	}
	if sinkErr != nil {
		return nil, up.stopStreaming(sinkErr)
	}
	if bodyErr != nil {
		if resumeAt+n == size {
			// The whole part reached the sink but the client disowned it
			// (for example it failed its Content-MD5 check), so the stream
			// holds data which can neither be completed nor replaced.
			return nil, up.poison(bodyErr)
		}
		return nil, up.stopStreaming(bodyErr)
	}
	if resumeAt+n != size {
		return nil, up.stopStreaming(gofakes3.ErrIncompleteBody)
	}

	md5Sum = hasher.Sum(nil)
	up.mu.Lock()
	if up.closed {
		up.pumping = false
		up.mu.Unlock()
		return nil, gofakes3.ErrNoSuchUpload
	}
	up.partMD5s[partNumber] = md5Sum
	up.partSizes[partNumber] = size
	up.partialSize, up.partialMD5 = 0, nil
	up.nextPart++
	up.broadcast()
	return md5Sum, up.pump()
}

// poison marks the upload as one which can never be completed, because the
// stream holds data which can't be taken back, then gives up the sink as
// stopStreaming does.
func (up *multipartUpload) poison(err error) error {
	up.mu.Lock()
	up.poisoned = true
	up.mu.Unlock()
	return up.stopStreaming(err)
}

// isPoisoned reports whether the upload can never be completed.
func (up *multipartUpload) isPoisoned() bool {
	up.mu.Lock()
	defer up.mu.Unlock()
	return up.poisoned
}

// stopStreaming gives up the sink after a part failed with err, returning
// the error to report: ErrNoSuchUpload if the failure was because the
// upload was aborted.
func (up *multipartUpload) stopStreaming(err error) error {
	up.mu.Lock()
	defer up.mu.Unlock()
	up.pumping = false
	up.broadcast()
	if up.closed {
		// The write failed because the upload was aborted while this part
		// was being written: report the upload gone rather than the sink's
		// ECLOSED as an internal error.
		return gofakes3.ErrNoSuchUpload
	}
	return err
}

// sink returns the handle the parts are written to, creating it on first
// use. Call with the sink owned (up.pumping).
func (up *multipartUpload) sink() (io.WriteCloser, error) {
	up.mu.Lock()
	defer up.mu.Unlock()
	if up.closed {
		return nil, gofakes3.ErrNoSuchUpload
	}
	if up.fh == nil {
		fh, err := up.vfs.Create(up.streamFp)
		if err != nil {
			return nil, err
		}
		up.fh = fh
	}
	return up.fh, nil
}

// copyToSink copies body into sink and hasher, returning the number of
// bytes written to the sink, which are also the bytes hashed, and the error
// which stopped it, from either the sink or the body.
func copyToSink(sink io.Writer, hasher hash.Hash, body io.Reader) (n int64, sinkErr, bodyErr error) {
	buf := make([]byte, 32*1024)
	for {
		nr, err := body.Read(buf)
		if nr > 0 {
			nw, err := sink.Write(buf[:nr])
			_, _ = hasher.Write(buf[:nw])
			n += int64(nw)
			if err == nil && nw != nr {
				err = io.ErrShortWrite
			}
			if err != nil {
				return n, err, nil
			}
		}
		if err == io.EOF {
			return n, nil, nil
		}
		if err != nil {
			return n, nil, err
		}
	}
}

// bufferPart records a part received ahead of its turn, holding it in rw
// until the stream reaches it, then streams the parts into the sink in
// order.
//
// Parts must be uploaded in ascending, contiguous part-number order. Whichever
// goroutine finds the next part available does the pumping, so concurrent
// (but in-order) clients are tolerated, with the buffering bounded by
// waitForTurn. If the stream has reached the part while it was being
// received, it is streamed now, once any other request writing to the sink
// has finished.
//
// A part number may be uploaded more than once - typically a client retrying
// after its request timed out, but real S3 also allows replacing a part. If
// the earlier copy is still buffered it is replaced (last write wins). If it
// has already been streamed it can't be replaced: an identical re-upload is
// accepted idempotently (the data is already in the stream) and a different
// one is rejected.
func (up *multipartUpload) bufferPart(ctx context.Context, partNumber int, size int64, md5Sum []byte, rw *pool.RW) error {
	charge := bufferCharge(size)
	// done releases the part's buffer. Call with up.mu held.
	done := func() {
		up.unbuffer(charge)
		_ = rw.Close()
	}
	timer := time.NewTimer(multipartWaitTimeout)
	defer timer.Stop()
	up.mu.Lock()
	for partNumber == up.nextPart && up.pumping && !up.closed {
		// Another request is writing this part to the sink: wait to see
		// whether it gets there.
		changed := up.changed
		up.mu.Unlock()
		var err error
		select {
		case <-changed:
		case <-ctx.Done():
			err = ctx.Err()
		case <-timer.C:
			err = gofakes3.ErrSlowDown
		}
		up.mu.Lock()
		if err != nil {
			done()
			up.mu.Unlock()
			return err
		}
	}
	switch {
	case up.closed:
		// The upload was aborted or completed while the part body was
		// being received.
		done()
		up.mu.Unlock()
		return gofakes3.ErrNoSuchUpload
	case partNumber < up.nextPart:
		// Already streamed: the stream can't be rewritten, so accept an
		// identical part and reject the rest.
		same := bytes.Equal(md5Sum, up.partMD5s[partNumber]) && size == up.partSizes[partNumber]
		done()
		up.mu.Unlock()
		if !same {
			return errPartAlreadyStreamed(partNumber)
		}
		return nil
	case partNumber == up.nextPart:
		// The stream has reached this part while it was being received.
		up.pumping = true
		up.mu.Unlock()
		_, err := up.streamDirect(partNumber, size, rw)
		up.mu.Lock()
		done()
		up.mu.Unlock()
		return err
	}
	if old, buffered := up.streamBuf[partNumber]; buffered {
		_ = old.Close()
		up.unbuffer(bufferCharge(up.partSizes[partNumber]))
	}
	up.partMD5s[partNumber] = md5Sum
	up.partSizes[partNumber] = size
	up.streamBuf[partNumber] = rw
	up.mu.Unlock()
	return nil
}

// pump streams buffered parts into the sink in order until it reaches one
// which hasn't arrived, then gives up the sink. Call with up.mu held and
// the sink owned (up.pumping); returns with up.mu released.
func (up *multipartUpload) pump() error {
	for {
		if up.closed {
			// Aborted while pumping: the sink is closed and any
			// parts still buffered were released by the abort, so
			// report the upload gone rather than success.
			up.pumping = false
			up.mu.Unlock()
			return gofakes3.ErrNoSuchUpload
		}
		prw, ok := up.streamBuf[up.nextPart]
		if !ok {
			up.pumping = false
			up.broadcast()
			up.mu.Unlock()
			return nil
		}
		delete(up.streamBuf, up.nextPart)
		psize := bufferCharge(prw.Size())
		up.mu.Unlock()

		fh, err := up.sink()
		if err == nil {
			err = pipePart(fh, prw)
		}
		_ = prw.Close()
		if err != nil {
			up.mu.Lock()
			up.unbuffer(psize)
			// The client was told this part succeeded so won't resend it,
			// and the sink may hold some of it, so the upload can never
			// complete.
			up.poisoned = true
			up.mu.Unlock()
			return up.stopStreaming(err)
		}

		up.mu.Lock()
		up.nextPart++
		up.unbuffer(psize)
	}
}

// pipePart writes the whole of rw into w (the sink).
func pipePart(w io.Writer, rw *pool.RW) error {
	if _, err := rw.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(w, rw)
	return err
}

// CompleteMultipartUpload finalises a multipart upload. It closes the sink,
// committing the upload, renames the temporary object into place, computes the
// S3-style multipart ETag, and stores the user metadata so HeadObject and
// GetObject see the same fields the in-memory PutObject path produces.
func (b *s3Backend) CompleteMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, input *gofakes3.CompleteMultipartUploadRequest) (gofakes3.VersionID, string, error) {
	up, err := b.loadUpload(ctx, uploadID)
	if err != nil {
		return "", "", err
	}
	if err := up.startActivity(); err != nil {
		return "", "", err
	}
	defer up.endActivity()

	// gofakes3 keeps its record of an upload whose completion fails, so
	// forgetUpload removes that too, as the upload can't be retried.
	if err := up.validate(input); err != nil {
		b.forgetUpload(uploadID, up)
		return "", "", err
	}

	// close commits the upload, failing with the upload left open if the
	// streamed parts don't form the complete object; forgetUpload then
	// tears it down. (After a successful or failed commit the abort it
	// does is a no-op.)
	if err := up.close(); err != nil {
		b.forgetUpload(uploadID, up)
		return "", "", err
	}

	// Rename the temporary object into place: on a caching VFS the
	// write-back then uploads it under the final name, otherwise it is
	// moved server-side on the remote. On failure the upload record is
	// kept, because gofakes3 keeps its own record when the backend errors
	// so that the client can retry the CompleteMultipartUpload: the
	// committed close is idempotent, so the retry just renames again.
	if up.streamFp != up.fp {
		if err := up.vfs.Rename(up.streamFp, up.fp); err != nil {
			return "", "", err
		}
	}
	b.deleteUpload(uploadID, up)

	_ = up.tenant.storeMeta(up.vfs, up.fp, up.meta)

	return "", up.multipartETag(input), nil
}

// AbortMultipartUpload tears down an in-progress upload, discarding any data
// already received.
func (b *s3Backend) AbortMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID) error {
	up, err := b.loadUpload(ctx, uploadID)
	if err != nil {
		return err
	}
	if err := up.startActivity(); err != nil {
		return err
	}
	defer up.endActivity()
	defer b.deleteUpload(uploadID, up)
	if err := up.abort(); err != nil {
		fs.Errorf(up.fp, "aborting multipart upload: %v", err)
	}
	b.discardUpload(up)
	return nil
}

// failUpload tears down an upload which can never be completed, so that
// the client starts again instead of retrying a part which can't be
// accepted.
func (b *s3Backend) failUpload(uploadID gofakes3.UploadID, up *multipartUpload) {
	fs.Errorf(up.fp, "failing multipart upload %s: %v", uploadID, errMultipartPoisoned)
	b.forgetUpload(uploadID, up)
}

// forgetUpload aborts up, discards what it wrote and removes every record
// of it, ours and gofakes3's.
func (b *s3Backend) forgetUpload(uploadID gofakes3.UploadID, up *multipartUpload) {
	if up.startActivity() != nil {
		return // already gone
	}
	defer up.endActivity()
	b.deleteUpload(uploadID, up)
	if err := up.abort(); err != nil {
		fs.Errorf(up.fp, "aborting multipart upload: %v", err)
	}
	b.discardUpload(up)
	// gofakes3 doesn't know the upload has gone, so tell it
	_ = b.s.faker.ForgetMultipartUpload(up.tenant.context(), up.bucket, up.key, uploadID)
}

// startReaper starts a goroutine which aborts incomplete multipart
// uploads once they have been idle for expiry. Stopped by stopReaper.
func (b *s3Backend) startReaper(expiry time.Duration) {
	interval := min(expiry/2, time.Minute)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-b.reaperQuit:
				return
			case <-ticker.C:
				b.reapExpiredUploads(time.Now(), expiry)
			}
		}
	}()
}

// stopReaper stops the abandoned upload reaper, if running.
func (b *s3Backend) stopReaper() {
	b.reaperStop.Do(func() {
		close(b.reaperQuit)
	})
}

// reapExpiredUploads aborts and cleans up multipart uploads which have had
// no request activity for longer than expiry.
func (b *s3Backend) reapExpiredUploads(now time.Time, expiry time.Duration) {
	for _, t := range b.allTenants() {
		b.reapExpiredTenantUploads(t, now, expiry)
	}
}

// reapExpiredTenantUploads aborts and cleans up the multipart uploads of
// t which have had no request activity for longer than expiry.
func (b *s3Backend) reapExpiredTenantUploads(t *tenant, now time.Time, expiry time.Duration) {
	t.uploads.Range(func(key, value any) bool {
		uploadID := key.(gofakes3.UploadID)
		up := value.(*multipartUpload)
		up.mu.Lock()
		expired := up.active == 0 && now.Sub(up.lastUsed) >= expiry
		up.mu.Unlock()
		if !expired {
			return true
		}
		fs.Logf(up.fp, "aborting multipart upload %s idle for more than %v", uploadID, expiry)
		b.forgetUpload(uploadID, up)
		return true
	})
}

// forgetAllUploads aborts every in-flight upload, as forgetUpload does.
func (b *s3Backend) forgetAllUploads() {
	for _, t := range b.allTenants() {
		t.uploads.Range(func(key, value any) bool {
			b.forgetUpload(key.(gofakes3.UploadID), value.(*multipartUpload))
			return true
		})
	}
}

// discardUpload cleans up after a failed or aborted upload, removing the
// temporary object and any stale VFS state for it. It never removes the
// object at the final path: when the parts were written straight to the
// final object (a remote with no server-side move or copy) it may hold a
// pre-existing object the abandoned streaming write never disturbed, so
// only the VFS's view of the path is refreshed. (A caching VFS on such a
// remote commits the partial data instead - see abort.)
func (b *s3Backend) discardUpload(up *multipartUpload) {
	b.forgetPath(up.vfs, up.streamFp)
	if up.streamFp == up.fp {
		return
	}
	_ = up.vfs.Remove(up.streamFp)
}

// forgetPath invalidates the parent directory's cached VFS listing so that
// subsequent VFS Stat / List calls re-read fp from the underlying Fs.
func (b *s3Backend) forgetPath(_vfs *vfs.VFS, fp string) {
	if root, err := _vfs.Root(); err == nil {
		root.ForgetPath(fp, fs.EntryObject)
	}
}

// validate cross-checks the part list supplied by the client against the
// parts we actually received.
func (up *multipartUpload) validate(input *gofakes3.CompleteMultipartUploadRequest) error {
	up.mu.Lock()
	defer up.mu.Unlock()

	for i := 1; i < len(input.Parts); i++ {
		if input.Parts[i].PartNumber <= input.Parts[i-1].PartNumber {
			return gofakes3.ErrInvalidPartOrder
		}
	}
	if len(input.Parts) != len(up.partSizes) {
		return gofakes3.ErrInvalidPart
	}
	for _, p := range input.Parts {
		md5Sum, ok := up.partMD5s[p.PartNumber]
		if !ok {
			return gofakes3.ErrInvalidPart
		}
		clientETag := strings.Trim(p.ETag, `"`)
		if clientETag != hex.EncodeToString(md5Sum) {
			return gofakes3.ErrInvalidPart
		}
	}
	return nil
}

// close finalises the upload, committing it: closing the sink completes the
// streaming upload to the remote, or the write to the VFS cache whose
// write-back then uploads it.
//
// The upload is only committed if the streamed parts form the complete
// object: contiguous part numbers from 1 with nothing left buffered (a
// leftover means the client used non-contiguous part numbers, which the
// in-order stream can't place). Otherwise ErrInvalidPart is returned and
// the upload is left open. The check and the commit share one critical
// section so no late part can slip into the stream between them.
//
// Returns ErrNoSuchUpload if the upload was aborted by a concurrent
// AbortMultipartUpload, in which case nothing has been committed - the
// upload must not be reported as complete.
func (up *multipartUpload) close() error {
	up.mu.Lock()
	if up.closed {
		aborted := up.aborted
		up.mu.Unlock()
		if aborted {
			return gofakes3.ErrNoSuchUpload
		}
		return nil
	}
	if len(up.streamBuf) != 0 || up.nextPart-1 != len(up.partSizes) || up.pumping || up.partialSize != 0 {
		up.mu.Unlock()
		return gofakes3.ErrInvalidPart
	}
	if up.fh == nil {
		// Nothing was streamed so commit an empty object
		fh, err := up.vfs.Create(up.streamFp)
		if err != nil {
			up.mu.Unlock()
			return err
		}
		up.fh = fh
	}
	fh := up.fh
	up.closed = true
	up.broadcast()
	up.mu.Unlock()

	return fh.Close()
}

// errMultipartPoisoned is why an upload whose stream holds data it can
// never complete with is failed.
var errMultipartPoisoned = errors.New("a part was rejected after it had been streamed to the backend, so the upload must be started again")

// errMultipartAborted is the reason an aborted upload's write is abandoned
// with, so the streaming upload fails instead of committing what it has.
var errMultipartAborted = errors.New("serve s3: multipart upload aborted")

// abort tears down the upload and releases any buffered parts. The write is
// abandoned so nothing is committed; a sink which can't abandon (a caching
// one) is closed normally, committing what it has to the cache - the caller
// then removes its temporary file, except on a remote with no server-side
// move or copy, where the parts went straight to the final path and the
// partial data is left to be written back.
func (up *multipartUpload) abort() error {
	up.mu.Lock()
	if up.closed {
		up.mu.Unlock()
		return nil
	}
	up.closed = true
	up.aborted = true
	streamBuf := up.streamBuf
	up.streamBuf = nil
	for _, rw := range streamBuf {
		up.unbuffer(bufferCharge(rw.Size()))
	}
	fh := up.fh
	up.broadcast()
	up.mu.Unlock()

	for _, rw := range streamBuf {
		_ = rw.Close()
	}

	if fh == nil {
		// Nothing was streamed so there is nothing to abandon
		return nil
	}
	if aborter, ok := fh.(interface{ CloseWithError(error) error }); ok {
		_ = aborter.CloseWithError(errMultipartAborted)
		return nil
	}
	return fh.Close()
}

// multipartETag computes the S3 multipart ETag for the assembled object:
//
//	hex(md5(concat(part_md5s_in_order))) + "-" + N
func (up *multipartUpload) multipartETag(input *gofakes3.CompleteMultipartUploadRequest) string {
	partNumbers := make([]int, 0, len(input.Parts))
	for _, p := range input.Parts {
		partNumbers = append(partNumbers, p.PartNumber)
	}
	sort.Ints(partNumbers)

	up.mu.Lock()
	concat := make([]byte, 0, len(partNumbers)*md5.Size)
	for _, n := range partNumbers {
		concat = append(concat, up.partMD5s[n]...)
	}
	up.mu.Unlock()

	sum := md5.Sum(concat)
	return fmt.Sprintf("%q", fmt.Sprintf("%s-%d", hex.EncodeToString(sum[:]), len(partNumbers)))
}
