// Package mediavfs provides a filesystem interface to a PostgreSQL media database
//
// It creates a virtual filesystem where files are organized by username,
// with support for custom paths and names stored in the database.
package mediavfs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
)

const (
	minSleep       = 10 * time.Millisecond
	maxSleep       = 2 * time.Second
	decayConstant  = 2
	cacheTTL       = 2 * time.Hour   // Cache resolved URLs and ETags for 2 hours
	chunkSize      = 1000             // Load files in chunks of 1000
	defaultPollInt = 30 * time.Second // Default polling interval
)

var (
	errNotWritable = errors.New("mediavfs is read-only except for move/rename operations")
	errCrossUser   = errors.New("cannot move files between different users")
)

// urlMetadata stores cached URL resolution and ETag information
type urlMetadata struct {
	resolvedURL string
	etag        string
	size        int64
	expiresAt   time.Time
}

// urlCache is a TTL cache for resolved URLs and their ETags
type urlCache struct {
	cache map[string]*urlMetadata
	mu    sync.RWMutex
}

// cachedFile stores file metadata from database
type cachedFile struct {
	mediaKey    string
	fileName    string
	customName  string
	customPath  string
	sizeBytes   int64
	timestampUnix int64
}

// fileCache stores all files with lazy loading support
type fileCache struct {
	files          map[string][]cachedFile // key: userName, value: list of files
	lastPoll       time.Time
	mu             sync.RWMutex
	stopChan       chan struct{}
	pollRunning    bool
	intervalUpdate chan time.Duration // Channel to update poll interval dynamically
}

func newURLCache() *urlCache {
	return &urlCache{
		cache: make(map[string]*urlMetadata),
	}
}

func newFileCache() *fileCache {
	return &fileCache{
		files:          make(map[string][]cachedFile),
		lastPoll:       time.Now(),
		stopChan:       make(chan struct{}),
		intervalUpdate: make(chan time.Duration, 1),
	}
}

func (c *urlCache) get(key string) (*urlMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	meta, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(meta.expiresAt) {
		return nil, false
	}

	return meta, true
}

func (c *urlCache) set(key string, meta *urlMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	meta.expiresAt = time.Now().Add(cacheTTL)
	c.cache[key] = meta

	// Simple cleanup: remove expired entries if cache is too large
	if len(c.cache) > 1000 {
		for k, v := range c.cache {
			if time.Now().After(v.expiresAt) {
				delete(c.cache, k)
			}
		}
	}
}

