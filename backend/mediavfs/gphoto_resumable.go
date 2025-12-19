package mediavfs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/rclone/rclone/fs"
)

const (
	// DefaultChunkSize is 8MB - must be multiple of 256KB (262144 bytes)
	DefaultChunkSize = 8 * 1024 * 1024
	// MinChunkSize is 256KB - the granularity required by Google
	MinChunkSize = 256 * 1024
)

// ResumableUploadSession holds the state for a resumable upload
type ResumableUploadSession struct {
	UploadURL        string
	ChunkGranularity int64
	TotalSize        int64
	BytesUploaded    int64
}

// InitiateResumableUpload starts a resumable upload session
// Returns the session with upload URL and chunk granularity
func (api *GPhotoAPI) InitiateResumableUpload(ctx context.Context, fileSize int64, mimeType string) (*ResumableUploadSession, error) {
	url := "https://photoslibrary.googleapis.com/v1/uploads"

	headers := map[string]string{
		"Content-Length":          "0",
		"X-Goog-Upload-Command":   "start",
		"X-Goog-Upload-Protocol":  "resumable",
		"X-Goog-Upload-Raw-Size":  fmt.Sprintf("%d", fileSize),
		"X-Goog-Upload-File-Name": "upload", // Will be set properly in commit
	}

	if mimeType != "" {
		headers["X-Goog-Upload-Content-Type"] = mimeType
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+api.token)
	req.Header.Set("User-Agent", api.userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	fs.Debugf(nil, "gphoto: initiating resumable upload for %d bytes", fileSize)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate resumable upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to initiate resumable upload: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Extract upload URL from response header
	uploadURL := resp.Header.Get("X-Goog-Upload-URL")
	if uploadURL == "" {
		return nil, fmt.Errorf("no upload URL in response")
	}

	// Get chunk granularity (default to 256KB if not specified)
	granularity := int64(MinChunkSize)
	if g := resp.Header.Get("X-Goog-Upload-Chunk-Granularity"); g != "" {
		if parsed, err := strconv.ParseInt(g, 10, 64); err == nil {
			granularity = parsed
		}
	}

	fs.Debugf(nil, "gphoto: resumable upload initiated, URL: %s, granularity: %d", uploadURL, granularity)

	return &ResumableUploadSession{
		UploadURL:        uploadURL,
		ChunkGranularity: granularity,
		TotalSize:        fileSize,
		BytesUploaded:    0,
	}, nil
}

// UploadChunk uploads a single chunk to the resumable upload session
// Returns the number of bytes uploaded and any error
func (api *GPhotoAPI) UploadChunk(ctx context.Context, session *ResumableUploadSession, chunk []byte, isLast bool) (int64, error) {
	offset := session.BytesUploaded
	chunkSize := int64(len(chunk))

	command := "upload"
	if isLast {
		command = "upload, finalize"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", session.UploadURL, bytes.NewReader(chunk))
	if err != nil {
		return 0, fmt.Errorf("failed to create chunk request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+api.token)
	req.Header.Set("User-Agent", api.userAgent)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", chunkSize))
	req.Header.Set("X-Goog-Upload-Command", command)
	req.Header.Set("X-Goog-Upload-Offset", fmt.Sprintf("%d", offset))

	fs.Debugf(nil, "gphoto: uploading chunk: offset=%d, size=%d, isLast=%v", offset, chunkSize, isLast)

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to upload chunk: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("chunk upload failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	session.BytesUploaded += chunkSize

	// For the final chunk, read the upload token from response
	if isLast {
		fs.Debugf(nil, "gphoto: resumable upload complete, total bytes: %d", session.BytesUploaded)
	}

	return chunkSize, nil
}

// QueryUploadStatus queries the current status of a resumable upload
// Useful for resuming after a failure
func (api *GPhotoAPI) QueryUploadStatus(ctx context.Context, session *ResumableUploadSession) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", session.UploadURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+api.token)
	req.Header.Set("User-Agent", api.userAgent)
	req.Header.Set("Content-Length", "0")
	req.Header.Set("X-Goog-Upload-Command", "query")

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to query upload status: %w", err)
	}
	defer resp.Body.Close()

	// Get the current offset from response header
	offsetStr := resp.Header.Get("X-Goog-Upload-Size-Received")
	if offsetStr == "" {
		return 0, nil
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse upload offset: %w", err)
	}

	session.BytesUploaded = offset
	return offset, nil
}

// ResumableUploadFile performs a resumable upload with automatic chunking
// This is the main function to use for large files
func (api *GPhotoAPI) ResumableUploadFile(ctx context.Context, content io.Reader, fileSize int64, mimeType string, chunkSize int64) ([]byte, error) {
	// Validate chunk size
	if chunkSize < MinChunkSize {
		chunkSize = DefaultChunkSize
	}
	// Ensure chunk size is multiple of MinChunkSize
	chunkSize = (chunkSize / MinChunkSize) * MinChunkSize

	// Initiate the resumable upload
	session, err := api.InitiateResumableUpload(ctx, fileSize, mimeType)
	if err != nil {
		return nil, err
	}

	fs.Infof(nil, "gphoto: starting resumable upload of %d bytes in chunks of %d bytes", fileSize, chunkSize)

	// Read and upload chunks
	buffer := make([]byte, chunkSize)
	var lastResponse []byte

	for session.BytesUploaded < fileSize {
		// Calculate how much to read for this chunk
		remaining := fileSize - session.BytesUploaded
		toRead := chunkSize
		if remaining < chunkSize {
			toRead = remaining
		}

		// Read the chunk
		n, err := io.ReadFull(content, buffer[:toRead])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}

		if n == 0 {
			break
		}

		chunk := buffer[:n]
		isLast := session.BytesUploaded+int64(n) >= fileSize

		// Upload the chunk with retry
		_, err = api.uploadChunkWithRetry(ctx, session, chunk, isLast, 3)
		if err != nil {
			return nil, err
		}

		// Log progress
		progress := float64(session.BytesUploaded) / float64(fileSize) * 100
		fs.Infof(nil, "gphoto: upload progress: %.1f%% (%d / %d bytes)", progress, session.BytesUploaded, fileSize)
	}

	fs.Infof(nil, "gphoto: resumable upload complete")
	return lastResponse, nil
}

// uploadChunkWithRetry uploads a chunk with automatic retry on failure
func (api *GPhotoAPI) uploadChunkWithRetry(ctx context.Context, session *ResumableUploadSession, chunk []byte, isLast bool, maxRetries int) (int64, error) {
	var lastErr error

	for retry := 0; retry < maxRetries; retry++ {
		n, err := api.UploadChunk(ctx, session, chunk, isLast)
		if err == nil {
			return n, nil
		}

		lastErr = err
		fs.Debugf(nil, "gphoto: chunk upload failed (attempt %d/%d): %v", retry+1, maxRetries, err)

		// Query the current status to see where we are
		currentOffset, queryErr := api.QueryUploadStatus(ctx, session)
		if queryErr != nil {
			fs.Debugf(nil, "gphoto: failed to query upload status: %v", queryErr)
			continue
		}

		// If the chunk was actually uploaded, we're done
		expectedOffset := session.BytesUploaded
		if currentOffset >= expectedOffset+int64(len(chunk)) {
			session.BytesUploaded = currentOffset
			return int64(len(chunk)), nil
		}

		// Update session with current offset for next retry
		session.BytesUploaded = currentOffset
	}

	return 0, fmt.Errorf("chunk upload failed after %d retries: %w", maxRetries, lastErr)
}

// ResumableUploadWithProgress performs a resumable upload with progress callback
func (api *GPhotoAPI) ResumableUploadWithProgress(
	ctx context.Context,
	content io.Reader,
	fileSize int64,
	mimeType string,
	chunkSize int64,
	progressFn func(bytesUploaded, totalBytes int64),
) ([]byte, error) {
	// Validate chunk size
	if chunkSize < MinChunkSize {
		chunkSize = DefaultChunkSize
	}
	chunkSize = (chunkSize / MinChunkSize) * MinChunkSize

	// Initiate the resumable upload
	session, err := api.InitiateResumableUpload(ctx, fileSize, mimeType)
	if err != nil {
		return nil, err
	}

	fs.Infof(nil, "gphoto: starting resumable upload of %d bytes", fileSize)

	buffer := make([]byte, chunkSize)

	for session.BytesUploaded < fileSize {
		remaining := fileSize - session.BytesUploaded
		toRead := chunkSize
		if remaining < chunkSize {
			toRead = remaining
		}

		n, err := io.ReadFull(content, buffer[:toRead])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}

		if n == 0 {
			break
		}

		chunk := buffer[:n]
		isLast := session.BytesUploaded+int64(n) >= fileSize

		_, err = api.uploadChunkWithRetry(ctx, session, chunk, isLast, 3)
		if err != nil {
			return nil, err
		}

		if progressFn != nil {
			progressFn(session.BytesUploaded, fileSize)
		}
	}

	fs.Infof(nil, "gphoto: resumable upload complete")
	return nil, nil
}
