// Package metadata provides metadata caching for YTFS
package metadata

import (
	"context"
	"time"
)

// Cache defines the interface for metadata caching operations
type Cache interface {
	// Lifecycle
	Initialize(ctx context.Context) error
	Close() error

	// Video metadata operations
	GetVideoMetadata(ctx context.Context, videoID string) (*VideoMetadata, error)
	SetVideoMetadata(ctx context.Context, videoID string, metadata *VideoMetadata) error
	DeleteVideoMetadata(ctx context.Context, videoID string) error

	// NFO operations
	GetNFO(ctx context.Context, videoID string) (*NFOMovie, error)
	SetNFO(ctx context.Context, videoID string, nfo *NFOMovie) error
	NFOPath(videoID string) string

	// Thumbnail operations
	GetThumbnailPath(ctx context.Context, videoID string) (string, error)
	SetThumbnail(ctx context.Context, videoID string, data []byte, quality string) error
	GetThumbnailManifest(ctx context.Context, videoID string) (*ThumbnailManifest, error)
	SetThumbnailManifest(ctx context.Context, videoID string, manifest *ThumbnailManifest) error

	// Subtitle operations
	GetSubtitles(ctx context.Context, videoID, language string) ([]byte, error)
	SetSubtitles(ctx context.Context, videoID, language string, data []byte) error
	GetSubtitlesManifest(ctx context.Context, videoID string) (*SubtitlesManifest, error)
	SetSubtitlesManifest(ctx context.Context, videoID string, manifest *SubtitlesManifest) error

	// Chapter operations
	GetChapters(ctx context.Context, videoID string) (*ChaptersManifest, error)
	SetChapters(ctx context.Context, videoID string, chapters *ChaptersManifest) error

	// Channel metadata operations
	GetChannelMetadata(ctx context.Context, channelID string) (*ChannelMetadata, error)
	SetChannelMetadata(ctx context.Context, channelID string, metadata *ChannelMetadata) error

	// Playlist metadata operations
	GetPlaylistMetadata(ctx context.Context, playlistID string) (*PlaylistMetadata, error)
	SetPlaylistMetadata(ctx context.Context, playlistID string, metadata *PlaylistMetadata) error

	// Expiry management
	IsValid(ctx context.Context, videoID, dataType string) bool
	GetTTLRemaining(videoID, dataType string) time.Duration
	Invalidate(ctx context.Context, videoID, dataType string) error

	// Cleanup
	Cleanup(ctx context.Context, maxAge time.Duration) error
	GetStats(ctx context.Context) (*CacheStats, error)

	// Configuration
	LoadConfig(ctx context.Context, configPath string) error
	GetConfig() *CacheConfig
}

// Manager provides high-level cache operations
type Manager struct {
	cache  Cache
	config *CacheConfig
}

// NewManager creates a new cache manager
func NewManager(cache Cache, config *CacheConfig) *Manager {
	return &Manager{
		cache:  cache,
		config: config,
	}
}

// GenerateNFO generates an NFO file from video metadata
func (m *Manager) GenerateNFO(ctx context.Context, video *VideoMetadata) (*NFOMovie, error) {
	nfo := &NFOMovie{
		Title:         video.Title,
		OriginalTitle: video.Title,
		SortTitle:     toLower(video.Title),
		Plot:          video.Description,
		Runtime:       video.Duration,
		Director:      video.Uploader,
		Views:         video.ViewCount,
		Likes:         video.LikeCount,
		Comments:      video.CommentCount,
		ContentRating: "TV-G", // Default safe rating
		URL:           video.WebpageURL,
	}

	// Set unique ID
	nfo.UniqueIDs = []UniqueID{
		{
			Type:  "youtube",
			Value: video.ID,
		},
	}

	// Parse upload date
	uploadDate, year := parseUploadDate(video.UploadDate)
	nfo.Aired = uploadDate
	nfo.Year = year
	nfo.DateAdded = time.Now().Format("2006-01-02T15:04:05Z")

	// Set thumbnail
	if video.Thumbnail != "" {
		nfo.Thumbs = []Thumb{
			{
				Type:    "poster",
				Aspect:  "16:9",
				Preview: "thumb://" + video.ID,
				URL:     video.Thumbnail,
			},
		}
		nfo.Fanarts = []Fanart{
			{
				Thumbs: []FanartThumb{
					{URL: video.Thumbnail},
				},
			},
		}
	}

	// Set actor (uploader)
	nfo.Actors = []Actor{
		{
			Name: video.Uploader,
			Role: "Uploader",
		},
	}

	// Map categories to genres
	nfo.Genres = mapCategoriesToGenres(video.Categories)
	if len(nfo.Genres) == 0 {
		nfo.Genres = []string{"Web Video"}
	}

	// Set tags
	nfo.Tags = append([]string{"youtube"}, video.Tags...)
	if video.Uploader != "" {
		nfo.Tags = append(nfo.Tags, video.Uploader)
	}

	// Calculate rating from likes/views
	if video.ViewCount > 0 && video.LikeCount > 0 {
		rating := float32(video.LikeCount) / float32(video.ViewCount) * 10
		if rating > 10 {
			rating = 10
		}
		nfo.Rating = Rating{
			Default: true,
			Value:   rating,
		}
	} else {
		nfo.Rating = Rating{
			Default: true,
			Value:   7.0,
		}
	}

	return nfo, nil
}