func init() {
	fsi := &fs.RegInfo{
		Name:        "mediavfs",
		Description: "PostgreSQL Media Virtual Filesystem",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "db_connection",
			Help:     "PostgreSQL connection string.\n\nE.g. \"postgres://user:password@localhost/dbname?sslmode=disable\"",
			Required: true,
		}, {
			Name:     "download_url",
			Help:     "Base URL for file downloads.\n\nE.g. \"http://localhost/gphotos/download\"",
			Required: true,
		}, {
			Name:     "table_name",
			Help:     "Name of the media table in the database.",
			Default:  "media",
			Advanced: true,
		}, {
			Name:     "poll_interval",
			Help:     "How often to poll the database for new files.\n\nSet to 0 to disable polling.",
			Default:  fs.Duration(defaultPollInt),
			Advanced: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	DBConnection string      `config:"db_connection"`
	DownloadURL  string      `config:"download_url"`
	TableName    string      `config:"table_name"`
	PollInterval fs.Duration `config:"poll_interval"`
}

// Fs represents a connection to the media database
type Fs struct {
	name          string
	root          string
	opt           Options
	features      *fs.Features
	db            *sql.DB
	httpClient    *http.Client
	urlCache      *urlCache
	fileCache     *fileCache
	virtualDirs   map[string]bool // Track virtual directories created in memory
	vdirMu        sync.RWMutex    // Mutex for virtualDirs
	pollChanges   chan string     // Channel for change notifications
	pollCloseOnce sync.Once       // Ensure we only close once
}

// Object represents a media file in the database
type Object struct {
	fs          *Fs
	remote      string
	mediaKey    string
	size        int64
	modTime     time.Time
	userName    string
	displayName string // The name to display (from 'name' column or 'file_name')
	displayPath string // The path to display (from 'path' column or derived from remote)
}

// convertUnixTimestamp converts a Unix timestamp (seconds or milliseconds) to time.Time
func convertUnixTimestamp(timestamp int64) time.Time {
	// If timestamp is > 10^10, it's likely in milliseconds
	if timestamp > 10000000000 {
		return time.Unix(timestamp/1000, (timestamp%1000)*1000000)
	}
	// Otherwise assume seconds
	return time.Unix(timestamp, 0)
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Media VFS root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Ensure download URL doesn't have trailing slash
	opt.DownloadURL = strings.TrimSuffix(opt.DownloadURL, "/")

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", opt.DBConnection)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Create custom HTTP client with redirect handling that preserves headers
	baseClient := fshttp.NewClient(ctx)
	customClient := &http.Client{
		Transport: baseClient.Transport,
		Timeout:   0, // No timeout for streaming large files
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if len(via) > 0 {
				originalReq := via[0]
				// Preserve Range header (critical for resume/seeking)
				if rangeHeader := originalReq.Header.Get("Range"); rangeHeader != "" {
					req.Header.Set("Range", rangeHeader)
					fs.Infof(nil, "mediavfs: preserving Range header on redirect: %s", rangeHeader)
				}
				// Preserve If-Range header (critical for ETag validation)
				if ifRangeHeader := originalReq.Header.Get("If-Range"); ifRangeHeader != "" {
					req.Header.Set("If-Range", ifRangeHeader)
				}
				// Preserve User-Agent
				if userAgent := originalReq.Header.Get("User-Agent"); userAgent != "" {
					req.Header.Set("User-Agent", userAgent)
				}
				fs.Infof(nil, "mediavfs: following redirect from %s to %s",
					via[len(via)-1].URL.Redacted(), req.URL.Redacted())
			}
			return nil
		},
	}

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		db:          db,
		httpClient:  customClient,
		urlCache:    newURLCache(),
		fileCache:   newFileCache(),
		virtualDirs: make(map[string]bool),
		pollChanges: make(chan string, 1), // Buffered channel for change notifications
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         false,
		ChangeNotify:            f.ChangeNotify,
		UnWrap:                  f.UnWrap,
	}).Fill(ctx, f)

	// Start polling goroutine if poll_interval is set
	if f.opt.PollInterval > 0 {
		go f.startPolling(ctx)
	}

	// Validate root path if specified
	if root != "" {
		_, err := f.NewObject(ctx, root)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// Root might be a directory, which is fine
			} else if err != fs.ErrorIsDir {
				return nil, err
			}
		}
	}

	return f, nil
}

