// Package api provides metadata downloading for YouTube videos
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// VideoMetadata contains extended metadata for a YouTube video
type VideoMetadata struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Duration     int                    `json:"duration"` // in seconds
	ThumbnailURL string                 `json:"thumbnail"`
	UploadDate   string                 `json:"upload_date"`
	Uploader     string                 `json:"uploader"`
	UploaderID   string                 `json:"uploader_id"`
	ViewCount    int                    `json:"view_count"`
	LikeCount    int                    `json:"like_count"`
	Subtitles    map[string]interface{} `json:"subtitles"`
	Chapters     []Chapter              `json:"chapters"`
}

// Chapter represents a chapter/section in a video
type Chapter struct {
	Title     string `json:"title"`
	StartTime int    `json:"start_time"` // in seconds
	EndTime   int    `json:"end_time"`   // in seconds
}

// ThumbnailResult holds thumbnail download result
type ThumbnailResult struct {
	URL       string
	LocalPath string
	Size      int64
}

// CaptionResult holds caption download result
type CaptionResult struct {
	Language  string
	LocalPath string
	Size      int64
}

const (
	// retryAttempts is the number of retry attempts for transient failures
	retryAttempts = 3
	// retryDelay is the initial delay between retries (exponential backoff)
	retryDelay = 100 * time.Millisecond
	// requestTimeout is the timeout for individual requests
	requestTimeout = 30 * time.Second
)

// GetVideoMetadata fetches full metadata for a video using yt-dlp JSON output
func (c *Client) GetVideoMetadata(ctx context.Context, videoURL string) (*VideoMetadata, error) {
	data, err := extractJSON(ctx, videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract video metadata: %w", err)
	}

	var meta VideoMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse video metadata: %w", err)
	}

	// Ensure chapters is not nil for consistency
	if meta.Chapters == nil {
		meta.Chapters = []Chapter{}
	}

	return &meta, nil
}

// DownloadThumbnail downloads and caches a video thumbnail locally
// Returns the local path to the cached JPG file
func (c *Client) DownloadThumbnail(ctx context.Context, videoURL, cachePath string) (*ThumbnailResult, error) {
	// Fetch metadata to get thumbnail URL
	metadata, err := c.GetVideoMetadata(ctx, videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	if metadata.ThumbnailURL == "" {
		return nil, fmt.Errorf("no thumbnail available for video")
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Determine output path - use video ID for filename
	thumbPath := filepath.Join(cachePath, metadata.ID+".jpg")

	// Check if thumbnail already cached
	if info, err := os.Stat(thumbPath); err == nil {
		return &ThumbnailResult{
			URL:       metadata.ThumbnailURL,
			LocalPath: thumbPath,
			Size:      info.Size(),
		}, nil
	}

	// Download thumbnail with retries
	for attempt := 0; attempt < retryAttempts; attempt++ {
		err := downloadThumbnailWithRetry(ctx, metadata.ThumbnailURL, thumbPath)
		if err == nil {
			info, _ := os.Stat(thumbPath)
			return &ThumbnailResult{
				URL:       metadata.ThumbnailURL,
				LocalPath: thumbPath,
				Size:      info.Size(),
			}, nil
		}

		if attempt < retryAttempts-1 {
			select {
			case <-time.After(retryDelay * time.Duration(1<<uint(attempt))):
				// exponential backoff
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("failed to download thumbnail after %d attempts", retryAttempts)
}

// downloadThumbnailWithRetry downloads a thumbnail from a URL
func downloadThumbnailWithRetry(ctx context.Context, url, outputPath string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download thumbnail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("thumbnail download returned status %d", resp.StatusCode)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	_, err = file.ReadFrom(resp.Body)
	if err != nil {
		os.Remove(outputPath) // clean up partial file
		return fmt.Errorf("failed to write thumbnail: %w", err)
	}

	return nil
}

// DownloadCaptions downloads and caches video captions in SRT format
// Returns paths to all successfully downloaded caption files
func (c *Client) DownloadCaptions(ctx context.Context, videoURL, cachePath string) ([]CaptionResult, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Fetch metadata to get video ID
	metadata, err := c.GetVideoMetadata(ctx, videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	// Build output template for subtitles
	outputTemplate := filepath.Join(cachePath, metadata.ID+".%(lang)s.srt")

	// Run yt-dlp to download captions
	// --write-subs: download subtitles
	// --sub-format srt: use SRT format
	// --skip-unavailable-fragments: continue if some captions unavailable
	args := []string{
		videoURL,
		"--write-subs",
		"--sub-format", "srt",
		"--skip-unavailable-fragments",
		"-o", outputTemplate,
		"-J",
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// Ignore error - yt-dlp may fail but still create some subtitle files
	_ = cmd.Run()

	// Collect downloaded caption files
	var results []CaptionResult
	entries, _ := os.ReadDir(cachePath)
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type().IsRegular() {
			name := entry.Name()
			// Match pattern: videoID.LANG.srt
			if len(name) > len(metadata.ID)+5 && // minimum: ID + . + X + .srt
				name[:len(metadata.ID)] == metadata.ID &&
				name[len(metadata.ID)] == '.' {

				// Extract language code
				start := len(metadata.ID) + 1
				end := len(name) - 4 // exclude .srt
				if end > start {
					lang := name[start:end]
					fullPath := filepath.Join(cachePath, name)
					info, _ := os.Stat(fullPath)

					results = append(results, CaptionResult{
						Language:  lang,
						LocalPath: fullPath,
						Size:      info.Size(),
					})
				}
			}
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no captions available for video (or download failed)")
	}

	return results, nil
}

// GetChapters extracts chapter markers from video metadata
// Returns nil if no chapters are available
func (c *Client) GetChapters(ctx context.Context, videoURL string) ([]Chapter, error) {
	metadata, err := c.GetVideoMetadata(ctx, videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	if len(metadata.Chapters) == 0 {
		return nil, fmt.Errorf("no chapters available for this video")
	}

	return metadata.Chapters, nil
}

// CacheExists checks if metadata for a video is already cached locally
func (c *Client) CacheExists(cachePath, videoID string) bool {
	thumbPath := filepath.Join(cachePath, videoID+".jpg")
	_, err := os.Stat(thumbPath)
	return err == nil
}

// ClearVideoCache removes all cached files for a specific video
func (c *Client) ClearVideoCache(cachePath, videoID string) error {
	entries, err := os.ReadDir(cachePath)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && entry.Type().IsRegular() {
			name := entry.Name()
			// Match files starting with videoID
			if len(name) > len(videoID) && name[:len(videoID)] == videoID {
				path := filepath.Join(cachePath, name)
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("failed to remove cached file %s: %w", name, err)
				}
			}
		}
	}

	return nil
}
