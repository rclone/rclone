package internxt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/internxt/rclone-adapter/buckets"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
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
	pendingChunks  map[int][]byte
}

// OpenChunkWriter returns the chunk size and a ChunkWriter for multipart uploads.
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	size := src.Size()

	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(f.opt.ChunkSize),
		Concurrency:       f.opt.UploadConcurrency,
		LeavePartsOnError: false,
		MinFileSize:       minMultipartSize,
	}

	// Reject files below the multipart minimum
	if size >= 0 && size < minMultipartSize {
		return info, nil, fmt.Errorf("file size %d is below minimum %d for multipart upload", size, minMultipartSize)
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
		pendingChunks: make(map[int][]byte),
	}

	return info, w, nil
}

// WriteChunk encrypts plaintext per-chunk using AES-256-CTR at the correct
// byte offset, feeds encrypted data into the ordered hash accumulator, and
// uploads to the presigned URL.
func (w *internxtChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}
	if len(plaintext) == 0 {
		return 0, nil
	}
	size := int64(len(plaintext))

	byteOffset := int64(chunkNumber) * w.chunkSize
	cipherStream, err := w.session.NewCipherAtOffset(byteOffset)
	if err != nil {
		return 0, err
	}
	cipherStream.XORKeyStream(plaintext, plaintext)
	encrypted := plaintext

	w.submitForHashing(chunkNumber, encrypted)

	encReader := bytes.NewReader(encrypted)
	var etag string
	err = w.f.pacer.Call(func() (bool, error) {
		if _, err := encReader.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		var uploadErr error
		etag, uploadErr = w.session.UploadChunk(ctx, chunkNumber, encReader, size)
		return w.f.shouldRetry(ctx, uploadErr)
	})
	if err != nil {
		return 0, err
	}

	w.recordCompletedPart(chunkNumber, etag)
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

// submitForHashing feeds encrypted chunk data into the session's hash in order.
func (w *internxtChunkWriter) submitForHashing(chunkNumber int, encrypted []byte) {
	w.hashMu.Lock()
	defer w.hashMu.Unlock()

	if chunkNumber == w.nextHashChunk {
		w.session.HashEncryptedData(encrypted)
		w.nextHashChunk++
		for {
			next, ok := w.pendingChunks[w.nextHashChunk]
			if !ok {
				break
			}
			w.session.HashEncryptedData(next)
			delete(w.pendingChunks, w.nextHashChunk)
			w.nextHashChunk++
		}
	} else {
		buf := make([]byte, len(encrypted))
		copy(buf, encrypted)
		w.pendingChunks[chunkNumber] = buf
	}
}

// Close completes the multipart upload and registers the file in Internxt Drive.
func (w *internxtChunkWriter) Close(ctx context.Context) error {
	w.hashMu.Lock()
	pending := len(w.pendingChunks)
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
	fs.Logf(w.f, "Multipart upload aborted for %s", w.remote)
	return nil
}
