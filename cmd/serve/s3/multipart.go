// Multipart upload support for serve s3.
//
// Multipart uploads received by serve s3 are streamed, in part-number order,
// into a single PutStream upload to the underlying Fs, so the whole file is
// never buffered in memory. This implements the gofakes3.MultipartBackend
// interface on s3Backend.
//
// When streaming is disabled (--disable-multipart-streaming) or the Fs has no
// PutStream, ErrMultipartUploadNotSupported is returned so that gofakes3 falls
// back to buffering the parts in memory.

package s3

import (
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
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pool"
)

// multipartUpload tracks one in-flight S3 multipart upload that is being
// streamed, in part order, into a single PutStream upload to the underlying Fs.
type multipartUpload struct {
	bucket, key string
	fp          string // = path.Join(bucket, key)
	meta        map[string]string

	pipeW *io.PipeWriter // parts are streamed here, in part-number order

	mu        sync.Mutex
	partMD5s  map[int][]byte // raw MD5 sums per part (for the final S3 multipart ETag)
	partSizes map[int]int64  // observed part sizes
	closed    bool

	putCancel context.CancelFunc // cancels the background PutStream
	putDone   chan struct{}      // closed when the background PutStream returns
	putErr    error              // PutStream result (read only after putDone is closed)
	nextPart  int                // next part number to stream (1-based)
	streamBuf map[int]*pool.RW   // parts received ahead of nextPart, awaiting their turn
	pumping   bool               // a goroutine is currently writing to the pipe
}

// newMultipartUpload allocates an upload struct.
func newMultipartUpload(bucket, key, fp string, meta map[string]string) *multipartUpload {
	return &multipartUpload{
		bucket:    bucket,
		key:       key,
		fp:        fp,
		meta:      meta,
		partMD5s:  map[int][]byte{},
		partSizes: map[int]int64{},
		nextPart:  1,
		streamBuf: map[int]*pool.RW{},
	}
}

// loadUpload looks up an in-flight upload by ID.
func (b *s3Backend) loadUpload(uploadID gofakes3.UploadID) (*multipartUpload, error) {
	v, ok := b.multipartUploads.Load(uploadID)
	if !ok {
		return nil, gofakes3.ErrNoSuchUpload
	}
	return v.(*multipartUpload), nil
}