// splitUserPath splits a path into username and the rest
// e.g., "john/photos/img.jpg" -> "john", "photos/img.jpg"
func splitUserPath(remote string) (userName string, filePath string) {
	parts := strings.SplitN(remote, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// extractPathAndName extracts path and name from file_name if it contains "/"
// e.g., "photos/vacation/img.jpg" -> "photos/vacation", "img.jpg"
func extractPathAndName(fileName string) (path string, name string) {
	if strings.Contains(fileName, "/") {
		lastSlash := strings.LastIndex(fileName, "/")
		return fileName[:lastSlash], fileName[lastSlash+1:]
	}
	return "", fileName
}

// startPolling polls the database for new/updated files with dynamic interval support
func (f *Fs) startPolling(ctx context.Context) {
	f.fileCache.mu.Lock()
	if f.fileCache.pollRunning {
		f.fileCache.mu.Unlock()
		return
	}
	f.fileCache.pollRunning = true
	f.fileCache.mu.Unlock()

	currentInterval := time.Duration(f.opt.PollInterval)
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	fs.Infof(nil, "mediavfs: polling started with interval %v", currentInterval)

	for {
		select {
		case <-ctx.Done():
			fs.Infof(nil, "mediavfs: polling stopped (context done)")
			return
		case <-f.fileCache.stopChan:
			fs.Infof(nil, "mediavfs: polling stopped (stop channel)")
			return
		case newInterval := <-f.fileCache.intervalUpdate:
			// Update the ticker with new interval
			if newInterval > 0 && newInterval != currentInterval {
				ticker.Stop()
				currentInterval = newInterval
				ticker = time.NewTicker(currentInterval)
				fs.Infof(nil, "mediavfs: polling interval updated to %v", currentInterval)
			}
		case <-ticker.C:
			// Poll all users and update cache
			fs.Infof(nil, "mediavfs: polling database (interval: %v)", currentInterval)
			f.pollDatabaseForChanges(ctx)
		}
	}
}

// pollDatabaseForChanges queries the database for all files and updates the cache
func (f *Fs) pollDatabaseForChanges(ctx context.Context) {
	// Get all users first
	usersQuery := fmt.Sprintf("SELECT DISTINCT user_name FROM %s WHERE user_name IS NOT NULL", f.opt.TableName)
	rows, err := f.db.QueryContext(ctx, usersQuery)
	if err != nil {
		fs.Errorf(nil, "mediavfs: poll error getting users: %v", err)
		return
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var userName string
		if err := rows.Scan(&userName); err != nil {
			fs.Errorf(nil, "mediavfs: poll error scanning user: %v", err)
			continue
		}
		users = append(users, userName)
	}
	rows.Close()

	// For each user, load all files and update cache
	for _, userName := range users {
		filesQuery := fmt.Sprintf(`
			SELECT
				media_key,
				file_name,
				COALESCE(name, '') as custom_name,
				COALESCE(path, '') as custom_path,
				size_bytes,
				utc_timestamp
			FROM %s
			WHERE user_name = $1
			ORDER BY file_name
		`, f.opt.TableName)

		fileRows, err := f.db.QueryContext(ctx, filesQuery, userName)
		if err != nil {
			fs.Errorf(nil, "mediavfs: poll error getting files for user %s: %v", userName, err)
			continue
		}

		var userFiles []cachedFile
		for fileRows.Next() {
			var cf cachedFile
			if err := fileRows.Scan(&cf.mediaKey, &cf.fileName, &cf.customName, &cf.customPath, &cf.sizeBytes, &cf.timestampUnix); err != nil {
				fs.Errorf(nil, "mediavfs: poll error scanning file: %v", err)
				continue
			}
			userFiles = append(userFiles, cf)
		}
		fileRows.Close()

		// Update cache
		f.fileCache.mu.Lock()
		oldCount := len(f.fileCache.files[userName])
		f.fileCache.files[userName] = userFiles
		newCount := len(userFiles)
		f.fileCache.lastPoll = time.Now()
		f.fileCache.mu.Unlock()

		if newCount != oldCount {
			fs.Infof(nil, "mediavfs: poll updated user %s: %d -> %d files", userName, oldCount, newCount)

			// Send change notification for this user's directory
			select {
			case f.pollChanges <- userName:
				fs.Infof(nil, "mediavfs: sent change notification for user %s", userName)
			default:
				// Channel is full, skip notification (non-blocking)
				fs.Infof(nil, "mediavfs: skipped change notification for user %s (channel full)", userName)
			}
		}
	}
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	root := strings.Trim(path.Join(f.root, dir), "/")
	fs.Infof(nil, "mediavfs: List called for dir='%s', root='%s'", dir, root)

	// If root is empty, list all users
	if root == "" {
		fs.Infof(nil, "mediavfs: listing all users (root level)")
		entries, err = f.listUsers(ctx)
		fs.Infof(nil, "mediavfs: listUsers returned %d entries, err=%v", len(entries), err)
		return entries, err
	}

	userName, subPath := splitUserPath(root)
	fs.Infof(nil, "mediavfs: split path into userName='%s', subPath='%s'", userName, subPath)

	// If only username is specified, list top-level items for that user
	if subPath == "" {
		fs.Infof(nil, "mediavfs: listing top-level items for user '%s'", userName)
		entries, err = f.listUserFiles(ctx, userName, "")
		fs.Infof(nil, "mediavfs: listUserFiles returned %d entries, err=%v", len(entries), err)
		return entries, err
	}

	// List files in a specific directory
	fs.Infof(nil, "mediavfs: listing files in user '%s', subPath '%s'", userName, subPath)
	entries, err = f.listUserFiles(ctx, userName, subPath)
	fs.Infof(nil, "mediavfs: listUserFiles returned %d entries, err=%v", len(entries), err)
	return entries, err
}

// listUsers returns a list of all unique usernames as directories
func (f *Fs) listUsers(ctx context.Context) (entries fs.DirEntries, err error) {
	fs.Infof(nil, "mediavfs: listUsers called")

	query := fmt.Sprintf(`
		SELECT DISTINCT user_name
		FROM %s
		WHERE user_name IS NOT NULL
		ORDER BY user_name
	`, f.opt.TableName)

	fs.Infof(nil, "mediavfs: listUsers query: %s", query)

	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		fs.Errorf(nil, "mediavfs: listUsers query failed: %v", err)
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	userCount := 0
	for rows.Next() {
		var userName string
		if err := rows.Scan(&userName); err != nil {
			fs.Errorf(nil, "mediavfs: listUsers scan failed: %v", err)
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		fs.Infof(nil, "mediavfs: listUsers found user: '%s'", userName)
		entries = append(entries, fs.NewDir(userName, time.Time{}))
		userCount++
	}

	if err = rows.Err(); err != nil {
		fs.Errorf(nil, "mediavfs: listUsers iteration error: %v", err)
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	fs.Infof(nil, "mediavfs: listUsers returning %d users, %d entries", userCount, len(entries))
	return entries, nil
}

// listUserFiles lists files and directories for a specific user and path using cache
func (f *Fs) listUserFiles(ctx context.Context, userName string, dirPath string) (entries fs.DirEntries, err error) {
	fs.Infof(nil, "mediavfs: listUserFiles called for userName='%s', dirPath='%s'", userName, dirPath)

	// Try to get files from cache first
	f.fileCache.mu.RLock()
	cachedFiles, found := f.fileCache.files[userName]
	cacheSize := len(cachedFiles)
	f.fileCache.mu.RUnlock()

	fs.Infof(nil, "mediavfs: cache lookup: found=%v, cacheSize=%d", found, cacheSize)

	// If not in cache or cache is empty, load from database
	if !found || len(cachedFiles) == 0 {
		// Load files from database in chunks
		query := fmt.Sprintf(`
			SELECT
				media_key,
				file_name,
				COALESCE(name, '') as custom_name,
				COALESCE(path, '') as custom_path,
				size_bytes,
				utc_timestamp
			FROM %s
			WHERE user_name = $1
			ORDER BY file_name
		`, f.opt.TableName)

		rows, err := f.db.QueryContext(ctx, query, userName)
		if err != nil {
			return nil, fmt.Errorf("failed to query files: %w", err)
		}

		// Load all files for this user (they'll be cached for next time)
		for rows.Next() {
			var cf cachedFile
			if err := rows.Scan(&cf.mediaKey, &cf.fileName, &cf.customName, &cf.customPath, &cf.sizeBytes, &cf.timestampUnix); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan file: %w", err)
			}
			cachedFiles = append(cachedFiles, cf)
		}
		rows.Close()

		// Store in cache
		f.fileCache.mu.Lock()
		f.fileCache.files[userName] = cachedFiles
		f.fileCache.mu.Unlock()

		fs.Infof(nil, "mediavfs: loaded %d files for user %s into cache", len(cachedFiles), userName)
	}

	// Track directories we've already added
	dirsSeen := make(map[string]bool)
	var dirsSeenMu sync.Mutex

	// Normalize dirPath for comparison
	dirPath = strings.Trim(dirPath, "/")
	var dirPrefix string
	if dirPath != "" {
		dirPrefix = dirPath + "/"
	}

	// Process cached files concurrently for better performance with large file counts
	const numWorkers = 8
	const chunkSize = 1000

	// Split files into chunks
	var chunks [][]cachedFile
	for i := 0; i < len(cachedFiles); i += chunkSize {
		end := i + chunkSize
		if end > len(cachedFiles) {
			end = len(cachedFiles)
		}
		chunks = append(chunks, cachedFiles[i:end])
	}

	// Process chunks concurrently
	var wg sync.WaitGroup
	var entriesMu sync.Mutex
	chunkChan := make(chan []cachedFile, len(chunks))

	// Send chunks to channel
	for _, chunk := range chunks {
		chunkChan <- chunk
	}
	close(chunkChan)

	// Start workers
	workers := numWorkers
	if workers > len(chunks) {
		workers = len(chunks)
	}
	if workers == 0 {
		workers = 1
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for chunk := range chunkChan {
				localEntries := make([]fs.DirEntry, 0, len(chunk))

				for _, cf := range chunk {
					timestamp := convertUnixTimestamp(cf.timestampUnix)

					// Determine the display name and path
					var displayName, displayPath string

					if cf.customName != "" {
						displayName = cf.customName
					} else {
						_, displayName = extractPathAndName(cf.fileName)
					}

					if cf.customPath != "" {
						displayPath = strings.Trim(cf.customPath, "/")
					} else {
						extractedPath, _ := extractPathAndName(cf.fileName)
						displayPath = strings.Trim(extractedPath, "/")
					}

					// Construct the full path
					var fullPath string
					if displayPath != "" {
						fullPath = displayPath + "/" + displayName
					} else {
						fullPath = displayName
					}

					// Check if this file is in the current directory or a subdirectory
					if dirPath == "" {
						// We're at the root of the user's directory
						if strings.Contains(fullPath, "/") {
							// This is in a subdirectory
							subDir := strings.SplitN(fullPath, "/", 2)[0]
							dirsSeenMu.Lock()
							if !dirsSeen[subDir] {
								localEntries = append(localEntries, fs.NewDir(userName+"/"+subDir, time.Time{}))
								dirsSeen[subDir] = true
							}
							dirsSeenMu.Unlock()
						} else {
							// This is a file directly in the root
							remote := userName + "/" + fullPath
							localEntries = append(localEntries, &Object{
								fs:          f,
								remote:      remote,
								mediaKey:    cf.mediaKey,
								size:        cf.sizeBytes,
								modTime:     timestamp,
								userName:    userName,
								displayName: displayName,
								displayPath: displayPath,
							})
						}
					} else {
						// We're in a specific subdirectory
						if !strings.HasPrefix(fullPath, dirPrefix) {
							continue // Not in this directory
						}

						remainder := strings.TrimPrefix(fullPath, dirPrefix)
						if strings.Contains(remainder, "/") {
							// This is in a subdirectory
							subDir := strings.SplitN(remainder, "/", 2)[0]
							fullSubDir := dirPath + "/" + subDir
							dirsSeenMu.Lock()
							if !dirsSeen[fullSubDir] {
								localEntries = append(localEntries, fs.NewDir(userName+"/"+fullSubDir, time.Time{}))
								dirsSeen[fullSubDir] = true
							}
							dirsSeenMu.Unlock()
						} else {
							// This is a file directly in this directory
							remote := userName + "/" + fullPath
							localEntries = append(localEntries, &Object{
								fs:          f,
								remote:      remote,
								mediaKey:    cf.mediaKey,
								size:        cf.sizeBytes,
								modTime:     timestamp,
								userName:    userName,
								displayName: displayName,
								displayPath: displayPath,
							})
						}
					}
				}

				// Add local entries to shared entries slice
				entriesMu.Lock()
				entries = append(entries, localEntries...)
				entriesMu.Unlock()
			}
		}()
	}

	// Wait for all workers to complete
	wg.Wait()

	fs.Infof(nil, "mediavfs: processed %d files concurrently (%d workers, %d chunks)", len(cachedFiles), workers, len(chunks))

	// Add virtual directories created via Mkdir
	f.vdirMu.RLock()
	for vdir := range f.virtualDirs {
		// Check if this virtual directory belongs to this user and path
		vdirUser, vdirPath := splitUserPath(vdir)
		if vdirUser != userName {
			continue
		}

		// Check if it's in the current directory we're listing
		if dirPath == "" {
			// Root level - add if it's a top-level dir
			if !strings.Contains(vdirPath, "/") && vdirPath != "" && !dirsSeen[vdirPath] {
				entries = append(entries, fs.NewDir(userName+"/"+vdirPath, time.Time{}))
				dirsSeen[vdirPath] = true
			} else if strings.Contains(vdirPath, "/") {
				// It's nested, add the top-level part
				topDir := strings.SplitN(vdirPath, "/", 2)[0]
				if !dirsSeen[topDir] {
					entries = append(entries, fs.NewDir(userName+"/"+topDir, time.Time{}))
					dirsSeen[topDir] = true
				}
			}
		} else {
			// We're in a subdirectory
			dirPrefix := dirPath + "/"
			if strings.HasPrefix(vdirPath, dirPrefix) {
				remainder := strings.TrimPrefix(vdirPath, dirPrefix)
				if !strings.Contains(remainder, "/") && remainder != "" && !dirsSeen[vdirPath] {
					// Direct child directory
					entries = append(entries, fs.NewDir(userName+"/"+vdirPath, time.Time{}))
					dirsSeen[vdirPath] = true
				} else if strings.Contains(remainder, "/") {
					// Nested subdirectory
					subDir := strings.SplitN(remainder, "/", 2)[0]
					fullSubDir := dirPath + "/" + subDir
					if !dirsSeen[fullSubDir] {
						entries = append(entries, fs.NewDir(userName+"/"+fullSubDir, time.Time{}))
						dirsSeen[fullSubDir] = true
					}
				}
			} else if vdirPath == dirPath && !dirsSeen[vdirPath] {
				// The virtual directory is exactly this path
				entries = append(entries, fs.NewDir(userName+"/"+vdirPath, time.Time{}))
				dirsSeen[vdirPath] = true
			}
		}
	}
	f.vdirMu.RUnlock()

	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	userName, filePath := splitUserPath(remote)
	if userName == "" || filePath == "" {
		return nil, fs.ErrorIsDir
	}

	// Try to find the file by matching the constructed path
	query := fmt.Sprintf(`
		SELECT
			media_key,
			file_name,
			COALESCE(name, '') as custom_name,
			COALESCE(path, '') as custom_path,
			size_bytes,
			utc_timestamp
		FROM %s
		WHERE user_name = $1
	`, f.opt.TableName)

	rows, err := f.db.QueryContext(ctx, query, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to query file: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			mediaKey      string
			fileName      string
			customName    string
			customPath    string
			sizeBytes     int64
			timestampUnix int64
		)

		if err := rows.Scan(&mediaKey, &fileName, &customName, &customPath, &sizeBytes, &timestampUnix); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		timestamp := convertUnixTimestamp(timestampUnix)

		// Determine the display name and path (same logic as listUserFiles)
		var displayName, displayPath string

		if customName != "" {
			displayName = customName
		} else {
			_, displayName = extractPathAndName(fileName)
		}

		if customPath != "" {
			displayPath = strings.Trim(customPath, "/")
		} else {
			extractedPath, _ := extractPathAndName(fileName)
			displayPath = strings.Trim(extractedPath, "/")
		}

		// Construct the full path
		var fullPath string
		if displayPath != "" {
			fullPath = displayPath + "/" + displayName
		} else {
			fullPath = displayName
		}

		if fullPath == filePath {
			return &Object{
				fs:          f,
				remote:      remote,
				mediaKey:    mediaKey,
				size:        sizeBytes,
				modTime:     timestamp,
				userName:    userName,
				displayName: displayName,
				displayPath: displayPath,
			}, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// Put is not supported
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errNotWritable
}

// PutStream is not supported
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errNotWritable
}

// Mkdir creates a virtual directory in memory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// Store the full path including username
	fullPath := strings.Trim(path.Join(f.root, dir), "/")

	f.vdirMu.Lock()
	f.virtualDirs[fullPath] = true
	f.vdirMu.Unlock()

	fs.Infof(nil, "mediavfs: created virtual directory: %s", fullPath)
	return nil // Virtual directories, always succeed
}

// Rmdir is not supported
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return errNotWritable
}