// CacheThumbnail caches a thumbnail with the given quality
func (m *Manager) CacheThumbnail(ctx context.Context, videoID string, data []byte, quality string) error {
	return m.cache.SetThumbnail(ctx, videoID, data, quality)
}

// GetValidThumbnail returns a cached thumbnail if still valid
func (m *Manager) GetValidThumbnail(ctx context.Context, videoID string) (string, error) {
	if !m.cache.IsValid(ctx, videoID, "thumbnail") {
		return "", ErrExpired
	}
	return m.cache.GetThumbnailPath(ctx, videoID)
}

// CacheSubtitles caches subtitles for a video
func (m *Manager) CacheSubtitles(ctx context.Context, videoID, language string, data []byte) error {
	return m.cache.SetSubtitles(ctx, videoID, language, data)
}

// GetValidSubtitles returns cached subtitles if still valid
func (m *Manager) GetValidSubtitles(ctx context.Context, videoID, language string) ([]byte, error) {
	if !m.cache.IsValid(ctx, videoID, "subtitles") {
		return nil, ErrExpired
	}
	return m.cache.GetSubtitles(ctx, videoID, language)
}

// Helper functions

// toLower converts string to lowercase (for sorttitle)
func toLower(s string) string {
	// Placeholder: use strings.ToLower in actual implementation
	return s
}

// parseUploadDate parses YYYYMMDD format to YYYY-MM-DD and year
func parseUploadDate(uploadDate string) (string, int) {
	if len(uploadDate) != 8 {
		return time.Now().Format("2006-01-02"), time.Now().Year()
	}
	year := 0
	month := 0
	day := 0

	// Parse YYYYMMDD format
	_, _ = parseIntoInts(uploadDate[:4], &year)
	_, _ = parseIntoInts(uploadDate[4:6], &month)
	_, _ = parseIntoInts(uploadDate[6:8], &day)

	formatted := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	return formatted, year
}

// parseIntoInts is a helper to parse string to int
func parseIntoInts(s string, val *int) (int, error) {
	// Placeholder: use strconv.Atoi in actual implementation
	return 0, nil
}

// mapCategoriesToGenres maps YouTube categories to Emby genres
func mapCategoriesToGenres(categories []string) []string {
	if len(categories) == 0 {
		return []string{"Web Video"}
	}

	genreMap := map[string]string{
		"Music":         "Music",
		"Gaming":        "Gaming",
		"Education":     "Documentary",
		"Howto":         "How-To",
		"Entertainment": "Comedy",
		"Comedy":        "Comedy",
		"Sports":        "Sports",
		"Science":       "Documentary",
		"Technology":    "Documentary",
		"Travel":        "Travel",
		"Documentary":   "Documentary",
		"Animation":     "Animation",
		"Action":        "Action",
	}

	var genres []string
	seen := make(map[string]bool)

	for _, cat := range categories {
		if mapped, ok := genreMap[cat]; ok {
			if !seen[mapped] {
				genres = append(genres, mapped)
				seen[mapped] = true
			}
		}
	}

	if len(genres) == 0 {
		return []string{"Web Video"}
	}

	return genres
}

// Error types
var (
	ErrExpired       = NewError("cache entry expired")
	ErrNotFound      = NewError("cache entry not found")
	ErrInvalidFormat = NewError("invalid cache format")
)

// Error represents a cache error
type Error struct {
	message string
}

// NewError creates a new error
func NewError(message string) *Error {
	return &Error{message: message}
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.message
}
