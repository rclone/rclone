// Multipart upload support for serve s3.
//
// When the underlying Fs supports OpenChunkWriter or OpenWriterAt, multipart
// uploads received by serve s3 are streamed directly into the backend instead
// of being buffered in memory by gofakes3. This implements the
// gofakes3.MultipartBackend interface on s3Backend.

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
	"github.com/rclone/rclone/lib/ranges"
)

// multipartUpload tracks one in-flight S3 multipart upload that is being
// streamed straight into the underlying Fs.
type multipartUpload struct {
	bucket, key string
	fp          string // = path.Join(bucket, key)
	meta        map[string]string

	// Exactly one of these is non-nil for the lifetime of the upload.
	chunkWriter fs.ChunkWriter    // OpenChunkWriter path
	writerAt    fs.WriterAtCloser // OpenWriterAt path

	mu        sync.Mutex
	partMD5s  map[int][]byte // raw MD5 sums per part (for the final S3 multipart ETag)
	partSizes map[int]int64  // observed part sizes
	closed    bool

	// OpenWriterAt path only:
	partSize int64            // uniform part size; learned from part 1, 0 until then
	written  ranges.Ranges    // byte ranges that have been written via writerAt
	pending  map[int]*pool.RW // parts received before partSize was known
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
		pending:   map[int]*pool.RW{},
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

// CreateMultipartUpload begins a new multipart upload backed by the underlying
// Fs's chunked-write feature. If the Fs supports neither OpenChunkWriter nor
// OpenWriterAt, ErrMultipartUploadNotSupported is returned so that gofakes3
// falls back to its default in-memory implementation.
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
	if features.OpenChunkWriter == nil && features.OpenWriterAt == nil {
		return "", gofakes3.ErrMultipartUploadNotSupported
	}

	fp := path.Join(bucketName, objectName)
	objectDir := path.Dir(fp)
	if objectDir != "." {
		if err := mkdirRecursive(objectDir, _vfs); err != nil {
			return "", err
		}
	}

	up := newMultipartUpload(bucketName, objectName, fp, meta)

	if features.OpenChunkWriter != nil {
		src := object.NewStaticObjectInfo(fp, time.Now(), -1, true, nil, f)
		_, cw, err := features.OpenChunkWriter(ctx, fp, src)
		if err != nil {
			return "", fmt.Errorf("serve s3: open chunk writer: %w", err)
		}
		up.chunkWriter = cw
	} else {
		wa, err := features.OpenWriterAt(ctx, fp, -1)
		if err != nil {
			return "", fmt.Errorf("serve s3: open writer at: %w", err)
		}
		up.writerAt = wa
	}

	uploadID := gofakes3.UploadID(uuid.New().String())
	b.multipartUploads.Store(uploadID, up)
	return uploadID, nil
}

// UploadPart writes a single part from the S3 client to the underlying Fs.
func (b *s3Backend) UploadPart(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, partNumber int, contentLength int64, body io.Reader) (string, error) {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return "", err
	}

	// Buffer the part in a pool-backed RW. Pages return to the pool when
	// rw.Close() is called below (or by flushPending in the WriterAt path).
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

	switch {
	case up.chunkWriter != nil:
		// ChunkWriter is documented as safe under concurrent WriteChunk
		// calls. WriteChunk indexes from 0; S3 part numbers index from 1.
		if _, err := up.chunkWriter.WriteChunk(ctx, partNumber-1, rw); err != nil {
			_ = rw.Close()
			return "", err
		}
		// Done with the buffer.
		_ = rw.Close()

		up.mu.Lock()
		up.partMD5s[partNumber] = md5Sum
		up.partSizes[partNumber] = n
		up.mu.Unlock()
		return etag, nil

	case up.writerAt != nil:
		if err := up.writerAtPart(ctx, partNumber, n, md5Sum, rw); err != nil {
			_ = rw.Close()
			return "", err
		}
		return etag, nil
	}
	_ = rw.Close()
	return "", errors.New("serve s3: multipart upload has no writer")
}