// Move moves src to this remote
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	// Check that both source and destination are for the same user
	srcUser, _ := splitUserPath(src.Remote())
	dstUser, dstPath := splitUserPath(remote)

	if srcUser != dstUser {
		return nil, errCrossUser
	}

	// Parse the new path and name
	var newPath, newName string
	if strings.Contains(dstPath, "/") {
		lastSlash := strings.LastIndex(dstPath, "/")
		newPath = dstPath[:lastSlash]
		newName = dstPath[lastSlash+1:]
	} else {
		newPath = ""
		newName = dstPath
	}

	// Check if moving to a virtual directory - if so, remove it from virtual dirs
	if newPath != "" {
		vdirPath := dstUser + "/" + newPath
		f.vdirMu.Lock()
		if f.virtualDirs[vdirPath] {
			delete(f.virtualDirs, vdirPath)
			fs.Infof(nil, "mediavfs: removed virtual directory after file move: %s", vdirPath)
		}
		f.vdirMu.Unlock()
	}

	// Update the database
	query := fmt.Sprintf(`
		UPDATE %s
		SET name = $1, path = $2
		WHERE media_key = $3
	`, f.opt.TableName)

	_, err := f.db.ExecContext(ctx, query, newName, newPath, srcObj.mediaKey)
	if err != nil {
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	// Update the object in-place for real-time changes (no DB re-query)
	newObj := &Object{
		fs:          f,
		remote:      remote,
		mediaKey:    srcObj.mediaKey,
		size:        srcObj.size,
		modTime:     srcObj.modTime,
		userName:    dstUser,
		displayName: newName,
		displayPath: newPath,
	}

	fs.Infof(nil, "mediavfs: moved %s to %s (real-time update)", srcObj.remote, remote)

	return newObj, nil
}

