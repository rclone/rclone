package internxt

import (
	"context"
	"crypto/cipher"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/internxt/rclone-adapter/buckets"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pool"
)

var warnStreamUpload sync.Once

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	if cs < minChunkSize {
		return fmt.Errorf("%s is less than %s", cs, minChunkSize)
	}
	return nil
}

// SetUploadChunkSize sets the chunk size used for multipart uploads
func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	err := checkUploadChunkSize(cs)
	if err == nil {
		old := f.opt.ChunkSize
		f.opt.ChunkSize = cs
		return old, nil
	}
	return f.opt.ChunkSize, err
}

func checkUploadCutoff(cs fs.SizeSuffix) error {
	if cs < minUploadCutoff {
		return fmt.Errorf("%s is less than %s (Internxt requires minimum %s for multipart uploads)", cs, minUploadCutoff, minUploadCutoff)
	}
	if cs > maxUploadCutoff {
		return fmt.Errorf("%s is greater than %s", cs, maxUploadCutoff)
	}
	return nil
}

// SetUploadCutoff sets the cutoff for switching to multipart upload
func (f *Fs) SetUploadCutoff(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	err := checkUploadCutoff(cs)
	if err == nil {
		old := f.opt.UploadCutoff
		f.opt.UploadCutoff = cs
		return old, nil
	}
	return f.opt.UploadCutoff, err
}

// internxtChunkWriter implements fs.ChunkWriter for Internxt multipart uploads.
type internxtChunkWriter struct {
	f              *Fs
	remote         string
	src            fs.ObjectInfo
	session        *buckets.ChunkUploadSession
	completedParts []buckets.CompletedPart
	partsMu        sync.Mutex
	size           int64
	dirID          string
	meta           *buckets.CreateMetaResponse
	chunkSize      int64
	hashMu         sync.Mutex
	nextHashChunk  int
	pendingChunks  map[int]*pool.RW
}

// OpenChunkWriter returns the chunk size and a ChunkWriter for multipart uploads.
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	size := src.Size()

	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(f.opt.ChunkSize),
		Concurrency:       f.opt.UploadConcurrency,
		LeavePartsOnError: false,
	}

	// Reject files below the upload cutoff
	if size >= 0 && size < int64(f.opt.UploadCutoff) {
		return info, nil, &fs.FileTooSmallError{MinSize: int64(f.opt.UploadCutoff)}
	}

	chunkSize := f.opt.ChunkSize
	if size < 0 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				chunkSize, fs.SizeSuffix(int64(chunkSize)*int64(maxUploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(src, size, maxUploadParts, chunkSize)
		info.ChunkSize = int64(chunkSize)
	}

	// Ensure parent directory exists
	_, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return info, nil, fmt.Errorf("failed to find parent directory: %w", err)
	}

	var session *buckets.ChunkUploadSession
	err = f.pacer.Call(func() (bool, error) {
		var err error
		session, err = buckets.NewChunkUploadSession(ctx, f.cfg, size, int64(chunkSize))
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return info, nil, fmt.Errorf("failed to create upload session: %w", err)
	}

	w := &internxtChunkWriter{
		f:             f,
		remote:        remote,
		src:           src,
		session:       session,
		size:          size,
		dirID:         dirID,
		chunkSize:     int64(chunkSize),
		pendingChunks: make(map[int]*pool.RW),
	}

	return info, w, nil
}

// WriteChunk encrypts plaintext per-chunk using AES-256-CTR at the correct
// byte offset, uploads the encrypted data, then feeds it into the ordered
// hash accumulator
func (w *internxtChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	byteOffset := int64(chunkNumber) * w.chunkSize
	cipherStream, err := w.session.NewCipherAtOffset(byteOffset)
	if err != nil {
		return 0, err
	}

	encRW := multipart.NewRW().Reserve(w.chunkSize)
	cipherReader := &cipher.StreamReader{S: cipherStream, R: reader}
	size, err := io.Copy(encRW, cipherReader)
	if err != nil {
		_ = encRW.Close()
		return 0, err
	}
	if size == 0 {
		_ = encRW.Close()
		return 0, nil
	}

	var etag string
	err = w.f.pacer.Call(func() (bool, error) {
		if _, err := encRW.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		var uploadErr error
		etag, uploadErr = w.session.UploadChunk(ctx, chunkNumber, encRW, size)
		return w.f.shouldRetry(ctx, uploadErr)
	})
	if err != nil {
		_ = encRW.Close()
		return 0, err
	}

	w.recordCompletedPart(chunkNumber, etag)

	if _, err := encRW.Seek(0, io.SeekStart); err != nil {
		_ = encRW.Close()
		return 0, err
	}
	w.submitForHashing(chunkNumber, encRW)

	return size, nil
}

