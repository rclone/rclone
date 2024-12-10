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
    "github.com/anacrolix/torrent/metainfo"
    "github.com/rclone/rclone/fs"
    "github.com/rclone/rclone/fs/config/configmap"
    "github.com/rclone/rclone/fs/config/configstruct"
    "github.com/rclone/rclone/fs/hash"
    "golang.org/x/time/rate"
)

const (
    defaultCleanupTimeout = 0
    defaultHandshakeTimeout = 30 * time.Second
    maxConnectionsPerTorrent = 50
    defaultPendingPeers = 25
)

var (
    // Registry info for this backend
    fsInfo = &fs.RegInfo{
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
            Name:     "cache_dir",
            Help:     "Directory to store downloaded torrent data.",
            Default:  "",
            Advanced: true,
        }, {
            Name:     "cleanup_timeout",
            Help:     "Remove inactive torrents after X minutes (0 to disable).",
            Default:  defaultCleanupTimeout,
            Advanced: true,
        }},
    }
)

func init() {
    fs.Register(fsInfo)
}

type Options struct {
    RootDirectory    string `config:"root_directory"`
    MaxDownloadSpeed int    `config:"max_download_speed"`
    MaxUploadSpeed   int    `config:"max_upload_speed"`
    CacheDir         string `config:"cache_dir"`
    CleanupTimeout   int    `config:"cleanup_timeout"`
}

// Fs implements a read-only torrent filesystem
type Fs struct {
    name     string
    root     string
    opt      Options
    features *fs.Features
    client   *torrent.Client
    baseFs   fs.Fs

    // Track active torrents with concurrent map
    activeTorrents sync.Map // map[string]*torrentInfo
}

// torrentInfo tracks metadata for active torrents
type torrentInfo struct {
    torrent    *torrent.Torrent
    lastAccess time.Time
    mu         sync.RWMutex  // Protects lastAccess
}

// Directory represents a virtual directory in the torrent filesystem
type Directory struct {
    fs      *Fs
    remote  string
    modTime time.Time
    size    int64
    items   int64
}

// Common directory interface implementations
func (d *Directory) String() string                       { return d.remote }
func (d *Directory) Remote() string                       { return d.remote }
func (d *Directory) ModTime(context.Context) time.Time    { return d.modTime }
func (d *Directory) Size() int64                          { return d.size }
func (d *Directory) Items() int64                         { return d.items }
func (d *Directory) ID() string                           { return "torrentdir:" + d.remote }
func (d *Directory) SetID(string)                         {}
func (d *Directory) Fs() fs.Info                          { return d.fs }

// Standard Fs interface implementations
func (f *Fs) Name() string         { return f.name }
func (f *Fs) Root() string         { return f.root }
func (f *Fs) String() string       { return fmt.Sprintf("torrent root '%s'", f.root) }
func (f *Fs) Features() *fs.Features { return f.features }
func (f *Fs) Precision() time.Duration { return time.Second }
func (f *Fs) Hashes() hash.Set     { return hash.Set(hash.None) }

// Read-only operation errors
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
    return nil, fs.ErrorPermissionDenied
}
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
    return nil, fs.ErrorPermissionDenied
}
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
    return nil, fs.ErrorPermissionDenied
}

// Pass-through operations to base filesystem
func (f *Fs) Mkdir(ctx context.Context, dir string) error { return f.baseFs.Mkdir(ctx, dir) }
func (f *Fs) Rmdir(ctx context.Context, dir string) error { return f.baseFs.Rmdir(ctx, dir) }

// DirMove handles directory movement in the base filesystem
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
    srcFs, ok := src.(*Fs)
    if !ok {
        fs.Debugf(srcRemote, "Can't move directory - not same remote type")
        return fs.ErrorCantDirMove
    }
    return srcFs.baseFs.Features().DirMove(ctx, srcFs.baseFs, srcRemote, dstRemote)
}

