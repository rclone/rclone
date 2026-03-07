package internxt

import (
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
// All encryption is handled by the SDK's ChunkUploadSession.
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
}

// OpenChunkWriter returns the chunk size and a ChunkWriter for multipart uploads.
//
// When called from Update (via multipart.UploadMultipart), the session is
// pre-created and stored in f.pendingSession so that the encrypting reader
// can be applied to the input before UploadMultipart reads from it.
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	size := src.Size()

	chunkSize := f.opt.ChunkSize
	if size < 0 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				chunkSize, fs.SizeSuffix(int64(chunkSize)*int64(maxUploadParts)))
		})
	} else {
		chunkSize = chunksize.Calculator(src, size, maxUploadParts, chunkSize)
	}

	// Ensure parent directory exists
	_, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return info, nil, fmt.Errorf("failed to find parent directory: %w", err)
	}

	// Use pre-created session from Update() if available, otherwise create one
	session := f.pendingSession
	if session == nil {
		err = f.pacer.Call(func() (bool, error) {
			var err error
			session, err = buckets.NewChunkUploadSession(ctx, f.cfg, size, int64(chunkSize))
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return info, nil, fmt.Errorf("failed to create upload session: %w", err)
		}
	}

	w := &internxtChunkWriter{
		f:       f,
		remote:  remote,
		src:     src,
		session: session,
		size:    size,
		dirID:   dirID,
	}

	info = fs.ChunkWriterInfo{
		ChunkSize:         int64(chunkSize),
		Concurrency:       f.opt.UploadConcurrency,
		LeavePartsOnError: false,
	}

	return info, w, nil
}

// WriteChunk uploads chunk number with reader bytes.
// The data has already been encrypted by the EncryptingReader applied
// to the input stream before UploadMultipart started reading.
func (w *internxtChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	// Determine chunk size from the reader
	currentPos, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("failed to get current position: %w", err)
	}
	end, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to seek to end: %w", err)
	}
	size := end - currentPos
	if _, err := reader.Seek(currentPos, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek back: %w", err)
	}

	if size == 0 {
		return 0, nil
	}

	var etag string
	err = w.f.pacer.Call(func() (bool, error) {
		// Seek back to start for retries
		if _, err := reader.Seek(currentPos, io.SeekStart); err != nil {
			return false, err
		}
		var uploadErr error
		etag, uploadErr = w.session.UploadChunk(ctx, chunkNumber, reader, size)
		return w.f.shouldRetry(ctx, uploadErr)
	})
	if err != nil {
		return 0, err
	}

	w.partsMu.Lock()
	w.completedParts = append(w.completedParts, buckets.CompletedPart{
		PartNumber: chunkNumber + 1,
		ETag:       etag,
	})
	w.partsMu.Unlock()

	return size, nil
}

// Close completes the multipart upload and registers the file in Internxt Drive.
func (w *internxtChunkWriter) Close(ctx context.Context) error {
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
