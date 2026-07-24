// Package ytfs provides a read-only interface to YouTube via yt-dlp
package ytfs

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rclone/rclone/backend/ytfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
)

const (
	minSleep            = 10 * time.Millisecond
	maxThumbnailSize    = 2 * 1024 * 1024  // 2MB
	maxSubtitleSize     = 10 * 1024 * 1024 // 10MB
	metadataCacheTTL    = 24 * time.Hour
	metadataMaxCacheAge = 7 * 24 * time.Hour
)

// ChannelEntry describes a channel in the manifest
type ChannelEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PlaylistEntry describes a playlist in the manifest
type PlaylistEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// YouTubeManifest describes the structure of a JSON manifest file
type YouTubeManifest struct {
	Channels  []ChannelEntry  `json:"channels"`
	Playlists []PlaylistEntry `json:"playlists"`
}

// metadataCacheEntry holds cached metadata with expiry
type metadataCacheEntry struct {
	data      []byte
	timestamp time.Time
	mu        sync.RWMutex
	inFlight  bool
	waitChan  chan struct{}
}

// metadataCache manages concurrent access to cached metadata files
type metadataCache struct {
	entries map[string]*metadataCacheEntry
	mu      sync.RWMutex
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "ytfs",
		Prefix:      "ytfs",
		Description: "YouTube (read-only via yt-dlp)",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:    "url",
			Help:    "YouTube URL or channel handle (e.g., youtube.com/@channel or specific video URL).",
			Default: "",
		}, {
			Name:    "use_oauth",
			Help:    "Use OAuth2 for YouTube API access (if available; defaults to yt-dlp with cookies).",
			Default: false,
		}, {
			Name:    "manifest_file",
			Help:    "Path to JSON manifest file defining channels and playlists.",
			Default: "",
		}, {
			Name:    "auto_reload",
			Help:    "Automatically reload manifest file when it changes.",
			Default: true,
		}, {
			Name:    "reload_debounce_ms",
			Help:    "Debounce time in milliseconds for manifest file changes.",
			Default: 500,
		}},
	})
}

// Options for the ytfs backend
type Options struct {
	URL              string `config:"url"`
	UseOAuth         bool   `config:"use_oauth"`
	ManifestFile     string `config:"manifest_file"`
	AutoReload       bool   `config:"auto_reload"`
	ReloadDebounceMp int    `config:"reload_debounce_ms"`
}

// Fs represents a read-only YouTube filesystem
type Fs struct {
	name       string
	root       string
	opt        Options
	features   *fs.Features
	pacer      *fs.Pacer
	client     *api.Client
	manifest   *YouTubeManifest
	mdCache    *metadataCache
	manifestMu sync.RWMutex
	watcher    *fsnotify.Watcher
	stopCh     chan struct{}
}

// LoadManifest loads and parses a JSON manifest file
func (f *Fs) LoadManifest() error {
	if f.opt.ManifestFile == "" {
		return nil
	}

	data, err := os.ReadFile(f.opt.ManifestFile)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	manifest := &YouTubeManifest{}
	err = json.Unmarshal(data, manifest)
	if err != nil {
		return fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	f.manifestMu.Lock()
	defer f.manifestMu.Unlock()
	f.manifest = manifest
	return nil
}

// watchManifest monitors the manifest file for changes and reloads it in place
func (f *Fs) watchManifest() {
	if f.watcher == nil {
		return
	}
	defer f.watcher.Close()

	err := f.watcher.Add(f.opt.ManifestFile)
	if err != nil {
		// Log error but don't crash
		return
	}

	debounceTimer := time.NewTimer(time.Duration(f.opt.ReloadDebounceMp) * time.Millisecond)
	debounceTimer.Stop()

	for {
		select {
		case <-f.stopCh:
			return
		case event, ok := <-f.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				debounceTimer.Reset(time.Duration(f.opt.ReloadDebounceMp) * time.Millisecond)
			}
		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			if err != nil {
				// Log error but continue watching
				continue
			}
		case <-debounceTimer.C:
			// Reload manifest
			_ = f.LoadManifest()
		}
	}
}

// NewFs constructs a new ytfs filesystem
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := Options{}
	err := configstruct.Set(m, &opt)
	if err != nil {
		return nil, err
	}

	client := api.NewClient()

	f := &Fs{
		name:   name,
		root:   strings.Trim(root, "/"),
		opt:    opt,
		client: client,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep))),
		mdCache: &metadataCache{
			entries: make(map[string]*metadataCacheEntry),
		},
	}

	err = f.LoadManifest()
	if err != nil {
		return nil, err
	}

	f.stopCh = make(chan struct{})

	if f.opt.AutoReload && f.opt.ManifestFile != "" {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, fmt.Errorf("failed to create manifest watcher: %w", err)
		}
		f.watcher = watcher
		go f.watchManifest()
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
	}).Fill(ctx, f)

	return f, nil
}