// getTorrent loads or retrieves a torrent and updates its access time
func (f *Fs) getTorrent(path string) (*torrent.Torrent, error) {
    // Try to get existing torrent
    if v, ok := f.activeTorrents.Load(path); ok {
        info := v.(*torrentInfo)
        info.mu.Lock()
        info.lastAccess = time.Now()
        info.mu.Unlock()
        return info.torrent, nil
    }

    fs.Debugf(nil, "Loading new torrent: %s", path)
    
    // Load new torrent
    t, err := f.client.AddTorrentFromFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to load torrent: %w", err)
    }

    // Add standard public trackers
    t.AddTrackers([][]string{
        {"udp://tracker.opentrackr.org:1337/announce"},
        {"udp://tracker.openbittorrent.com:6969/announce"},
        {"udp://exodus.desync.com:6969/announce"},
        {"udp://tracker.torrent.eu.org:451/announce"},
    })

    // Wait for metadata with timeout
    select {
    case <-t.GotInfo():
        fs.Debugf(nil, "Got torrent metadata for: %s", t.Name())
    case <-time.After(defaultHandshakeTimeout):
        t.Drop()
        return nil, fmt.Errorf("timeout waiting for torrent metadata")
    }

    // Log torrent details
    fs.Debugf(nil, "Torrent loaded: %s (pieces: %d, length: %d, trackers: %d)", 
        t.Name(), t.Info().NumPieces(), t.Length(), len(t.Metainfo().AnnounceList))

    // Start downloading
    t.DownloadAll()

    // Store in active torrents map
    info := &torrentInfo{
        torrent:    t,
        lastAccess: time.Now(),
    }
    f.activeTorrents.Store(path, info)

    // Start cleanup if enabled
    if f.opt.CleanupTimeout > 0 {
        go f.cleanupTorrent(path)
    }

    // Start peer stats monitoring
    go f.monitorPeerStats(t, path)

    return t, nil
}

// monitorPeerStats periodically logs peer statistics
func (f *Fs) monitorPeerStats(t *torrent.Torrent, path string) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if _, exists := f.activeTorrents.Load(path); !exists {
            return
        }

        stats := t.Stats()
        fs.Debugf(nil, "Peer stats for %s - Active: %d, Total: %d, Pending: %d", 
            t.Name(), stats.ActivePeers, stats.TotalPeers, stats.PendingPeers)
    }
}

// cleanupTorrent monitors torrent activity and removes inactive ones
func (f *Fs) cleanupTorrent(path string) {
    if f.opt.CleanupTimeout <= 0 {
        return
    }

    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    timeout := time.Duration(f.opt.CleanupTimeout) * time.Minute
    
    for range ticker.C {
        v, ok := f.activeTorrents.Load(path)
        if !ok {
            return
        }

        info := v.(*torrentInfo)
        info.mu.RLock()
        inactive := time.Since(info.lastAccess) > timeout
        info.mu.RUnlock()

        if inactive {
            if v, ok := f.activeTorrents.LoadAndDelete(path); ok {
                info := v.(*torrentInfo)
                info.torrent.Drop()
                fs.Debugf(nil, "Dropped inactive torrent: %s", path)
            }
            return
        }
    }
}

// List implements directory listing with virtual torrent directories
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
    fs.Debugf(dir, "Listing directory")

    // Get base directory contents
    baseEntries, err := f.baseFs.List(ctx, dir)
    if err != nil {
        // Check if it's a virtual torrent directory
        if torrentPath, isVirtual := f.findTorrentForPath(dir); isVirtual {
            fs.Debugf(dir, "Found torrent file: %s", torrentPath)
            return f.listTorrentContents(ctx, torrentPath, dir)
        }
        return nil, err
    }

    // Track seen names to avoid duplicates
    seen := make(map[string]bool)
    entries = make(fs.DirEntries, 0, len(baseEntries))

    // Add regular non-torrent entries
    for _, entry := range baseEntries {
        name := entry.Remote()
        if !isTorrentFile(name) {
            entries = append(entries, entry)
            seen[name] = true
        }
    }

    // Add virtual torrent directories
    for _, entry := range baseEntries {
        if o, ok := entry.(fs.Object); ok && isTorrentFile(o.Remote()) {
            virtualName := strings.TrimSuffix(o.Remote(), filepath.Ext(o.Remote()))
            if !seen[virtualName] {
                if info, modTime, err := f.getTorrentInfo(o.Remote()); err == nil {
                    size, items := f.getTorrentSize(info)
                    entries = append(entries, &Directory{
                        fs:      f,
                        remote:  virtualName,
                        modTime: modTime,
                        size:    size,
                        items:   items,
                    })
                    seen[virtualName] = true
                }
            }
        }
    }

    fs.Debugf(dir, "Listed %d entries", len(entries))
    return entries, nil
}

// findTorrentForPath locates the .torrent file for a given path
func (f *Fs) findTorrentForPath(path string) (string, bool) {
    if path == "" {
        return "", false
    }

    current := path
    for {
        torrentPath := filepath.Join(f.opt.RootDirectory, current+".torrent")
        if _, err := os.Stat(torrentPath); err == nil {
            fs.Debugf(path, "Found torrent at: %s", torrentPath)
            return torrentPath, true
        }

        parent := filepath.Dir(current)
        if parent == "." || parent == current {
            break
        }
        current = parent
    }

    return "", false
}

