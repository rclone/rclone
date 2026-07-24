// Package api provides a yt-dlp wrapper for YouTube metadata access
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Client wraps yt-dlp for YouTube metadata queries
type Client struct{}

// NewClient creates a new yt-dlp client
func NewClient() *Client {
	return &Client{}
}

// Channel represents a YouTube channel
type Channel struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Handle   string `json:"uploader_id,omitempty"`
	URL      string `json:"webpage_url"`
	Uploader string `json:"uploader,omitempty"`
}

// Video represents a YouTube video
type Video struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Duration   int    `json:"duration"` // in seconds
	URL        string `json:"webpage_url"`
	UploadDate string `json:"upload_date,omitempty"`
	Uploader   string `json:"uploader,omitempty"`
	UploaderID string `json:"uploader_id,omitempty"`
}

// Playlist represents a YouTube playlist
type Playlist struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"webpage_url"`
}

// PlaylistEntry represents a video in a playlist
type PlaylistEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Index int    `json:"playlist_index"`
}

// GetChannelInfo fetches channel metadata via yt-dlp
func (c *Client) GetChannelInfo(ctx context.Context, channelURL string) (*Channel, error) {
	data, err := extractJSON(ctx, channelURL)
	if err != nil {
		return nil, err
	}

	var ch Channel
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil, fmt.Errorf("failed to parse channel: %w", err)
	}

	return &ch, nil
}

// GetVideoInfo fetches video metadata via yt-dlp
func (c *Client) GetVideoInfo(ctx context.Context, videoURL string) (*Video, error) {
	data, err := extractJSON(ctx, videoURL)
	if err != nil {
		return nil, err
	}

	var v Video
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("failed to parse video: %w", err)
	}

	return &v, nil
}

// GetPlaylistInfo fetches playlist metadata via yt-dlp
func (c *Client) GetPlaylistInfo(ctx context.Context, playlistURL string) (*Playlist, error) {
	data, err := extractJSON(ctx, playlistURL)
	if err != nil {
		return nil, err
	}

	var p Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse playlist: %w", err)
	}

	return &p, nil
}

// GetPlaylistEntries fetches all videos in a playlist via yt-dlp
func (c *Client) GetPlaylistEntries(ctx context.Context, playlistURL string) ([]PlaylistEntry, error) {
	args := []string{
		playlistURL,
		"-J",
		"--flat-playlist",
		"--skip-unavailable-fragments",
	}
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var result struct {
		Entries []PlaylistEntry `json:"entries"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse playlist entries: %w", err)
	}

	return result.Entries, nil
}

// GetChannelVideos fetches all videos from a channel via yt-dlp
func (c *Client) GetChannelVideos(ctx context.Context, channelURL string) ([]Video, error) {
	args := []string{
		channelURL,
		"-J",
		"--flat-playlist",
		"yt-user",
		"--skip-unavailable-fragments",
	}
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	var result struct {
		Entries []Video `json:"entries"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse channel videos: %w", err)
	}

	return result.Entries, nil
}

// extractJSON runs yt-dlp with -J (JSON output) and returns raw JSON
func extractJSON(ctx context.Context, args ...string) ([]byte, error) {
	fullArgs := append(args, "-J")
	cmd := exec.CommandContext(ctx, "yt-dlp", fullArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w", err)
	}

	return stdout.Bytes(), nil
}
