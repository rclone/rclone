package pan123

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/123pan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/readers"
)

const (
	// maxMemoryUploadSize is the maximum file size to load into memory for upload
	maxMemoryUploadSize   = 100 * 1024 * 1024 // 100MB
	uploadCompleteMaxWait = 120               // 2 minutes max wait (in seconds)
	sliceUploadMaxRetries = 5
	sliceUploadTimeout    = 5 * time.Minute
)

// ensureHTTPScheme ensures the URL has an HTTP scheme
func ensureHTTPScheme(url string) string {
	if !strings.HasPrefix(url, "http") {
		return "http://" + url
	}
	return url
}

// calculateMD5 calculates MD5 hash of data and returns hex string
func calculateMD5(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

// upload uploads a file to 123pan
func (f *Fs) upload(ctx context.Context, in io.Reader, parentID int64, filename string, size int64, options ...fs.OpenOption) (*api.File, error) {
	// For small files, use the simple in-memory approach
	if size <= maxMemoryUploadSize {
		return f.uploadSmallFile(ctx, in, parentID, filename, size)
	}
	// For large files, use streaming upload
	return f.uploadLargeFile(ctx, in, parentID, filename, size)
}

// uploadSmallFile handles uploads for files <= maxMemoryUploadSize
func (f *Fs) uploadSmallFile(ctx context.Context, in io.Reader, parentID int64, filename string, size int64) (*api.File, error) {
	// Read the entire file to calculate MD5 (required by 123pan API)
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	if int64(len(data)) != size {
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", size, len(data))
	}

	etag := calculateMD5(data)

	// sliceUploader is called if instant upload fails
	sliceUploader := func(server, preuploadID string, sliceSize int64) error {
		return f.uploadSlicesFromMemory(ctx, data, server, preuploadID, sliceSize)
	}

	return f.processUpload(ctx, parentID, filename, etag, size, sliceUploader)
}

// uploadLargeFile handles uploads for files > maxMemoryUploadSize using streaming
func (f *Fs) uploadLargeFile(ctx context.Context, in io.Reader, parentID int64, filename string, size int64) (*api.File, error) {
	// Calculate MD5 and prepare reader for upload
	etag, reader, err := f.prepareStreamingUpload(in, filename, size)
	if err != nil {
		return nil, err
	}

	// sliceUploader is called if instant upload fails
	sliceUploader := func(server, preuploadID string, sliceSize int64) error {
		return f.uploadSlicesStreaming(ctx, reader, size, server, preuploadID, sliceSize)
	}

	return f.processUpload(ctx, parentID, filename, etag, size, sliceUploader)
}

// prepareStreamingUpload calculates MD5 and prepares reader for streaming upload
func (f *Fs) prepareStreamingUpload(in io.Reader, filename string, size int64) (string, io.Reader, error) {
	// Unwrap accounting to check if the underlying reader can seek
	// We need to check the underlying reader because accounting wrappers
	// implement io.Seeker but may delegate to readers that don't support Seek
	// (e.g., asyncreader.AsyncReader)
	unwrapped, wrap := accounting.UnWrap(in)
	if seeker, canSeek := unwrapped.(io.Seeker); canSeek {
		fs.Debugf(f, "Calculating MD5 for large file %s (streaming)", filename)
		hasher := md5.New()
		n, err := io.Copy(hasher, in)
		if err != nil {
			return "", nil, fmt.Errorf("failed to calculate MD5: %w", err)
		}
		if n != size {
			return "", nil, fmt.Errorf("file size mismatch: expected %d, got %d", size, n)
		}
		if _, err = seeker.Seek(0, io.SeekStart); err != nil {
			return "", nil, fmt.Errorf("failed to seek: %w", err)
		}
		// Re-wrap the reader with accounting if it was wrapped before
		return hex.EncodeToString(hasher.Sum(nil)), wrap(unwrapped), nil
	}

	// Cannot seek, use RepeatableReader
	fs.Debugf(f, "Using RepeatableReader for large file %s", filename)
	repeatableReader := readers.NewRepeatableReader(in)
	hasher := md5.New()
	n, err := io.Copy(hasher, repeatableReader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to calculate MD5: %w", err)
	}
	if n != size {
		return "", nil, fmt.Errorf("file size mismatch: expected %d, got %d", size, n)
	}
	if _, err = repeatableReader.Seek(0, io.SeekStart); err != nil {
		return "", nil, fmt.Errorf("failed to seek: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), repeatableReader, nil
}

// sliceUploaderFunc is a function type for uploading slices
type sliceUploaderFunc func(server, preuploadID string, sliceSize int64) error

// processUpload handles the common upload logic after MD5 calculation
func (f *Fs) processUpload(ctx context.Context, parentID int64, filename, etag string, size int64, uploadSlices sliceUploaderFunc) (*api.File, error) {
	// Create file (check for instant upload)
	createResp, err := f.createFile(ctx, parentID, filename, etag, size)
	if err != nil {
		return nil, err
	}

	fileID := createResp.Data.FileID

	// Check for instant upload (秒传)
	if createResp.Data.Reuse {
		fs.Debugf(f, "Instant upload succeeded for %s", filename)
	} else {
		if len(createResp.Data.Servers) == 0 {
			return nil, errors.New("no upload server returned")
		}

		// Upload slices
		if err = uploadSlices(createResp.Data.Servers[0], createResp.Data.PreuploadID, createResp.Data.SliceSize); err != nil {
			return nil, fmt.Errorf("failed to upload slices: %w", err)
		}

		// Complete upload with polling
		fileID, err = f.waitForUploadComplete(ctx, createResp.Data.PreuploadID)
		if err != nil {
			return nil, err
		}
	}

	return &api.File{
		FileID:       fileID,
		Filename:     filename,
		Size:         size,
		Etag:         etag,
		ParentFileID: parentID,
		Type:         0,
		UpdateAt:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// waitForUploadComplete polls the upload_complete endpoint until the upload is finished
func (f *Fs) waitForUploadComplete(ctx context.Context, preuploadID string) (int64, error) {
	for i := 0; i < uploadCompleteMaxWait; i++ {
		completeResp, err := f.completeUpload(ctx, preuploadID)
		if err == nil && completeResp.Data.Completed && completeResp.Data.FileID != 0 {
			return completeResp.Data.FileID, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return 0, errors.New("upload complete timeout")
}

// sliceProvider returns slice data for a given slice number (1-indexed)
type sliceProvider func(sliceNo int64) ([]byte, error)

// uploadSlicesFromMemory uploads file data in slices from memory buffer
func (f *Fs) uploadSlicesFromMemory(ctx context.Context, data []byte, uploadServer, preuploadID string, sliceSize int64) error {
	size := int64(len(data))
	provider := func(sliceNo int64) ([]byte, error) {
		offset := (sliceNo - 1) * sliceSize
		end := offset + sliceSize
		if end > size {
			end = size
		}
		return data[offset:end], nil
	}
	return f.uploadSlices(ctx, size, sliceSize, uploadServer, preuploadID, provider)
}

// uploadSlicesStreaming uploads file data in slices using streaming
func (f *Fs) uploadSlicesStreaming(ctx context.Context, reader io.Reader, size int64, uploadServer, preuploadID string, sliceSize int64) error {
	// Unwrap accounting to get the raw reader
	if unwrapped, _ := accounting.UnWrap(reader); unwrapped != nil {
		reader = unwrapped
	}
	sliceBuffer := make([]byte, sliceSize)

	provider := func(sliceNo int64) ([]byte, error) {
		expectedSize := sliceSize
		numSlices := (size + sliceSize - 1) / sliceSize
		if sliceNo == numSlices {
			expectedSize = size - (sliceNo-1)*sliceSize
		}
		n, err := io.ReadFull(reader, sliceBuffer[:expectedSize])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read slice %d: %w", sliceNo, err)
		}
		if int64(n) != expectedSize {
			return nil, fmt.Errorf("slice %d: expected %d bytes, got %d", sliceNo, expectedSize, n)
		}
		return sliceBuffer[:n], nil
	}
	return f.uploadSlices(ctx, size, sliceSize, uploadServer, preuploadID, provider)
}

// uploadSlices uploads file data in slices using the provided slice provider
func (f *Fs) uploadSlices(ctx context.Context, size, sliceSize int64, uploadServer, preuploadID string, provider sliceProvider) error {
	numSlices := (size + sliceSize - 1) / sliceSize
	uploadServer = ensureHTTPScheme(uploadServer)

	for sliceNo := int64(1); sliceNo <= numSlices; sliceNo++ {
		sliceData, err := provider(sliceNo)
		if err != nil {
			return err
		}
		sliceMD5 := calculateMD5(sliceData)
		if err := f.uploadSliceWithRetry(ctx, uploadServer, preuploadID, sliceNo, sliceMD5, sliceData); err != nil {
			return fmt.Errorf("failed to upload slice %d: %w", sliceNo, err)
		}
		fs.Debugf(f, "Uploaded slice %d/%d", sliceNo, numSlices)
	}
	return nil
}

// uploadSliceWithRetry uploads a single slice with retry on failure
func (f *Fs) uploadSliceWithRetry(ctx context.Context, server, preuploadID string, sliceNo int64, sliceMD5 string, data []byte) error {
	var lastErr error

	for attempt := 0; attempt < sliceUploadMaxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := calculateRetryDelay(attempt-1, baseRetryDelay, maxRetryDelay)
			fs.Debugf(f, "Retrying slice %d, attempt %d/%d after %v", sliceNo, attempt+1, sliceUploadMaxRetries, retryDelay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		err := f.uploadSlice(ctx, server, preuploadID, sliceNo, sliceMD5, data)
		if err == nil {
			return nil
		}
		lastErr = err
		fs.Debugf(f, "Slice %d upload failed (attempt %d/%d): %v", sliceNo, attempt+1, sliceUploadMaxRetries, err)
	}
	return fmt.Errorf("slice upload failed after %d retries: %w", sliceUploadMaxRetries, lastErr)
}

// uploadSlice uploads a single slice
func (f *Fs) uploadSlice(ctx context.Context, server, preuploadID string, sliceNo int64, sliceMD5 string, data []byte) (err error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Add form fields
	if err := w.WriteField("preuploadID", preuploadID); err != nil {
		return err
	}
	if err := w.WriteField("sliceNo", formatID(sliceNo)); err != nil {
		return err
	}
	if err := w.WriteField("sliceMD5", sliceMD5); err != nil {
		return err
	}

	// Add file data
	fw, err := w.CreateFormFile("slice", fmt.Sprintf("part%d", sliceNo))
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	// Create and send request
	req, err := http.NewRequestWithContext(ctx, "POST", server+"/upload/v2/file/slice", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.opt.AccessToken)
	req.Header.Set("Platform", "open_platform")
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{Timeout: sliceUploadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slice upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp api.BaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}
	if apiResp.Code != 0 {
		return fmt.Errorf("slice upload API error: %s (code %d)", apiResp.Message, apiResp.Code)
	}
	return nil
}