// getTorrentInfo reads and parses a torrent file's metadata
func (f *Fs) getTorrentInfo(path string) (*metainfo.Info, time.Time, error) {
    absPath := path
    if !filepath.IsAbs(path) {
        absPath = filepath.Join(f.opt.RootDirectory, path)
    }

    mi, err := metainfo.LoadFromFile(absPath)
    if err != nil {
        return nil, time.Time{}, fmt.Errorf("failed to read torrent info: %w", err)
    }

    info, err := mi.UnmarshalInfo()
    if err != nil {
        return nil, time.Time{}, fmt.Errorf("failed to unmarshal torrent info: %w", err)
    }

    stat, err := os.Stat(absPath)
    if err != nil {
        return nil, time.Time{}, fmt.Errorf("failed to stat torrent file: %w", err)
    }

    return &info, stat.ModTime(), nil
}

// listTorrentContents returns the contents of a torrent as directory entries
func (f *Fs) listTorrentContents(ctx context.Context, torrentPath, virtualPath string) (fs.DirEntries, error) {
    // Get torrent info
    info, modTime, err := f.getTorrentInfo(torrentPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read torrent info: %w", err)
    }

    // Calculate paths
    torrentName := strings.TrimSuffix(filepath.Base(torrentPath), ".torrent")
    relTorrentDir, err := filepath.Rel(f.opt.RootDirectory, filepath.Dir(torrentPath))
    if err != nil {
        return nil, fmt.Errorf("failed to get relative torrent dir: %w", err)
    }
    virtualTorrentDir := filepath.Join(relTorrentDir, torrentName)

    // Get relative path within torrent
    var internalPath string
    switch {
    case virtualPath == virtualTorrentDir:
        internalPath = ""
    case strings.HasPrefix(virtualPath, virtualTorrentDir+string(filepath.Separator)):
        internalPath = strings.TrimPrefix(virtualPath, virtualTorrentDir+string(filepath.Separator))
    default:
        return nil, fmt.Errorf("path %q is not within torrent directory %q", virtualPath, virtualTorrentDir)
    }

    entries := make(fs.DirEntries, 0)
    seen := make(map[string]bool)

    // Handle single file torrents
    if len(info.Files) == 0 {
        if internalPath == "" {
            entries = append(entries, &Object{
                fs:          f,
                virtualPath: filepath.Join(virtualPath, info.Name),
                torrentPath: info.Name,
                size:       info.Length,
                modTime:    modTime,
                sourcePath: torrentPath,
            })
        }
        return entries, nil
    }

    // Handle multi-file torrents efficiently
    prefix := ""
    if internalPath != "" {
        prefix = internalPath + string(filepath.Separator)
    }

    // Use map to track directory sizes
    dirSizes := make(map[string]int64)
    dirItems := make(map[string]int64)

    // Process all files in a single pass
    for _, file := range info.Files {
        filePath := filepath.Join(file.Path...)

        // Skip files not in current directory
        if !strings.HasPrefix(filePath, prefix) {
            continue
        }

        // Get path relative to current directory
        relPath := strings.TrimPrefix(filePath, prefix)
        if relPath == "" {
            continue
        }

        // Split into components
        components := strings.Split(relPath, string(filepath.Separator))
        firstComponent := components[0]

        if len(components) == 1 {
            // File in current directory
            entries = append(entries, &Object{
                fs:          f,
                virtualPath: filepath.Join(virtualPath, firstComponent),
                torrentPath: filePath,
                size:       file.Length,
                modTime:    modTime,
                sourcePath: torrentPath,
            })
        } else if !seen[firstComponent] {
            // Directory - accumulate sizes
            currentPath := firstComponent
            for i := 0; i < len(components)-1; i++ {
                dirSizes[currentPath] += file.Length
                dirItems[currentPath]++
                if i < len(components)-2 {
                    currentPath = filepath.Join(currentPath, components[i+1])
                }
            }

            // Add directory entry if not already seen
            entries = append(entries, &Directory{
                fs:      f,
                remote:  filepath.Join(virtualPath, firstComponent),
                modTime: modTime,
                size:    dirSizes[firstComponent],
                items:   dirItems[firstComponent],
            })
            seen[firstComponent] = true
        }
    }

    fs.Debugf(virtualPath, "Listed %d entries", len(entries))
    return entries, nil
}

// getTorrentSize returns total size and item count for a torrent
func (f *Fs) getTorrentSize(info *metainfo.Info) (size, items int64) {
    if len(info.Files) == 0 {
        return info.Length, 1
    }

    for _, file := range info.Files {
        size += file.Length
        items++
    }
    return
}

