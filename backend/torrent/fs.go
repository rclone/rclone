// Package torrent provides a read-only remote for accessing torrent files
package torrent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/fsnotify/fsnotify"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"golang.org/x/time/rate"
)

// Register with rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "torrent",
		Description: "Read-only torrent backend for accessing torrent contents",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "root_directory",
			Help:     "Local directory containing torrent files.",
			Required: true,
		}, {
			Name:    "max_download_speed",
			Help:    "Maximum download speed (kBytes/s).",
			Default: 0,
		}, {
			Name:    "max_upload_speed",
			Help:    "Maximum upload speed (kBytes/s).",
			Default: 0,
		}, {
			Name:     "piece_read_ahead",
			Help:     "Number of pieces to read ahead.",
			Default:  5,
			Advanced: true,
		}, {
			Name:     "cleanup_timeout",
			Help:     "Remove inactive torrents after this many minutes (0 to disable).",
			Default:  0,
			Advanced: true,
		}, {
			Name:     "cache_dir",
			Help:     "Directory to store downloaded torrent data.",
			Default:  "",
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	RootDirectory    string `config:"root_directory"`
	MaxDownloadSpeed int    `config:"max_download_speed"`
	MaxUploadSpeed   int    `config:"max_upload_speed"`
	PieceReadAhead   int    `config:"piece_read_ahead"`
	CleanupTimeout   int    `config:"cleanup_timeout"`
	CacheDir         string `config:"cache_dir"`
}

// Fs represents a torrent remote
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	client   *torrent.Client
	watcher  *fsnotify.Watcher

	mu       sync.RWMutex
	torrents map[string]*TorrentEntry
}

// TorrentEntry represents a torrent and its filesystem
type TorrentEntry struct {
	name     string           // original torrent filename
	torrent  *torrent.Torrent // underlying torrent
	accessed time.Time        // last access time
	rootPath string           // virtual root path
}

// getCacheDir returns the directory to store downloaded torrent data
func getCacheDir(opt Options) string {
	if opt.CacheDir != "" {
		return opt.CacheDir
	}
	return filepath.Join(os.TempDir(), "rclone-torrent-cache")
}

// NewFs constructs a new filesystem
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	// Create client config
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = getCacheDir(*opt)

	// Set bandwidth limits
	if opt.MaxDownloadSpeed > 0 {
		cfg.DownloadRateLimiter = rate.NewLimiter(rate.Limit(opt.MaxDownloadSpeed*1024), 256*1024)
	}
	if opt.MaxUploadSpeed > 0 {
		cfg.UploadRateLimiter = rate.NewLimiter(rate.Limit(opt.MaxUploadSpeed*1024), 256*1024)
	}

	// Create client
	client, err := torrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create torrent client: %w", err)
	}

	f := &Fs{
		name:     name,
		root:     root,
		opt:      *opt,
		client:   client,
		torrents: make(map[string]*TorrentEntry),
	}

	// Set features
	f.features = (&fs.Features{
		CaseInsensitive:         false,
		DuplicateFiles:          false,
		ReadMimeType:            false,
		WriteMimeType:           false,
		CanHaveEmptyDirectories: true,
		BucketBased:             false,
	}).Fill(ctx, f)

	// Setup watcher and perform initial scan
	if err := f.setupWatcher(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to setup watcher: %w", err)
	}

	if err := f.initialScan(); err != nil {
		f.watcher.Close()
		client.Close()
		return nil, fmt.Errorf("failed to scan root: %w", err)
	}

	// Start cleanup if enabled
	if opt.CleanupTimeout > 0 {
		go f.cleanupLoop(ctx)
	}

	return f, nil
}

// Required interface implementations for fs.Fs
func (f *Fs) String() string           { return fmt.Sprintf("torrent root '%s'", f.root) }
func (f *Fs) Features() *fs.Features   { return f.features }
func (f *Fs) Name() string             { return f.name }
func (f *Fs) Root() string             { return f.root }
func (f *Fs) Precision() time.Duration { return time.Second }
func (f *Fs) Hashes() hash.Set         { return hash.Set(hash.None) }

// Read-only filesystem methods
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, fs.ErrorPermissionDenied
}
func (f *Fs) Mkdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }
func (f *Fs) Rmdir(ctx context.Context, dir string) error { return fs.ErrorPermissionDenied }

