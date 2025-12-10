package mediavfs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

// httpReader implements intelligent HTTP reading with ETag support and range requests
// Based on the working Stash implementation with proper streaming support
type httpReader struct {
	ctx         context.Context
	url         string
	client      *http.Client
	size        int64
	etag        string
	etagMu      sync.Mutex
	options     []fs.OpenOption
	res         *http.Response
	offset      int64
	closeErr    error
	contentMD5  string
	retryCount  int
	maxRetries  int
	acceptRange bool
	initialized bool
}

// createStreamingHTTPClient creates an HTTP client optimized for streaming large files
func createStreamingHTTPClient() *http.Client {
	return &http.Client{
		// No timeout for streaming large files - let the context handle cancellation
		Timeout: 0,
		Transport: &http.Transport{
			// Short timeout for initial response headers
			ResponseHeaderTimeout: 10 * time.Second,
			// Disable compression to avoid buffering entire file
			DisableCompression: true,
			// Connection pooling settings
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// newHTTPReader creates a new HTTP reader with ETag support
// Does NOT make an initial request - opens stream on first read
func newHTTPReader(ctx context.Context, url string, client *http.Client, size int64, options []fs.OpenOption) (*httpReader, error) {
	// Use streaming-optimized client instead of default
	streamClient := createStreamingHTTPClient()

	r := &httpReader{
		ctx:         ctx,
		url:         url,
		client:      streamClient,
		size:        size,
		options:     options,
		maxRetries:  3,
		initialized: false,
	}

	return r, nil
}

// openStream opens or reopens the HTTP stream at the given offset
func (r *httpReader) openStream(offset int64) error {
	// Close existing response if any
	if r.res != nil {
		_ = r.res.Body.Close()
		r.res = nil
	}

	req, err := http.NewRequestWithContext(r.ctx, "GET", r.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent - some servers require this
	req.Header.Set("User-Agent", "rclone/mediavfs")

	// Add range header if we have an offset or if specified in options
	rangeStart, rangeEnd := offset, int64(-1)
	needsRange := offset > 0 || hasRangeOption(r.options)

	if needsRange {
		// Check if a specific range was requested in options
		for _, opt := range r.options {
			if rangeOpt, ok := opt.(*fs.RangeOption); ok {
				rangeStart = rangeOpt.Start
				rangeEnd = rangeOpt.End
				break
			}
		}

		if rangeEnd >= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
		} else {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", rangeStart))
		}

		fs.Debugf(nil, "mediavfs: requesting range bytes=%d-%v", rangeStart, rangeEnd)

		// CRITICAL: Use If-Range with ETag if we have one
		// This ensures we get partial content only if file hasn't changed
		r.etagMu.Lock()
		if r.etag != "" {
			req.Header.Set("If-Range", r.etag)
			fs.Debugf(nil, "mediavfs: setting If-Range with ETag: %s", r.etag)
		}
		r.etagMu.Unlock()
	}

	// Add any custom headers from options
	for k, v := range fs.OpenOptionHeaders(r.options) {
		if k != "Range" && k != "If-Range" && k != "User-Agent" {
			req.Header.Set(k, v)
		}
	}

	// Execute request
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}

	// Accept both 200 (full content) and 206 (partial content)
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		_ = res.Body.Close()
		return fmt.Errorf("HTTP error: %s (status %d)", res.Status, res.StatusCode)
	}

	// If we requested range but got 200, server doesn't support ranges properly
	if needsRange && res.StatusCode == http.StatusOK {
		fs.Debugf(nil, "mediavfs: range request returned 200 instead of 206 - server may not support ranges")
		r.acceptRange = false
	} else if res.StatusCode == http.StatusPartialContent {
		r.acceptRange = true
	}

	// Store or validate ETag
	newETag := res.Header.Get("ETag")
	if newETag != "" {
		r.etagMu.Lock()
		if !r.initialized {
			// First request, store the ETag
			r.etag = newETag
			fs.Debugf(nil, "mediavfs: stored ETag: %s", r.etag)
			r.initialized = true
		} else if r.etag != "" && r.etag != newETag {
			// ETag changed - file was modified on server
			r.etagMu.Unlock()
			_ = res.Body.Close()
			return fmt.Errorf("file changed on server (ETag mismatch: %s != %s)", r.etag, newETag)
		}
		r.etagMu.Unlock()
	}

	// Check for Accept-Ranges header
	acceptRanges := res.Header.Get("Accept-Ranges")
	if acceptRanges != "" && acceptRanges != "none" {
		r.acceptRange = true
		fs.Debugf(nil, "mediavfs: server supports range requests: %s", acceptRanges)
	}

	// Store Content-MD5 if present
	if md5 := res.Header.Get("Content-MD5"); md5 != "" {
		r.contentMD5 = md5
	}

	// Log Content-Range for debugging
	if contentRange := res.Header.Get("Content-Range"); contentRange != "" {
		fs.Debugf(nil, "mediavfs: partial content range: %s", contentRange)
	}

	r.res = res
	r.offset = offset

	// If we got full content (200) but need an offset, discard bytes
	if res.StatusCode == http.StatusOK && offset > 0 {
		fs.Debugf(nil, "mediavfs: discarding %d bytes to reach offset", offset)
		_, err := io.CopyN(io.Discard, res.Body, offset)
		if err != nil {
			_ = res.Body.Close()
			return fmt.Errorf("failed to seek to offset %d: %w", offset, err)
		}
	}

	return nil
}

// Read reads data from the HTTP stream
func (r *httpReader) Read(p []byte) (n int, err error) {
	// Open stream on first read if not already open
	if r.res == nil {
		if err := r.openStream(r.offset); err != nil {
			return 0, err
		}
	}

	n, err = r.res.Body.Read(p)
	r.offset += int64(n)

	// Handle network errors by attempting to resume
	if err != nil && err != io.EOF {
		// Check if we should retry
		if r.retryCount >= r.maxRetries {
			fs.Debugf(nil, "mediavfs: max retries (%d) reached, giving up", r.maxRetries)
			return n, err
		}

		fs.Debugf(nil, "mediavfs: read error at offset %d (attempt %d/%d): %v",
			r.offset, r.retryCount+1, r.maxRetries, err)

		r.retryCount++

		// Try to reopen the stream at current offset
		if reopenErr := r.openStream(r.offset); reopenErr == nil {
			// Successfully reopened, reset retry count and try reading again
			fs.Debugf(nil, "mediavfs: successfully resumed at offset %d", r.offset)
			r.retryCount = 0
			return r.Read(p)
		} else {
			fs.Debugf(nil, "mediavfs: failed to resume: %v", reopenErr)
		}
	} else if err == nil {
		// Successful read, reset retry count
		r.retryCount = 0
	}

	return n, err
}

// Close closes the HTTP stream
func (r *httpReader) Close() error {
	if r.res != nil {
		err := r.res.Body.Close()
		r.res = nil
		if err != nil {
			r.closeErr = err
			return err
		}
	}
	return r.closeErr
}

// GetETag returns the ETag of the file
func (r *httpReader) GetETag() string {
	r.etagMu.Lock()
	defer r.etagMu.Unlock()
	return r.etag
}

// GetContentMD5 returns the Content-MD5 header if present
func (r *httpReader) GetContentMD5() string {
	return r.contentMD5
}

// hasRangeOption checks if options contain a range request
func hasRangeOption(options []fs.OpenOption) bool {
	for _, opt := range options {
		if _, ok := opt.(*fs.RangeOption); ok {
			return true
		}
	}
	return false
}

// seekableHTTPReader wraps httpReader to add seeking capability
type seekableHTTPReader struct {
	*httpReader
	size int64
}

// newSeekableHTTPReader creates a seekable HTTP reader
func newSeekableHTTPReader(ctx context.Context, url string, client *http.Client, size int64, options []fs.OpenOption) (*seekableHTTPReader, error) {
	r, err := newHTTPReader(ctx, url, client, size, options)
	if err != nil {
		return nil, err
	}

	return &seekableHTTPReader{
		httpReader: r,
		size:       size,
	}, nil
}

// Seek implements io.Seeker
func (r *seekableHTTPReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64

	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = r.offset + offset
	case io.SeekEnd:
		if r.size < 0 {
			return 0, fmt.Errorf("cannot seek from end: size unknown")
		}
		newOffset = r.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if newOffset < 0 {
		return 0, fmt.Errorf("negative position: %d", newOffset)
	}

	// If we're already at this position, no need to reopen
	if newOffset == r.offset {
		return newOffset, nil
	}

	// Reopen stream at new offset
	err := r.openStream(newOffset)
	if err != nil {
		return 0, fmt.Errorf("seek failed: %w", err)
	}

	return newOffset, nil
}

// RangeSeek implements fs.RangeSeeker
func (r *seekableHTTPReader) RangeSeek(offset int64, whence int, length int64) (int64, error) {
	newOffset, err := r.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	_ = length // Length hint, not used currently
	return newOffset, nil
}

// optimizedHTTPReader is a simpler reader for full-file streaming
type optimizedHTTPReader struct {
	ctx     context.Context
	url     string
	client  *http.Client
	options []fs.OpenOption
	res     *http.Response
	etag    string
}

// newOptimizedHTTPReader creates a simple HTTP reader for streaming
func newOptimizedHTTPReader(ctx context.Context, url string, client *http.Client, options []fs.OpenOption) (io.ReadCloser, error) {
	// Use streaming-optimized client
	streamClient := createStreamingHTTPClient()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent
	req.Header.Set("User-Agent", "rclone/mediavfs")

	// Add headers from options
	for k, v := range fs.OpenOptionHeaders(options) {
		req.Header.Set(k, v)
	}

	// Execute request
	res, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Accept both 200 and 206
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		_ = res.Body.Close()
		return nil, fmt.Errorf("HTTP error: %s (status %d)", res.Status, res.StatusCode)
	}

	r := &optimizedHTTPReader{
		ctx:     ctx,
		url:     url,
		client:  streamClient,
		options: options,
		res:     res,
		etag:    res.Header.Get("ETag"),
	}

	return r, nil
}

// Read reads from the HTTP response
func (r *optimizedHTTPReader) Read(p []byte) (n int, err error) {
	if r.res == nil {
		return 0, io.EOF
	}
	return r.res.Body.Read(p)
}

// Close closes the HTTP response
func (r *optimizedHTTPReader) Close() error {
	if r.res != nil {
		err := r.res.Body.Close()
		r.res = nil
		return err
	}
	return nil
}

// parseContentRange parses a Content-Range header
// Format: "bytes start-end/total" or "bytes start-end/*"
func parseContentRange(s string) (start, end, total int64, err error) {
	var startStr, endStr, totalStr string

	// Parse "bytes start-end/total"
	n, err := fmt.Sscanf(s, "bytes %s-%s/%s", &startStr, &endStr, &totalStr)
	if err != nil || n != 3 {
		return 0, 0, 0, fmt.Errorf("invalid Content-Range format: %s", s)
	}

	start, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid start in Content-Range: %s", startStr)
	}

	end, err = strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid end in Content-Range: %s", endStr)
	}

	if totalStr != "*" {
		total, err = strconv.ParseInt(totalStr, 10, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid total in Content-Range: %s", totalStr)
		}
	} else {
		total = -1 // Unknown total
	}

	return start, end, total, nil
}