// writerAtPart handles a single part on the OpenWriterAt path. It updates the
// upload state under up.mu and writes the buffer through the WriterAt as soon
// as the part's offset can be determined.
func (up *multipartUpload) writerAtPart(ctx context.Context, partNumber int, size int64, md5Sum []byte, rw *pool.RW) error {
	up.mu.Lock()
	up.partSizes[partNumber] = size
	up.partMD5s[partNumber] = md5Sum

	if partNumber == 1 {
		up.partSize = size
		pending := up.pending
		up.pending = map[int]*pool.RW{}
		up.mu.Unlock()

		if err := up.flushAt(0, size, rw); err != nil {
			return err
		}
		// Flush parts that were waiting for partSize to be known.
		for n, prw := range pending {
			offset := int64(n-1) * size
			psize := up.partSizes[n]
			err := up.flushAt(offset, psize, prw)
			_ = prw.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}

	if up.partSize == 0 {
		// Part 1 hasn't arrived yet; remember the buffer for later.
		up.pending[partNumber] = rw
		up.mu.Unlock()
		return nil
	}
	offset := int64(partNumber-1) * up.partSize
	up.mu.Unlock()

	if err := up.flushAt(offset, size, rw); err != nil {
		return err
	}
	_ = rw.Close()
	return nil
}

// flushAt copies rw into the upload's WriterAt at the given offset and
// records the byte range as written. It detects overlaps with previously
// written parts (which mean the client violated the uniform-part-size
// assumption) and returns ErrInvalidPart in that case.
func (up *multipartUpload) flushAt(offset, size int64, rw *pool.RW) error {
	if size == 0 {
		// Nothing to write and nothing to track.
		return nil
	}
	r := ranges.Range{Pos: offset, Size: size}

	up.mu.Lock()
	if up.written.Present(r) {
		up.mu.Unlock()
		return gofakes3.ErrInvalidPart
	}
	up.written.Insert(r)
	up.mu.Unlock()

	if _, err := rw.Seek(0, io.SeekStart); err != nil {
		return err
	}
	w := io.NewOffsetWriter(up.writerAt, offset)
	n, err := io.Copy(w, rw)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("serve s3: short write at offset %d: %d of %d", offset, n, size)
	}
	return nil
}

// CompleteMultipartUpload finalises a streamed multipart upload. It closes
// the underlying writer, registers the new file with the VFS, computes the
// S3-style multipart ETag, and stores the user metadata so HeadObject and
// GetObject see the same fields the in-memory PutObject path produces.
func (b *s3Backend) CompleteMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID, input *gofakes3.CompleteMultipartUploadRequest) (gofakes3.VersionID, string, error) {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return "", "", err
	}
	defer b.multipartUploads.Delete(uploadID)

	if err := up.validate(input); err != nil {
		_ = up.abort(ctx)
		return "", "", err
	}

	if up.writerAt != nil {
		up.mu.Lock()
		hasPending := len(up.pending) > 0
		var totalSize int64
		for _, sz := range up.partSizes {
			totalSize += sz
		}
		missing := up.written.FindMissing(ranges.Range{Pos: 0, Size: totalSize})
		up.mu.Unlock()
		if hasPending {
			_ = up.abort(ctx)
			return "", "", gofakes3.ErrInvalidPart
		}
		if !missing.IsEmpty() {
			_ = up.abort(ctx)
			return "", "", gofakes3.ErrInvalidPart
		}
	}

	if err := up.close(ctx); err != nil {
		return "", "", err
	}

	_vfs, err := b.s.getVFS(ctx)
	if err != nil {
		return "", "", err
	}

	// Invalidate the parent directory's cached listing so subsequent VFS
	// Stat / List calls pick up the newly-written object from the
	// underlying Fs (we wrote to the Fs directly, bypassing VFS).
	if root, err := _vfs.Root(); err == nil {
		root.ForgetPath(up.fp, fs.EntryObject)
	}

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

// AbortMultipartUpload tears down an in-progress upload, asking the
// underlying writer to discard any data already sent.
func (b *s3Backend) AbortMultipartUpload(ctx context.Context, bucketName, objectName string, uploadID gofakes3.UploadID) error {
	up, err := b.loadUpload(uploadID)
	if err != nil {
		return err
	}
	defer b.multipartUploads.Delete(uploadID)
	return up.abort(ctx)
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

// close finalises the underlying writer.
func (up *multipartUpload) close(ctx context.Context) error {
	up.mu.Lock()
	if up.closed {
		up.mu.Unlock()
		return nil
	}
	up.closed = true
	up.mu.Unlock()

	switch {
	case up.chunkWriter != nil:
		return up.chunkWriter.Close(ctx)
	case up.writerAt != nil:
		return up.writerAt.Close()
	}
	return nil
}

// abort cancels the underlying writer and releases any pending buffers.
func (up *multipartUpload) abort(ctx context.Context) error {
	up.mu.Lock()
	if up.closed {
		up.mu.Unlock()
		return nil
	}
	up.closed = true
	pending := up.pending
	up.pending = nil
	up.mu.Unlock()

	for _, rw := range pending {
		_ = rw.Close()
	}

	switch {
	case up.chunkWriter != nil:
		return up.chunkWriter.Abort(ctx)
	case up.writerAt != nil:
		return up.writerAt.Close()
	}
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