// CreateMultipartUpload begins a new multipart upload that streams the parts,
// in part-number order, into a single PutStream upload to the underlying Fs.
//
// If streaming is disabled (--disable-multipart-streaming) or the Fs has no
// PutStream, ErrMultipartUploadNotSupported is returned so that gofakes3 falls
// back to buffering the whole upload in memory; a one-off NOTICE warns about
// the memory use.
func (b *s3Backend) CreateMultipartUpload(ctx context.Context, bucketName, objectName string, meta map[string]string) (gofakes3.UploadID, error) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return "", err
	}
	if _, err := _vfs.Stat(bucketName); err != nil {
		return "", gofakes3.BucketNotFound(bucketName)
	}

	f := _vfs.Fs()
	features := f.Features()
	if b.s.opt.DisableMultipartStreaming || features.PutStream == nil {
		b.warnInMemoryOnce.Do(func() {
			reason := "this backend doesn't support streaming uploads"
			if b.s.opt.DisableMultipartStreaming {
				reason = "--disable-multipart-streaming is set"
			}
			fs.Logf(nil, "serve s3: buffering multipart uploads in memory because %s - this may use a lot of memory", reason)
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

	up := newMultipartUpload(bucketName, objectName, fp, meta)

	src := object.NewStaticObjectInfo(fp, time.Now(), -1, true, nil, f)
	pr, pw := io.Pipe()
	// Use a context that outlives this request (it's cancelled on abort) but
	// keeps its values.
	putCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	up.pipeW = pw
	up.putCancel = cancel
	up.putDone = make(chan struct{})
	go func() {
		_, err := features.PutStream(putCtx, pr, src)
		up.putErr = err
		_ = pr.CloseWithError(err)
		close(up.putDone)
	}()

	uploadID := gofakes3.UploadID(uuid.New().String())
	b.multipartUploads.Store(uploadID, up)
	return uploadID, nil
}

// UploadPart writes a single part from the S3 client into the streaming upload.
func (b *s3Backend) UploadPart(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, partNumber int, contentLength int64, body io.Reader) (string, error) {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return "", err
	}

	// Buffer the part in a pool-backed RW so we can MD5 it (for the ETag) and
	// stream it once it is this part's turn.
	rw := multipart.NewRW().Reserve(contentLength)
	hasher := md5.New()
	n, err := io.Copy(rw, io.TeeReader(body, hasher))
	if err != nil {
		_ = rw.Close()
		return "", err
	}
	if n != contentLength {
		_ = rw.Close()
		return "", gofakes3.ErrIncompleteBody
	}
	md5Sum := hasher.Sum(nil)
	etag := fmt.Sprintf("%q", hex.EncodeToString(md5Sum))

	if err := up.streamPart(partNumber, n, md5Sum, rw); err != nil {
		return "", err
	}
	return etag, nil
}

// streamPart records a part and streams the parts into the pipe in order.
//
// Parts must be uploaded in ascending, contiguous part-number order. A part
// that arrives ahead of the next expected one is buffered until its turn; the
// parts are then pumped into the pipe in order. Whichever goroutine finds the
// next part available does the pumping, so concurrent (but in-order) clients
// are tolerated with buffering bounded by how far ahead they run.
func (up *multipartUpload) streamPart(partNumber int, size int64, md5Sum []byte, rw *pool.RW) error {
	up.mu.Lock()
	up.partMD5s[partNumber] = md5Sum
	up.partSizes[partNumber] = size
	up.streamBuf[partNumber] = rw

	if up.pumping {
		// Another goroutine owns the pipe and will pump this part in turn.
		up.mu.Unlock()
		return nil
	}
	up.pumping = true

	for {
		prw, ok := up.streamBuf[up.nextPart]
		if !ok {
			up.pumping = false
			up.mu.Unlock()
			return nil
		}
		delete(up.streamBuf, up.nextPart)
		up.mu.Unlock()

		err := pipePart(up.pipeW, prw)
		_ = prw.Close()
		if err != nil {
			up.mu.Lock()
			up.pumping = false
			up.mu.Unlock()
			return err
		}

		up.mu.Lock()
		up.nextPart++
	}
}

// pipePart writes the whole of rw into w (the pipe).
func pipePart(w io.Writer, rw *pool.RW) error {
	if _, err := rw.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := io.Copy(w, rw)
	return err
}

// CompleteMultipartUpload finalises a streamed multipart upload. It closes the
// pipe (so PutStream finishes), registers the new file with the VFS, computes
// the S3-style multipart ETag, and stores the user metadata so HeadObject and
// GetObject see the same fields the in-memory PutObject path produces.
func (b *s3Backend) CompleteMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, input *gofakes3.CompleteMultipartUploadRequest) (gofakes3.VersionID, string, error) {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return "", "", err
	}
	defer b.multipartUploads.Delete(uploadID)

	if err := up.validate(input); err != nil {
		_ = up.abort(ctx)
		b.forgetPath(ctx, up.fp)
		return "", "", err
	}

	// All parts must have been streamed: contiguous part numbers from 1 with
	// nothing left buffered. A leftover means the client used non-contiguous
	// part numbers, which the in-order stream can't place.
	up.mu.Lock()
	streamed := up.nextPart - 1
	total := len(up.partSizes)
	leftover := len(up.streamBuf)
	up.mu.Unlock()
	if leftover != 0 || streamed != total {
		_ = up.abort(ctx)
		b.forgetPath(ctx, up.fp)
		return "", "", gofakes3.ErrInvalidPart
	}

	if err := up.close(ctx); err != nil {
		return "", "", err
	}

	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return "", "", err
	}

	b.forgetPath(ctx, up.fp)

	b.meta.Store(up.fp, up.meta)
	if val, ok := up.meta["X-Amz-Meta-Mtime"]; ok {
		if ti, err := swift.FloatStringToTime(val); err == nil {
			b.storeModtime(up.fp, up.meta, val)
			_ = _vfs.Chtimes(up.fp, ti, ti)
		}
	} else if val, ok := up.meta["mtime"]; ok {
		if ti, err := swift.FloatStringToTime(val); err == nil {
			b.storeModtime(up.fp, up.meta, val)
			_ = _vfs.Chtimes(up.fp, ti, ti)
		}
	}

	return "", up.multipartETag(input), nil
}

// AbortMultipartUpload tears down an in-progress upload, asking the background
// PutStream to discard any data already sent.
func (b *s3Backend) AbortMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID) error {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return err
	}
	defer b.multipartUploads.Delete(uploadID)
	err = up.abort(ctx)
	b.forgetPath(ctx, up.fp)
	return err
}

// forgetPath invalidates the parent directory's cached VFS listing so that
// subsequent VFS Stat / List calls re-read up.fp from the underlying Fs. The
// streamed multipart path writes to (and, on abort, removes from) the Fs
// directly, bypassing the VFS, so the VFS cache would otherwise keep serving a
// stale entry - including a ghost of a pre-existing object that an aborted
// upload has overwritten and removed.
func (b *s3Backend) forgetPath(ctx context.Context, fp string) {
	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return
	}
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

// close finalises the upload by signalling EOF to the background PutStream and
// waiting for it to finish.
func (up *multipartUpload) close(ctx context.Context) error {
	up.mu.Lock()
	if up.closed {
		up.mu.Unlock()
		return nil
	}
	up.closed = true
	up.mu.Unlock()

	err := up.pipeW.Close()
	<-up.putDone
	up.putCancel()
	if up.putErr != nil {
		return up.putErr
	}
	return err
}

// errMultipartAborted makes the background PutStream fail when an upload is
// aborted, so it tears down its partial object instead of completing.
var errMultipartAborted = errors.New("serve s3: multipart upload aborted")

// abort cancels the background PutStream and releases any buffered parts.
func (up *multipartUpload) abort(ctx context.Context) error {
	up.mu.Lock()
	if up.closed {
		up.mu.Unlock()
		return nil
	}
	up.closed = true
	streamBuf := up.streamBuf
	up.streamBuf = nil
	up.mu.Unlock()

	for _, rw := range streamBuf {
		_ = rw.Close()
	}

	// Fail the background PutStream (so it discards its partial object) and
	// wait for it to return.
	up.putCancel()
	_ = up.pipeW.CloseWithError(errMultipartAborted)
	<-up.putDone
	return nil
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