// setupWatcher creates and configures the filesystem watcher
func (f *Fs) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	f.watcher = watcher

	// Watch root directory and all subdirectories
	if err := filepath.Walk(f.opt.RootDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		return watcher.Add(path)
	}); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to setup directory watching: %w", err)
	}

	go f.watcherLoop()
	return nil
}

// watcherLoop handles filesystem events
func (f *Fs) watcherLoop() {
	for {
		select {
		case event, ok := <-f.watcher.Events:
			if !ok {
				return
			}

			// Handle new directories
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					_ = f.watcher.Add(event.Name)
				}
			}

			// Handle torrent files
			if isTorrentFile(event.Name) {
				switch {
				case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
					if err := f.handleNewTorrent(event.Name); err != nil {
						fs.Errorf(f, "Failed to handle new/modified torrent %s: %v", event.Name, err)
					}
				case event.Op&(fsnotify.Remove|fsnotify.Rename) != 0:
					f.handleRemovedTorrent(event.Name)
				}
			}

		case err, ok := <-f.watcher.Errors:
			if !ok {
				return
			}
			fs.Errorf(f, "Watcher error: %v", err)
		}
	}
}

// initialScan performs the initial scan of the root directory
func (f *Fs) initialScan() error {
	return filepath.Walk(f.opt.RootDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isTorrentFile(path) {
			return nil
		}
		if err := f.handleNewTorrent(path); err != nil {
			fs.Errorf(f, "Failed to handle torrent %s during initial scan: %v", path, err)
		}
		return nil
	})
}

// handleNewTorrent processes a new or modified torrent file
func (f *Fs) handleNewTorrent(path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Remove existing torrent if present
	if existing, ok := f.torrents[path]; ok {
		existing.torrent.Drop()
		delete(f.torrents, path)
	}

	// Add new torrent
	t, err := f.client.AddTorrentFromFile(path)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}

	// Don't start downloading yet
	t.DisallowDataDownload()

	// Wait for metadata
	select {
	case <-t.GotInfo():
	case <-time.After(10 * time.Second):
		t.Drop()
		return fmt.Errorf("timeout waiting for torrent metadata")
	}

	// Get relative path and determine root path
	relPath, err := filepath.Rel(f.opt.RootDirectory, filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}
	if relPath == "." {
		relPath = ""
	}

	files := t.Files()
	if len(files) == 0 {
		t.Drop()
		return fmt.Errorf("torrent contains no files")
	}

	// Create virtual root path
	rootPath := f.createVirtualRootPath(relPath, path, files)

	// Create entry
	entry := &TorrentEntry{
		name:     filepath.Base(path),
		torrent:  t,
		accessed: time.Now(),
		rootPath: rootPath,
	}

	f.torrents[path] = entry
	fs.Infof(f, "Added torrent: %s at virtual path: %s", entry.name, entry.rootPath)
	return nil
}

// createVirtualRootPath determines the virtual root path for a torrent
func (f *Fs) createVirtualRootPath(relPath, torrentPath string, files []*torrent.File) string {
	torrentBase := strings.TrimSuffix(filepath.Base(torrentPath), ".torrent")

	if len(files) == 1 {
		// Single file torrent - always place in subfolder
		return filepath.Join(relPath, torrentBase)
	}

	// Check for common root in multi-file torrent
	firstPath := files[0].Path()
	parts := strings.Split(firstPath, "/")
	if len(parts) > 1 {
		commonRoot := parts[0]
		hasCommonRoot := true

		for _, file := range files[1:] {
			parts := strings.Split(file.Path(), "/")
			if len(parts) <= 1 || parts[0] != commonRoot {
				hasCommonRoot = false
				break
			}
		}

		if hasCommonRoot {
			return relPath // Use parent directory as-is
		}
	}

	return filepath.Join(relPath, torrentBase) // Create new root named after torrent
}

// handleRemovedTorrent handles removal of a torrent file
func (f *Fs) handleRemovedTorrent(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if entry, ok := f.torrents[path]; ok {
		fs.Infof(f, "Removing torrent: %s from virtual path: %s", entry.name, entry.rootPath)
		entry.torrent.Drop()
		delete(f.torrents, path)
	}
}