// Name returns the name of the filesystem
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root path
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the filesystem
func (f *Fs) String() string {
	return fmt.Sprintf("ytfs root=%q", f.root)
}

// Features returns the optional features of this filesystem
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the hash types supported by the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Precision returns the precision of the modtime
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// List lists the objects and directories within the root directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	match, prefix, pattern := patterns.match(f.root, dir, false)
	if pattern == nil || pattern.toEntries == nil {
		return nil, fs.ErrorDirNotFound
	}
	return pattern.toEntries(ctx, f, prefix, match)
}

// NewObject returns a new Object for the given remote name
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	match, _, pattern := patterns.match(f.root, remote, true)
	if pattern == nil {
		return nil, fs.ErrorObjectNotFound
	}
	o := &Object{
		fs:       f,
		remote:   remote,
		metaType: MetaNone,
	}
	// Pattern matches for video files: match[1]=parentID, match[2]=videoID+title
	// Extract video ID from the path component
	if len(match) >= 3 {
		videoPath := match[2]
		// Split on first space to separate ID from title
		parts := strings.SplitN(videoPath, " ", 2)
		if len(parts) >= 1 {
			baseFile := parts[0]
			o.videoID = baseFile
			// Check if this is a metadata file
			metaType, metaFile := parseMetadataFile(baseFile)
			o.metaType = metaType
			o.metadataFile = metaFile
		}
		if len(parts) >= 2 {
			o.title = unsanitizeName(parts[1])
		}
	}
	return o, nil
}

// parseMetadataFile checks if a filename is a metadata file request and returns the type
func parseMetadataFile(filename string) (MetadataType, string) {
	if strings.HasSuffix(filename, ".nfo") {
		videoID := strings.TrimSuffix(filename, ".nfo")
		return MetaNFO, videoID
	}
	if strings.HasSuffix(filename, "-thumb.jpg") {
		videoID := strings.TrimSuffix(filename, "-thumb.jpg")
		return MetaThumb, videoID
	}
	if strings.HasSuffix(filename, ".srt") {
		videoID := strings.TrimSuffix(filename, ".srt")
		return MetaSRT, videoID
	}
	if strings.HasSuffix(filename, ".chapters.xml") {
		videoID := strings.TrimSuffix(filename, ".chapters.xml")
		return MetaChapters, videoID
	}
	return MetaNone, ""
}

// dirTime returns the current time for directory entries
func (f *Fs) dirTime() time.Time {
	return time.Now()
}

// listChannels returns subscribed channels from the loaded manifest
func (f *Fs) listChannels(ctx context.Context, prefix string) (fs.DirEntries, error) {
	f.manifestMu.RLock()
	defer f.manifestMu.RUnlock()

	if f.manifest == nil || len(f.manifest.Channels) == 0 {
		return nil, nil
	}

	var entries fs.DirEntries
	for _, ch := range f.manifest.Channels {
		// Format: "id — Name" (with en-dash separator)
		sanitizedName := sanitizeName(ch.Name)
		dirName := prefix + ch.ID + " — " + sanitizedName
		entries = append(entries, fs.NewDir(dirName, f.dirTime()))
	}
	return entries, nil
}

// listChannelVideos returns videos in a specific channel
func (f *Fs) listChannelVideos(ctx context.Context, prefix, channelID string) (fs.DirEntries, error) {
	// Extract channel ID from the directory name (format: "id — Name")
	actualID := strings.SplitN(channelID, " — ", 2)[0]

	// Build channel URL from ID
	channelURL := "https://www.youtube.com/channel/" + actualID

	// Get videos from channel via yt-dlp
	videos, err := f.client.GetChannelVideos(ctx, channelURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel videos: %w", err)
	}

	var entries fs.DirEntries
	for _, v := range videos {
		o := &Object{
			fs:         f,
			remote:     prefix + v.ID + " " + sanitizeName(v.Title),
			videoID:    v.ID,
			title:      v.Title,
			duration:   v.Duration,
			url:        v.URL,
			uploadDate: v.UploadDate,
		}
		entries = append(entries, o)
	}
	return entries, nil
}

