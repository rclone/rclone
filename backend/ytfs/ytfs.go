// Package ytfs provides a read-only interface to YouTube via yt-dlp
package ytfs

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/ytfs/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
)

const (
	minSleep = 10 * time.Millisecond
)

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
		}},
	})
}

// Options for the ytfs backend
type Options struct {
	URL      string `config:"url"`
	UseOAuth bool   `config:"use_oauth"`
}

// Fs represents a read-only YouTube filesystem
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	pacer    *fs.Pacer
	client   *api.Client
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
		fs:     f,
		remote: remote,
	}
	// Pattern matches for video files: match[1]=parentID, match[2]=videoID+title
	// Extract video ID from the path component
	if len(match) >= 3 {
		videoPath := match[2]
		// Split on first space to separate ID from title
		parts := strings.SplitN(videoPath, " ", 2)
		if len(parts) >= 1 {
			o.videoID = parts[0]
		}
		if len(parts) >= 2 {
			o.title = unsanitizeName(parts[1])
		}
	}
	return o, nil
}

// dirTime returns the current time for directory entries
func (f *Fs) dirTime() time.Time {
	return time.Now()
}

// listChannels returns subscribed channels (stub — not yet implemented)
// Full channel enumeration requires YouTube API authentication or advanced yt-dlp configuration.
// MVP implementation returns empty list; future versions will fetch from YouTube API or yt-dlp.
func (f *Fs) listChannels(ctx context.Context, prefix string) (fs.DirEntries, error) {
	return nil, nil
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

// listPlaylists returns user playlists (stub — not yet implemented)
// Full playlist enumeration requires YouTube API authentication or advanced yt-dlp configuration.
// MVP implementation returns empty list; future versions will fetch from YouTube API or yt-dlp.
func (f *Fs) listPlaylists(ctx context.Context, prefix string) (fs.DirEntries, error) {
	return nil, nil
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

// Object describes a YouTube video object
type Object struct {
	fs       *Fs
	remote   string
	videoID  string
	title    string
	duration int
	url      string
	uploadDate string
}

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

// Open opens the object and returns a reader for the video stream
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
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