// DirMove is not supported
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	return fs.ErrorCantDirMove
}

// Shutdown the backend
func (f *Fs) Shutdown(ctx context.Context) error {
	// Stop polling goroutine
	if f.fileCache != nil {
		close(f.fileCache.stopChan)
	}

	// Close pollChanges channel safely
	f.pollCloseOnce.Do(func() {
		if f.pollChanges != nil {
			close(f.pollChanges)
		}
	})

	// Close database connection
	if f.db != nil {
		return f.db.Close()
	}
	return nil
}

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	fs.Infof(nil, "mediavfs: ChangeNotify started, waiting for events...")

	// Immediately try to read the initial poll interval (non-blocking)
	select {
	case interval, ok := <-pollIntervalChan:
		if ok && interval > 0 {
			fs.Infof(nil, "mediavfs: received INITIAL poll interval from rclone: %v", interval)
			select {
			case f.fileCache.intervalUpdate <- interval:
				fs.Infof(nil, "mediavfs: forwarded initial interval to polling goroutine")
			default:
				fs.Infof(nil, "mediavfs: interval update channel full on initial update")
			}
		}
	default:
		fs.Infof(nil, "mediavfs: no initial poll interval available, using backend default")
	}

	for {
		select {
		case <-ctx.Done():
			fs.Infof(nil, "mediavfs: ChangeNotify stopping due to context cancellation")
			return
		case path, ok := <-f.pollChanges:
			if !ok {
				fs.Infof(nil, "mediavfs: ChangeNotify stopping due to channel closure")
				return
			}
			// Notify that this path has changed
			fs.Infof(nil, "mediavfs: notifying change for path: %s", path)
			notifyFunc(path, fs.EntryDirectory)
		case interval, ok := <-pollIntervalChan:
			if !ok {
				fs.Infof(nil, "mediavfs: ChangeNotify stopping due to pollIntervalChan closure")
				return
			}
			if interval > 0 {
				fs.Infof(nil, "mediavfs: received poll interval update from rclone: %v", interval)
				// Forward the interval to the polling goroutine
				select {
				case f.fileCache.intervalUpdate <- interval:
					fs.Infof(nil, "mediavfs: forwarded interval update to polling goroutine")
				default:
					fs.Infof(nil, "mediavfs: interval update channel full, skipping")
				}
			} else {
				fs.Infof(nil, "mediavfs: received zero/negative poll interval (%v), ignoring", interval)
			}
		}
	}
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f
}