// List implements fs.Fs
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	seen := make(map[string]bool)
	entries = make(fs.DirEntries, 0)

	for _, entry := range f.torrents {
		torrentEntries, err := f.listTorrentContents(entry, dir)
		if err != nil {
			continue
		}

		// Add unique entries
		for _, e := range torrentEntries {
			if !seen[e.Remote()] {
				entries = append(entries, e)
				seen[e.Remote()] = true
			}
		}
	}

	return entries, nil
}

// listTorrentContents lists the contents of a specific torrent
func (f *Fs) listTorrentContents(entry *TorrentEntry, dir string) (fs.DirEntries, error) {
	info := entry.torrent.Info()
	if info == nil {
		select {
		case <-entry.torrent.GotInfo():
			info = entry.torrent.Info()
		case <-time.After(10 * time.Second):
			return nil, fmt.Errorf("timeout waiting for torrent info")
		}
	}

	entries := make(fs.DirEntries, 0)
	seenDirs := make(map[string]bool)
	files := entry.torrent.Files()
	isSingleFile := len(files) == 1

	for _, file := range files {
		filePath := f.getVirtualFilePath(entry, file.Path(), isSingleFile)

		// Get path relative to requested directory
		relPath, err := filepath.Rel(dir, filePath)
		if err != nil || strings.HasPrefix(relPath, "..") {
			continue
		}

		parts := strings.Split(relPath, string(filepath.Separator))

		if len(parts) == 1 {
			// Direct file in current directory
			obj := &Object{
				fs:          f,
				entry:       entry,
				virtualPath: filePath,
				torrentPath: file.Path(),
				size:        file.Length(),
				modTime:     entry.accessed,
			}
			entries = append(entries, obj)
		} else {
			// Add directory entry for first component if not seen
			dirName := parts[0]
			if !seenDirs[dirName] {
				dir := fs.NewDir(
					filepath.Join(dir, dirName),
					entry.accessed,
				)
				entries = append(entries, dir)
				seenDirs[dirName] = true
			}
		}
	}

	return entries, nil
}

// getVirtualFilePath returns the virtual file path for a torrent file
func (f *Fs) getVirtualFilePath(entry *TorrentEntry, torrentPath string, isSingleFile bool) string {
	if isSingleFile {
		return filepath.Join(entry.rootPath, filepath.Base(torrentPath))
	}
	return filepath.Join(entry.rootPath, torrentPath)
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Search all torrents for the file
	for _, entry := range f.torrents {
		info := entry.torrent.Info()
		if info == nil {
			continue
		}

		for _, file := range entry.torrent.Files() {
			virtualPath := filepath.Join(entry.rootPath, file.Path())
			if virtualPath == remote {
				return &Object{
					fs:          f,
					entry:       entry,
					virtualPath: virtualPath,
					torrentPath: file.Path(),
					size:        file.Length(),
					modTime:     entry.accessed,
				}, nil
			}
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// cleanupLoop periodically removes inactive torrents
func (f *Fs) cleanupLoop(ctx context.Context) {
	if f.opt.CleanupTimeout <= 0 {
		return
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.cleanup()
		}
	}
}

// cleanup removes inactive torrents
func (f *Fs) cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	timeout := time.Duration(f.opt.CleanupTimeout) * time.Minute
	now := time.Now()

	for path, entry := range f.torrents {
		if now.Sub(entry.accessed) > timeout {
			fs.Debugf(f, "Removing inactive torrent: %s", entry.name)
			entry.torrent.Drop()
			delete(f.torrents, path)
		}
	}
}

// Shutdown closes the backend
func (f *Fs) Shutdown(ctx context.Context) error {
	if f.watcher != nil {
		f.watcher.Close()
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, entry := range f.torrents {
		entry.torrent.Drop()
	}

	if f.client != nil {
		f.client.Close()
	}

	return nil
}

// isTorrentFile returns true if the filename has a .torrent extension
func isTorrentFile(path string) bool {
	return strings.ToLower(filepath.Ext(path)) == ".torrent"
}

// Check interfaces
var (
	_ fs.Fs         = (*Fs)(nil)
	_ fs.Shutdowner = (*Fs)(nil)
)
