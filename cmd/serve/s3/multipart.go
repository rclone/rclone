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
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ncw/swift/v2"
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

// multipartUpload tracks one in-flight S3 multipart upload. The parts are
// written, in part-number order, into fh - a VFS file handle which either
// streams straight through to the remote (the default) or is backed by the
// VFS cache (when the VFS is caching writes).
type multipartUpload struct {
	bucket, key string
	fp          string // final object path
	streamFp    string // path the parts are written to (fp when the remote has no server-side move or copy)
	meta        map[string]string

	fh  io.WriteCloser // sink the in-order parts are written to
	vfs *vfs.VFS       // the VFS fh was created on, used for all later operations

	mu        sync.Mutex
	cond      *sync.Cond     // signalled when buffered shrinks, nextPart advances or the upload closes
	partMD5s  map[int][]byte // raw MD5 sums per part (for the final S3 multipart ETag)
	partSizes map[int]int64  // observed part sizes
	closed    bool
	aborted   bool      // closed by abort, so nothing was committed
	active    int       // requests in flight for this upload, protecting it from the reaper
	lastUsed  time.Time // when the last request for this upload finished

	nextPart    int              // next part number to stream (1-based)
	streamBuf   map[int]*pool.RW // parts received ahead of nextPart, awaiting their turn
	pumping     bool             // a goroutine is currently writing to the sink
	buffered    int64            // bytes of parts admitted but not yet streamed or released
	bufferLimit int64            // max buffered before parts ahead of nextPart must wait (<= 0 for no limit)
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
	}
	up.cond = sync.NewCond(&up.mu)
	return up
}

// startActivity marks the upload as having a request in flight.
func (up *multipartUpload) startActivity() {
	up.mu.Lock()
	up.active++
	up.mu.Unlock()
}

// endActivity marks the request done and restarts the upload's idle time.
func (up *multipartUpload) endActivity() {
	up.mu.Lock()
	up.active--
	up.lastUsed = time.Now()
	up.mu.Unlock()
}