// Object methods

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// Hash is not supported
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns true if the object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime is not supported
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for reading with URL caching and ETag support
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Build the initial download URL (may return 302 redirect)
	initialURL := fmt.Sprintf("%s/%s", o.fs.opt.DownloadURL, o.mediaKey)

	// Check if we have cached metadata for this URL
	cacheKey := o.mediaKey
	cachedMeta, found := o.fs.urlCache.get(cacheKey)

	var resolvedURL, etag string
	var fileSize int64

	if found {
		// Use cached metadata
		resolvedURL = cachedMeta.resolvedURL
		etag = cachedMeta.etag
		fileSize = cachedMeta.size
		fs.Infof(nil, "mediavfs: using cached URL and ETag for %s", o.mediaKey)
	} else {
		// First time accessing this file - resolve URL and get ETag via HEAD request
		fs.Infof(nil, "mediavfs: resolving URL and fetching metadata for %s", o.mediaKey)

		headReq, err := http.NewRequestWithContext(ctx, "HEAD", initialURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HEAD request: %w", err)
		}
		headReq.Header.Set("User-Agent", "AndroidDownloadManager/13")

		headResp, err := o.fs.httpClient.Do(headReq)
		if err != nil {
			return nil, fmt.Errorf("failed to execute HEAD request: %w", err)
		}
		defer headResp.Body.Close()

		// Get the final URL after any redirects
		resolvedURL = headResp.Request.URL.String()

		// Get ETag and size from headers
		etag = headResp.Header.Get("ETag")
		if contentLength := headResp.Header.Get("Content-Length"); contentLength != "" {
			fmt.Sscanf(contentLength, "%d", &fileSize)
		} else {
			fileSize = o.size
		}

		// Cache the metadata
		o.fs.urlCache.set(cacheKey, &urlMetadata{
			resolvedURL: resolvedURL,
			etag:        etag,
			size:        fileSize,
		})

		fs.Infof(nil, "mediavfs: cached URL and ETag=%s for %s", etag, o.mediaKey)
	}

	// Now make the actual GET request to the resolved URL
	req, err := http.NewRequestWithContext(ctx, "GET", resolvedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent header
	req.Header.Set("User-Agent", "AndroidDownloadManager/13")

	// Add If-Range header with ETag if we have it
	if etag != "" {
		req.Header.Set("If-Range", etag)
	}

	// Add range headers if specified in options
	for k, v := range fs.OpenOptionHeaders(options) {
		req.Header.Set(k, v)
	}

	// Execute the request
	res, err := o.fs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Check status code - accept both 200 and 206
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		_ = res.Body.Close()
		return nil, fmt.Errorf("HTTP error: %s (status %d)", res.Status, res.StatusCode)
	}

	fs.Infof(nil, "mediavfs: opened %s with status %d", o.mediaKey, res.StatusCode)

	return res.Body, nil
}

// Update is not supported
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errNotWritable
}

// Remove is not supported
func (o *Object) Remove(ctx context.Context) error {
	return errNotWritable
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = (*Fs)(nil)
	_ fs.Object         = (*Object)(nil)
	_ fs.Mover          = (*Fs)(nil)
	_ fs.ChangeNotifier = (*Fs)(nil)
)