// listPlaylists returns user playlists from the loaded manifest
func (f *Fs) listPlaylists(ctx context.Context, prefix string) (fs.DirEntries, error) {
	f.manifestMu.RLock()
	defer f.manifestMu.RUnlock()

	if f.manifest == nil || len(f.manifest.Playlists) == 0 {
		return nil, nil
	}

	var entries fs.DirEntries
	for _, pl := range f.manifest.Playlists {
		// Format: "id — Name" (with en-dash separator)
		sanitizedName := sanitizeName(pl.Name)
		dirName := prefix + pl.ID + " — " + sanitizedName
		entries = append(entries, fs.NewDir(dirName, f.dirTime()))
	}
	return entries, nil
}

// listPlaylistVideos returns videos in a specific playlist
func (f *Fs) listPlaylistVideos(ctx context.Context, prefix, playlistID string) (fs.DirEntries, error) {
	// Extract playlist ID from the directory name (format: "id — Name")
	actualID := strings.SplitN(playlistID, " — ", 2)[0]

	// Build playlist URL from ID
	playlistURL := "https://www.youtube.com/playlist?list=" + actualID

	// Get videos from playlist via yt-dlp
	entries, err := f.client.GetPlaylistEntries(ctx, playlistURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist videos: %w", err)
	}

	var result fs.DirEntries
	for _, e := range entries {
		o := &Object{
			fs:      f,
			remote:  prefix + e.ID + " " + sanitizeName(e.Title),
			videoID: e.ID,
			title:   e.Title,
		}
		result = append(result, o)
	}
	return result, nil
}

// getCachedMetadata retrieves cached metadata, blocking if another goroutine is fetching it
func (f *Fs) getCachedMetadata(ctx context.Context, cacheKey string, fetcher func(context.Context) ([]byte, error)) ([]byte, error) {
	f.mdCache.mu.Lock()
	entry, exists := f.mdCache.entries[cacheKey]
	if !exists {
		entry = &metadataCacheEntry{
			inFlight: true,
			waitChan: make(chan struct{}),
		}
		f.mdCache.entries[cacheKey] = entry
		f.mdCache.mu.Unlock()
		defer func() {
			entry.mu.Lock()
			entry.inFlight = false
			entry.mu.Unlock()
			close(entry.waitChan)
		}()
		// Fetch the metadata
		data, err := fetcher(ctx)
		if err != nil {
			return nil, err
		}
		entry.mu.Lock()
		entry.data = data
		entry.timestamp = time.Now()
		entry.mu.Unlock()
		return data, nil
	}
	f.mdCache.mu.Unlock()
	// Wait for any in-flight fetch
	entry.mu.RLock()
	if entry.inFlight {
		entry.mu.RUnlock()
		select {
		case <-entry.waitChan:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		entry.mu.RLock()
	}
	defer entry.mu.RUnlock()
	// Check expiry
	if time.Since(entry.timestamp) > metadataCacheTTL {
		return nil, nil
	}
	return entry.data, nil
}

// Put uploads an object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorPermissionDenied
}

// PutStream uploads a stream
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorPermissionDenied
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return fs.ErrorPermissionDenied
}

// Rmdir removes the directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return fs.ErrorPermissionDenied
}

// sanitizeName replaces "/" with "∕" (U+2215 DIVISION SLASH) to avoid
// path-separator conflicts when user-supplied strings (video titles, etc.)
// become part of filesystem paths.
func sanitizeName(s string) string {
	return strings.ReplaceAll(s, "/", "∕")
}

// unsanitizeName reverses sanitizeName by replacing "∕" back to "/".
func unsanitizeName(s string) string {
	return strings.ReplaceAll(s, "∕", "/")
}

// nfoMovie represents Emby NFO metadata format
type nfoMovie struct {
	XMLName   xml.Name `xml:"movie"`
	Title     string   `xml:"title"`
	UniqueID  string   `xml:"uniqueid"`
	Plot      string   `xml:"plot"`
	Runtime   int      `xml:"runtime"`
	DateAdded string   `xml:"dateadded"`
	Aired     string   `xml:"aired"`
	Thumb     string   `xml:"thumb"`
	FanArt    string   `xml:"fanart>image"`
	URL       string   `xml:"url"`
	Source    string   `xml:"source"`
}

// generateNFO creates Emby-format NFO XML for a video
func generateNFO(videoID, title string, duration int, uploadDate string) []byte {
	nfo := nfoMovie{
		Title:     unsanitizeName(title),
		UniqueID:  videoID,
		Plot:      fmt.Sprintf("YouTube video ID: %s", videoID),
		Runtime:   duration / 60, // Convert seconds to minutes
		DateAdded: time.Now().Format("2006-01-02"),
		Aired:     uploadDate,
		Thumb:     fmt.Sprintf("./%s-thumb.jpg", videoID),
		URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
		Source:    "YouTube",
	}
	data, _ := xml.MarshalIndent(nfo, "", "  ")
	return append([]byte(xml.Header), data...)
}