// loadUpload looks up an in-flight upload by ID.
func (b *s3Backend) loadUpload(uploadID gofakes3.UploadID) (*multipartUpload, error) {
	v, ok := b.multipartUploads.Load(uploadID)
	if !ok {
		return nil, gofakes3.ErrNoSuchUpload
	}
	return v.(*multipartUpload), nil
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
	objectDir := path.Dir(fp)
	if objectDir != "." {
		if err := mkdirRecursive(objectDir, _vfs); err != nil {
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
	fh, err := _vfs.Create(streamFp)
	if err != nil {
		return "", err
	}
	up.fh = fh
	up.vfs = _vfs

	b.multipartUploads.Store(uploadID, up)
	return uploadID, nil
}

// UploadPart writes a single part from the S3 client into the streaming upload.
func (b *s3Backend) UploadPart(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, partNumber int, contentLength int64, body io.Reader) (string, error) {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return "", err
	}
	up.startActivity()
	defer up.endActivity()

	// Wait until there is room to buffer this part, bounding the memory a
	// client which uploads faster than the backend drains can consume.
	if err := up.waitForTurn(partNumber, contentLength); err != nil {
		return "", err
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

	if err := up.streamPart(partNumber, n, md5Sum, rw); err != nil {
		return "", err
	}
	return etag, nil
}

// waitForTurn blocks until size bytes can be admitted to the reorder buffer,
// then reserves them, bounding the memory an upload can consume when the
// client sends parts faster than the backend drains them.
//
// The next part the stream needs (and any retry of an earlier one) is always
// admitted so the sink can keep draining; so is a single part bigger than the
// limit when the buffer is empty, to guarantee progress. Reserved bytes are
// returned with release, or by the pump as the part is streamed.
//
// size is the client-declared part length and is not trusted: a negative value
// is rejected, and the admission test is written so a huge value can't overflow
// the running total and wrongly admit further parts past the limit.
func (up *multipartUpload) waitForTurn(partNumber int, size int64) error {
	if size < 0 {
		return gofakes3.ErrInvalidArgument
	}
	up.mu.Lock()
	defer up.mu.Unlock()
	for {
		if up.closed {
			return gofakes3.ErrNoSuchUpload
		}
		if up.bufferLimit <= 0 || partNumber <= up.nextPart || up.buffered == 0 || size <= up.bufferLimit-up.buffered {
			up.buffered += size
			return nil
		}
		up.cond.Wait()
	}
}

// release returns size bytes reserved by waitForTurn to the reorder buffer
// budget and wakes any parts waiting for room.
func (up *multipartUpload) release(size int64) {
	up.mu.Lock()
	up.buffered -= size
	up.cond.Broadcast()
	up.mu.Unlock()
}

// streamPart records a part and streams the parts into the sink in order.
//
// Parts must be uploaded in ascending, contiguous part-number order. A part
// that arrives ahead of the next expected one is buffered until its turn; the
// parts are then pumped into the sink in order. Whichever goroutine finds the
// next part available does the pumping, so concurrent (but in-order) clients
// are tolerated, with the buffering bounded by waitForTurn.
//
// A part number may be uploaded more than once - typically a client retrying
// after its request timed out, but real S3 also allows replacing a part. If
// the earlier copy is still buffered it is replaced (last write wins). If it
// has already been streamed it can't be replaced: an identical re-upload is
// accepted idempotently (the data is already in the stream) and a different
// one is rejected.
func (up *multipartUpload) streamPart(partNumber int, size int64, md5Sum []byte, rw *pool.RW) error {
	up.mu.Lock()
	if up.closed {
		// The upload was aborted or completed while the part body was
		// being received.
		up.buffered -= size
		up.cond.Broadcast()
		up.mu.Unlock()
		_ = rw.Close()
		return gofakes3.ErrNoSuchUpload
	}
	if oldMD5, exists := up.partMD5s[partNumber]; exists {
		if old, buffered := up.streamBuf[partNumber]; buffered {
			_ = old.Close()
			up.buffered -= up.partSizes[partNumber]
			up.cond.Broadcast()
		} else {
			// Already streamed (or streaming right now): the stream can't be
			// rewritten, so accept an identical part and reject the rest.
			same := bytes.Equal(md5Sum, oldMD5) && size == up.partSizes[partNumber]
			up.buffered -= size
			up.cond.Broadcast()
			up.mu.Unlock()
			_ = rw.Close()
			if !same {
				return gofakes3.ErrorMessagef(gofakes3.ErrNotImplemented, "part %d has already been streamed to the backend and cannot be replaced with different contents", partNumber)
			}
			return nil
		}
	}
	up.partMD5s[partNumber] = md5Sum
	up.partSizes[partNumber] = size
	up.streamBuf[partNumber] = rw

	if up.pumping {
		// Another goroutine owns the sink and will pump this part in turn.
		up.mu.Unlock()
		return nil
	}
	up.pumping = true

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
			up.mu.Unlock()
			return nil
		}
		delete(up.streamBuf, up.nextPart)
		psize := prw.Size()
		up.mu.Unlock()

		err := pipePart(up.fh, prw)
		_ = prw.Close()
		if err != nil {
			up.mu.Lock()
			up.pumping = false
			up.buffered -= psize
			closed := up.closed
			up.cond.Broadcast()
			up.mu.Unlock()
			if closed {
				// The write failed because the upload was aborted while
				// this part was being pumped: report the upload gone
				// rather than the sink's ECLOSED as an internal error.
				return gofakes3.ErrNoSuchUpload
			}
			return err
		}

		up.mu.Lock()
		up.nextPart++
		up.buffered -= psize
		up.cond.Broadcast()
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
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return "", "", err
	}
	up.startActivity()
	defer up.endActivity()

	if err := up.validate(input); err != nil {
		b.multipartUploads.Delete(uploadID)
		_ = up.abort()
		b.discardUpload(up)
		return "", "", err
	}

	// close commits the upload, failing with the upload left open if the
	// streamed parts don't form the complete object; abort then tears it
	// down. (After a successful or failed commit the abort is a no-op.)
	if err := up.close(); err != nil {
		b.multipartUploads.Delete(uploadID)
		_ = up.abort()
		b.discardUpload(up)
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
	b.multipartUploads.Delete(uploadID)

	b.meta.Store(up.fp, up.meta)
	if val, ok := up.meta["X-Amz-Meta-Mtime"]; ok {
		if ti, err := swift.FloatStringToTime(val); err == nil {
			b.storeModtime(up.fp, up.meta, val)
			_ = up.vfs.Chtimes(up.fp, ti, ti)
		}
	} else if val, ok := up.meta["mtime"]; ok {
		if ti, err := swift.FloatStringToTime(val); err == nil {
			b.storeModtime(up.fp, up.meta, val)
			_ = up.vfs.Chtimes(up.fp, ti, ti)
		}
	}

	return "", up.multipartETag(input), nil
}

// AbortMultipartUpload tears down an in-progress upload, discarding any data
// already received.
func (b *s3Backend) AbortMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID) error {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return err
	}
	defer b.multipartUploads.Delete(uploadID)
	if err := up.abort(); err != nil {
		fs.Errorf(up.fp, "aborting multipart upload: %v", err)
	}
	b.discardUpload(up)
	return nil
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
	b.multipartUploads.Range(func(key, value any) bool {
		uploadID := key.(gofakes3.UploadID)
		up := value.(*multipartUpload)
		up.mu.Lock()
		expired := up.active == 0 && now.Sub(up.lastUsed) >= expiry
		up.mu.Unlock()
		if !expired {
			return true
		}
		fs.Logf(up.fp, "aborting multipart upload %s idle for more than %v", uploadID, expiry)
		b.multipartUploads.Delete(uploadID)
		if err := up.abort(); err != nil {
			fs.Errorf(up.fp, "aborting abandoned multipart upload: %v", err)
		}
		b.discardUpload(up)
		return true
	})
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
	if len(up.streamBuf) != 0 || up.nextPart-1 != len(up.partSizes) {
		up.mu.Unlock()
		return gofakes3.ErrInvalidPart
	}
	up.closed = true
	up.cond.Broadcast()
	up.mu.Unlock()

	return up.fh.Close()
}

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
	up.cond.Broadcast()
	up.mu.Unlock()

	for _, rw := range streamBuf {
		_ = rw.Close()
	}

	if aborter, ok := up.fh.(interface{ CloseWithError(error) error }); ok {
		_ = aborter.CloseWithError(errMultipartAborted)
		return nil
	}
	return up.fh.Close()
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