// isTorrentFile checks if a file is a torrent based on extension
func isTorrentFile(path string) bool {
    return strings.ToLower(filepath.Ext(path)) == ".torrent"
}

// NewObject finds an Object at the given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
    // Try regular filesystem first
    obj, err := f.baseFs.NewObject(ctx, remote)
    if err == nil && !isTorrentFile(remote) {
        return obj, nil
    }

    // Look for the file in a torrent
    if torrentPath, isVirtual := f.findTorrentForPath(remote); isVirtual {
        info, modTime, err := f.getTorrentInfo(torrentPath)
        if err != nil {
            return nil, err
        }

        // Calculate relative path
        torrentRoot := strings.TrimSuffix(filepath.Base(torrentPath), ".torrent")
        relPath, err := filepath.Rel(torrentRoot, remote)
        if err != nil {
            return nil, fs.ErrorObjectNotFound
        }

        // Find file in torrent
        switch {
        case len(info.Files) == 0:
            // Single file torrent
            if relPath == info.Name {
                return &Object{
                    fs:          f,
                    virtualPath: remote,
                    torrentPath: info.Name,
                    size:        info.Length,
                    modTime:     modTime,
                    sourcePath:  torrentPath,
                }, nil
            }
        default:
            // Multi-file torrent - look for exact match
            for _, file := range info.Files {
                if relPath == filepath.Join(file.Path...) {
                    return &Object{
                        fs:          f,
                        virtualPath: remote,
                        torrentPath: filepath.Join(file.Path...),
                        size:        file.Length,
                        modTime:     modTime,
                        sourcePath:  torrentPath,
                    }, nil
                }
            }
        }
    }

    return nil, fs.ErrorObjectNotFound
}

// getCacheDir determines the directory for storing downloaded data
func getCacheDir(opt Options) string {
    if opt.CacheDir != "" {
        return opt.CacheDir
    }
    return filepath.Join(os.TempDir(), "rclone-torrent-cache")
}

// Shutdown cleanly shuts down the filesystem
func (f *Fs) Shutdown(ctx context.Context) error {
    // Drop all active torrents
    f.activeTorrents.Range(func(key, value interface{}) bool {
        if info, ok := value.(*torrentInfo); ok {
            info.mu.Lock()
            if info.torrent != nil {
                info.torrent.Drop()
            }
            info.mu.Unlock()
        }
        return true
    })

    // Close torrent client
    if f.client != nil {
        f.client.Close()
    }

    // Shutdown base filesystem if supported
    if shutdowner, ok := f.baseFs.(fs.Shutdowner); ok {
        return shutdowner.Shutdown(ctx)
    }

    return nil
}

// NewFs creates a new Fs instance
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
    // Parse config
    opt := new(Options)
    err := configstruct.Set(m, opt)
    if err != nil {
        return nil, err
    }

    // Configure torrent client
    cfg := torrent.NewDefaultClientConfig()
    cfg.DataDir = getCacheDir(*opt)

    // Set bandwidth limits
    if opt.MaxDownloadSpeed > 0 {
        cfg.DownloadRateLimiter = rate.NewLimiter(rate.Limit(opt.MaxDownloadSpeed*1024), 256*1024)
    }
    if opt.MaxUploadSpeed > 0 {
        cfg.UploadRateLimiter = rate.NewLimiter(rate.Limit(opt.MaxUploadSpeed*1024), 256*1024)
    }

    // Configure for read-only operation
    cfg.NoUpload = true
    cfg.Seed = false
    
    // Network settings
    cfg.HandshakesTimeout = defaultHandshakeTimeout
    cfg.HalfOpenConnsPerTorrent = defaultPendingPeers
    cfg.EstablishedConnsPerTorrent = maxConnectionsPerTorrent
    cfg.DisableUTP = false
    cfg.DisableTCP = false
    cfg.NoDHT = false
    cfg.DisableIPv6 = false

    // Create torrent client
    client, err := torrent.NewClient(cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create torrent client: %w", err)
    }

    // Create base filesystem
    baseFs, err := fs.NewFs(ctx, opt.RootDirectory)
    if err != nil {
        client.Close()
        return nil, fmt.Errorf("failed to create base filesystem: %w", err)
    }

    // Set up features
    features := &fs.Features{
        CaseInsensitive:         false,
        DuplicateFiles:          false,
        ReadMimeType:            false,
        WriteMimeType:           false,
        CanHaveEmptyDirectories: true,
        BucketBased:            false,
        BucketBasedRootOK:      false,
        SetTier:                false,
        GetTier:                false,
    }

    return &Fs{
        name:     name,
        root:     root,
        opt:      *opt,
        client:   client,
        features: features,
        baseFs:   baseFs,
    }, nil
}