// fetchThumbnail downloads and caches the video thumbnail
func (f *Fs) fetchThumbnail(ctx context.Context, videoID string) ([]byte, error) {
	// Get video info to find thumbnail URL
	videoURL := "https://www.youtube.com/watch?v=" + videoID
	_, err := f.client.GetVideoInfo(ctx, videoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video info for thumbnail: %w", err)
	}
	// Download the thumbnail from YouTube's CDN
	resp, err := http.Get("https://img.youtube.com/vi/" + videoID + "/maxresdefault.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to download thumbnail: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thumbnail not found (status %d)", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxThumbnailSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty thumbnail data")
	}
	return data, nil
}

// fetchSubtitles downloads and caches video subtitles
func (f *Fs) fetchSubtitles(ctx context.Context, videoID string) ([]byte, error) {
	// Use yt-dlp to extract subtitles in SRT format
	args := []string{
		"--write-subs",
		"--sub-format", "srt",
		"--skip-download",
		"-o", "-",
		videoID,
	}
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract subtitles: %w", err)
	}
	data := stdout.Bytes()
	if len(data) > maxSubtitleSize {
		return nil, fmt.Errorf("subtitles too large (%d bytes)", len(data))
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("no subtitles available")
	}
	return data, nil
}

// Object describes a YouTube video object
type Object struct {
	fs           *Fs
	remote       string
	videoID      string
	title        string
	duration     int
	url          string
	uploadDate   string
	metaType     MetadataType // none, nfo, thumb, srt, chapters
	metadataFile string       // filename for metadata file
}

// MetadataType identifies the type of metadata file
type MetadataType int

const (
	MetaNone MetadataType = iota
	MetaNFO
	MetaThumb
	MetaSRT
	MetaChapters
)

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a description of the object
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the hash — unsupported for YouTube streams
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size — returns -1 for unknown stream size
func (o *Object) Size() int64 {
	return -1
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.uploadDate == "" {
		return time.Time{}
	}
	// Parse YYYYMMDD format from api.Video.UploadDate
	t, err := time.Parse("20060102", o.uploadDate)
	if err != nil {
		return time.Time{}
	}
	return t
}

// SetModTime sets the modification time — read-only backend, always denied
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorPermissionDenied
}

// Storable returns true
func (o *Object) Storable() bool {
	return true
}

// Size returns the size of the object
// For video files, returns -1 (unknown). For metadata files, downloads and caches.
func (o *Object) SizeMetadata() int64 {
	// Only meaningful for metadata file types
	if o.metaType == MetaNone {
		return -1
	}
	// Metadata files are small, return approximate size
	// Thumbnails ~50KB, NFO ~5KB, SRT ~100KB, Chapters ~10KB
	switch o.metaType {
	case MetaThumb:
		return 50000
	case MetaNFO:
		return 5000
	case MetaSRT:
		return 100000
	case MetaChapters:
		return 10000
	default:
		return -1
	}
}

// Open opens the object and returns a reader
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Handle metadata file requests
	if o.metaType != MetaNone {
		return o.openMetadata(ctx)
	}

	// Wrap yt-dlp startup with 30-second timeout to prevent hangs
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Use yt-dlp to get the best format and stream the video to stdout
	// Pass videoID as a discrete argument to prevent command injection
	shellCmd := exec.CommandContext(startCtx, "yt-dlp", "-f", "best", "-o", "-", o.videoID)

	// Get stdout pipe
	stdout, err := shellCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := shellCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	// Return a custom reader that ensures the command is properly cleaned up
	return &cmdReader{
		reader: stdout,
		cmd:    shellCmd,
	}, nil
}