// recordCompletedPart appends a completed part to the list (thread-safe).
func (w *internxtChunkWriter) recordCompletedPart(chunkNumber int, etag string) {
	w.partsMu.Lock()
	w.completedParts = append(w.completedParts, buckets.CompletedPart{
		PartNumber: chunkNumber + 1,
		ETag:       etag,
	})
	w.partsMu.Unlock()
}

// hashWriter is an io.Writer that feeds data into the session's SHA-256 hash.
type hashWriter struct {
	session *buckets.ChunkUploadSession
}

func (hw hashWriter) Write(p []byte) (int, error) {
	hw.session.HashEncryptedData(p)
	return len(p), nil
}

// submitForHashing feeds encrypted chunk data into the session's hash in order.
func (w *internxtChunkWriter) submitForHashing(chunkNumber int, encRW *pool.RW) {
	w.hashMu.Lock()
	defer w.hashMu.Unlock()

	hw := hashWriter{w.session}

	if chunkNumber == w.nextHashChunk {
		_, _ = encRW.WriteTo(hw)
		_ = encRW.Close()
		w.nextHashChunk++
		for {
			next, ok := w.pendingChunks[w.nextHashChunk]
			if !ok {
				break
			}
			_, _ = next.WriteTo(hw)
			_ = next.Close()
			delete(w.pendingChunks, w.nextHashChunk)
			w.nextHashChunk++
		}
	} else {
		w.pendingChunks[chunkNumber] = encRW
	}
}

// Close completes the multipart upload and registers the file in Internxt Drive.
func (w *internxtChunkWriter) Close(ctx context.Context) error {
	w.hashMu.Lock()
	pending := len(w.pendingChunks)
	if pending != 0 {
		for _, rw := range w.pendingChunks {
			_ = rw.Close()
		}
		w.pendingChunks = nil
	}
	w.hashMu.Unlock()
	if pending != 0 {
		return fmt.Errorf("internal error: %d chunks still pending hash", pending)
	}

	// Sort parts by part number
	w.partsMu.Lock()
	sort.Slice(w.completedParts, func(i, j int) bool {
		return w.completedParts[i].PartNumber < w.completedParts[j].PartNumber
	})
	parts := make([]buckets.CompletedPart, len(w.completedParts))
	copy(parts, w.completedParts)
	w.partsMu.Unlock()

	// Finish multipart upload (SDK computes hash + calls FinishMultipartUpload)
	var finishResp *buckets.FinishUploadResp
	err := w.f.pacer.Call(func() (bool, error) {
		var err error
		finishResp, err = w.session.Finish(ctx, parts)
		return w.f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to finish multipart upload: %w", err)
	}

	// Create file metadata in Internxt Drive
	baseName := w.f.opt.Encoding.FromStandardName(path.Base(w.remote))
	name := strings.TrimSuffix(baseName, path.Ext(baseName))
	ext := strings.TrimPrefix(path.Ext(baseName), ".")

	var meta *buckets.CreateMetaResponse
	err = w.f.pacer.Call(func() (bool, error) {
		var err error
		meta, err = buckets.CreateMetaFile(ctx, w.f.cfg,
			name, w.f.cfg.Bucket, &finishResp.ID, "03-aes",
			w.dirID, name, ext, w.size, w.src.ModTime(ctx))
		return w.f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to create file metadata: %w", err)
	}
	w.meta = meta

	return nil
}

// Abort cleans up after a failed upload.
func (w *internxtChunkWriter) Abort(ctx context.Context) error {
	w.hashMu.Lock()
	for _, rw := range w.pendingChunks {
		_ = rw.Close()
	}
	w.pendingChunks = nil
	w.hashMu.Unlock()
	fs.Logf(w.f, "Multipart upload aborted for %s", w.remote)
	return nil
}