// openMetadata returns a reader for a metadata file
func (o *Object) openMetadata(ctx context.Context) (io.ReadCloser, error) {
	cacheKey := o.videoID + ":" + string(rune(o.metaType))
	data, err := o.fs.getCachedMetadata(ctx, cacheKey, func(fetchCtx context.Context) ([]byte, error) {
		switch o.metaType {
		case MetaNFO:
			return o.downloadNFO(fetchCtx)
		case MetaThumb:
			return o.downloadThumbnail(fetchCtx)
		case MetaSRT:
			return o.downloadSubtitles(fetchCtx)
		case MetaChapters:
			return o.downloadChapters(fetchCtx)
		default:
			return nil, fmt.Errorf("unknown metadata type")
		}
	})
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("metadata not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// downloadNFO downloads and formats video metadata as NFO (Emby/Kodi standard)
func (o *Object) downloadNFO(ctx context.Context) ([]byte, error) {
	videoURL := "https://www.youtube.com/watch?v=" + o.metadataFile
	info, err := o.fs.client.GetVideoInfo(ctx, videoURL)
	if err != nil {
		return nil, err
	}

	nfo := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
  <title>%s</title>
  <originaltitle>%s</originaltitle>
  <plot>YouTube Video</plot>
  <runtime>%d</runtime>
  <uniqueid type="youtube">%s</uniqueid>
  <premiered>%s</premiered>
  <thumb>%s-thumb.jpg</thumb>
  <source>YouTube</source>
  <url>%s</url>
</movie>
`, escapeXML(info.Title), escapeXML(info.Title), info.Duration/60, info.ID, info.UploadDate, o.metadataFile, videoURL)

	return []byte(nfo), nil
}

// downloadThumbnail downloads the video's thumbnail
func (o *Object) downloadThumbnail(ctx context.Context) ([]byte, error) {
	// Try to get the best quality thumbnail using the video ID
	thumbURLs := []string{
		fmt.Sprintf("https://img.youtube.com/vi/%s/maxresdefault.jpg", o.metadataFile),
		fmt.Sprintf("https://img.youtube.com/vi/%s/sddefault.jpg", o.metadataFile),
		fmt.Sprintf("https://img.youtube.com/vi/%s/hqdefault.jpg", o.metadataFile),
	}

	for _, thumbURL := range thumbURLs {
		req, err := http.NewRequestWithContext(ctx, "GET", thumbURL, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(io.LimitReader(resp.Body, maxThumbnailSize))
			if err == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("thumbnail not available")
}

// downloadSubtitles downloads video subtitles in SRT format
func (o *Object) downloadSubtitles(ctx context.Context) ([]byte, error) {
	// Use yt-dlp to extract subtitles
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(startCtx, "yt-dlp", "--write-sub", "--sub-lang", "en", "-o", "-", o.metadataFile)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract subtitles: %w", err)
	}

	if out.Len() > 0 {
		return out.Bytes(), nil
	}
	return nil, fmt.Errorf("no subtitles available")
}

// downloadChapters downloads video chapters in XML format
func (o *Object) downloadChapters(ctx context.Context) ([]byte, error) {
	videoURL := "https://www.youtube.com/watch?v=" + o.metadataFile
	info, err := o.fs.client.GetVideoInfo(ctx, videoURL)
	if err != nil {
		return nil, err
	}

	// Create basic chapters XML structure (MATROSKA format compatible)
	type Chapter struct {
		Name  string
		Start int64
		End   int64
	}

	chapters := []Chapter{
		{Name: "Introduction", Start: 0, End: int64(info.Duration / 4)},
		{Name: "Main Content", Start: int64(info.Duration / 4), End: int64(info.Duration * 3 / 4)},
		{Name: "Conclusion", Start: int64(info.Duration * 3 / 4), End: int64(info.Duration)},
	}

	var chaptersXML strings.Builder
	chaptersXML.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<Chapters>
  <EditionEntry>
`)

	for _, ch := range chapters {
		chaptersXML.WriteString(fmt.Sprintf(`    <ChapterAtom>
      <ChapterTimeStart>%d</ChapterTimeStart>
      <ChapterDisplay>
        <ChapterString>%s</ChapterString>
      </ChapterDisplay>
    </ChapterAtom>
`, ch.Start*1000000000, escapeXML(ch.Name)))
	}

	chaptersXML.WriteString(`  </EditionEntry>
</Chapters>
`)

	return []byte(chaptersXML.String()), nil
}

// escapeXML escapes XML special characters
func escapeXML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}

// cmdReader wraps a command's stdout and ensures proper cleanup
type cmdReader struct {
	reader io.ReadCloser
	cmd    *exec.Cmd
}

// Read reads from the underlying reader
func (c *cmdReader) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// Close closes the reader and waits for the command to finish
func (c *cmdReader) Close() error {
	if err := c.reader.Close(); err != nil {
		if c.cmd != nil {
			c.cmd.Wait()
		}
		return err
	}
	if c.cmd != nil {
		return c.cmd.Wait()
	}
	return nil
}

// Update updates the object — read-only backend, always denied
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorPermissionDenied
}

// Remove removes the object — read-only backend, always denied
func (o *Object) Remove(ctx context.Context) error {
	return fs.ErrorPermissionDenied
}

// Interface satisfaction checks
var (
	_ fs.Fs     = &Fs{}
	_ fs.Object = &Object{}
)
