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
	"net/url"
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
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2
	cacheTTL      = 2 * time.Hour // Cache resolved URLs and ETags for 2 hours
)

var (
	errNotWritable = errors.New("mediavfs is read-only - files cannot be created, modified, or deleted from database")
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

func newURLCache() *urlCache {
	return &urlCache{
		cache: make(map[string]*urlMetadata),
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
			Name:     "user",
			Help:     "Google Photos username for this mount.\n\nEach user should have a separate mount.",
			Required: true,
		}, {
			Name:     "db_connection",
			Help:     "PostgreSQL connection string (without database name).\n\nE.g. \"postgres://user:password@localhost:5432?sslmode=disable\"",
			Required: true,
		}, {
			Name:     "db_name",
			Help:     "Name of the PostgreSQL database to use.",
			Default:  "gphotos",
		}, {
			Name:     "table_name",
			Help:     "Name of the media table in the database.",
			Default:  "remote_media",
			Advanced: true,
		}, {
			Name:     "enable_upload",
			Help:     "Enable uploading files to Google Photos.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "enable_delete",
			Help:     "Enable deleting files from Google Photos.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "token_server_url",
			Help:     "URL of the token server for Google Photos authentication.\n\nE.g. \"https://m.alicuxi.net\"",
			Default:  "https://m.alicuxi.net",
			Advanced: true,
		}, {
			Name:     "auto_sync",
			Help:     "Enable automatic background sync to detect new files uploaded via Google Photos web/app.",
			Default:  false,
		}, {
			Name:     "sync_interval",
			Help:     "Interval between automatic syncs in seconds. Only used when auto_sync is enabled.",
			Default:  60,
			Advanced: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	User           string `config:"user"`
	DBConnection   string `config:"db_connection"`
	DBName         string `config:"db_name"`
	TableName      string `config:"table_name"`
	BatchSize      int    `config:"batch_size"`
	EnableUpload   bool   `config:"enable_upload"`
	EnableDelete   bool   `config:"enable_delete"`
	TokenServerURL string `config:"token_server_url"`
	AutoSync       bool   `config:"auto_sync"`
	SyncInterval   int    `config:"sync_interval"`
}

// Fs represents a connection to the media database
type Fs struct {
	name        string
	root        string
	opt         Options
	features    *fs.Features
	db          *sql.DB
	httpClient  *http.Client
	api         *GPhotoAPI // Google Photos API client for download URLs
	urlCache *urlCache
	// lazyMeta stores metadata loaded asynchronously for large listings
	lazyMeta map[string]*Object
	lazyMu   sync.RWMutex
	// dirCache caches directory listings to avoid reloading on every change
	dirCache map[string]*dirCacheEntry
	dirMu    sync.RWMutex
	// folderExistsCache tracks folders we've already created/verified in this session
	folderExistsCache map[string]bool
	folderCacheMu     sync.RWMutex
	// syncStop channel to stop background sync goroutine
	syncStop chan struct{}
	// notifyListener for PostgreSQL LISTEN/NOTIFY real-time updates
	notifyListener *NotifyListener
}

// dirCacheEntry represents a cached directory listing
type dirCacheEntry struct {
	entries   fs.DirEntries
	expiresAt time.Time
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

	// Build the full connection string with database name
	dbConnStr := buildConnectionString(opt.DBConnection, opt.DBName)

	// Ensure the database exists (create if needed)
	if err := ensureDatabaseExists(ctx, opt.DBConnection, opt.DBName); err != nil {
		return nil, fmt.Errorf("failed to ensure database exists: %w", err)
	}

	// Connect to PostgreSQL
	// All users share the same database, distinguished by user_name column
	db, err := sql.Open("postgres", dbConnStr)
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
		Timeout:   60 * time.Second, // 60 second timeout to prevent indefinite hangs
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
		httpClient:        customClient,
		urlCache:          newURLCache(),
		lazyMeta:          make(map[string]*Object),
		dirCache:          make(map[string]*dirCacheEntry),
		folderExistsCache: make(map[string]bool),
		syncStop:          make(chan struct{}),
	}

	// Initialize Google Photos API client for download URLs
	f.api = NewGPhotoAPI(opt.User, opt.TokenServerURL, customClient)

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         false,
	}).Fill(ctx, f)

	// Enable ChangeNotify support so vfs can poll this backend
	f.features.ChangeNotify = f.ChangeNotify

	// Initialize database schema
	fs.Infof(f, "Initializing database schema...")
	if err := f.InitializeDatabase(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Setup PostgreSQL LISTEN/NOTIFY trigger for real-time updates
	if err := f.SetupNotifyTrigger(ctx); err != nil {
		fs.Errorf(f, "Failed to setup notify trigger (non-fatal): %v", err)
	}

	// Start PostgreSQL notification listener for real-time updates
	f.notifyListener = NewNotifyListener(dbConnStr, opt.User)
	if err := f.notifyListener.Start(ctx); err != nil {
		fs.Errorf(f, "Failed to start notify listener (falling back to polling): %v", err)
		f.notifyListener = nil
	}

	// Perform initial sync if needed
	if opt.User != "" {
		fs.Infof(f, "Checking sync state for user: %s", opt.User)
		state, err := f.GetSyncState(ctx)
		if err != nil {
			fs.Errorf(f, "Failed to get sync state: %v", err)
		} else if !state.InitComplete {
			fs.Infof(f, "Performing initial sync for user: %s", opt.User)
			go func() {
				if err := f.SyncFromGooglePhotos(context.Background(), opt.User); err != nil {
					fs.Errorf(f, "Initial sync failed: %v", err)
				}
				// Start background sync after initial sync completes if enabled
				if opt.AutoSync {
					f.startBackgroundSync()
				}
			}()
		} else {
			fs.Infof(f, "User %s already synced", opt.User)
			// Start background sync immediately if already synced and auto_sync is enabled
			if opt.AutoSync {
				f.startBackgroundSync()
			}
		}
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

// buildConnectionString combines base connection string with database name
// baseConn format: postgres://user:password@host:port?params or postgres://user:password@host:port/?params
// Returns: postgres://user:password@host:port/dbname?params
func buildConnectionString(baseConn, dbName string) string {
	u, err := url.Parse(baseConn)
	if err != nil {
		// Fallback to simple string manipulation if URL parsing fails
		questionMark := strings.Index(baseConn, "?")
		if questionMark != -1 {
			base := strings.TrimSuffix(baseConn[:questionMark], "/")
			return base + "/" + dbName + baseConn[questionMark:]
		}
		base := strings.TrimSuffix(baseConn, "/")
		return base + "/" + dbName
	}
	// Set the path to just the database name
	u.Path = "/" + dbName
	return u.String()
}

// ensureDatabaseExists creates the database if it doesn't already exist
func ensureDatabaseExists(ctx context.Context, baseConn, dbName string) error {
	if dbName == "" {
		return fmt.Errorf("database name cannot be empty")
	}

	// Connect to the 'postgres' system database
	postgresConnStr := buildConnectionString(baseConn, "postgres")

	db, err := sql.Open("postgres", postgresConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer db.Close()

	// Check if database exists
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err = db.QueryRowContext(ctx, query, dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if database exists: %w", err)
	}

	if !exists {
		// Create the database
		// Note: Database names cannot be parameterized in CREATE DATABASE
		createQuery := fmt.Sprintf("CREATE DATABASE %q", dbName)
		_, err = db.ExecContext(ctx, createQuery)
		if err != nil {
			return fmt.Errorf("failed to create database %s: %w", dbName, err)
		}
		fs.Infof(nil, "Created database: %s", dbName)
	}

	return nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	root := strings.Trim(path.Join(f.root, dir), "/")

	// Check cache first
	cacheKey := root
	if cacheKey == "" {
		cacheKey = "." // Use "." for root
	}

	f.dirMu.RLock()
	if entry, ok := f.dirCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		f.dirMu.RUnlock()
		fs.Infof(f, "List cache hit for %s", dir)
		return entry.entries, nil
	}
	f.dirMu.RUnlock()

	// Cache miss or expired, load from database
	// Use configured user for per-user mounts
	var result fs.DirEntries
	result, err = f.listUserFiles(ctx, f.opt.User, root)

	if err != nil {
		return nil, err
	}

	// Cache the result with a long TTL (1 hour)
	f.dirMu.Lock()
	f.dirCache[cacheKey] = &dirCacheEntry{
		entries:   result,
		expiresAt: time.Now().Add(time.Hour),
	}
	f.dirMu.Unlock()

	fs.Infof(f, "List cached %s with %d entries", dir, len(result))
	return result, nil
}

// listUsers is no longer needed - single user per database
// Removed in favor of single-user per database model

// listUserFiles lists files and directories for a specific path
// Now uses DB-based folders (type = -1) for much faster directory listing
func (f *Fs) listUserFiles(ctx context.Context, userName string, dirPath string) (entries fs.DirEntries, err error) {
	// Normalize dirPath for comparison
	dirPath = strings.Trim(dirPath, "/")

	// Simple query: get all items where path = dirPath
	// Folders have type = -1, files have type >= 0
	query := fmt.Sprintf(`
		SELECT
			media_key,
			file_name,
			COALESCE(name, '') as custom_name,
			COALESCE(path, '') as custom_path,
			COALESCE(type, 0) as item_type,
			COALESCE(size_bytes, 0) as size_bytes,
			COALESCE(utc_timestamp, 0) as utc_timestamp
		FROM %s
		WHERE user_name = $1 AND COALESCE(path, '') = $2
		ORDER BY type ASC, file_name ASC
	`, f.opt.TableName)

	rows, err := f.db.QueryContext(ctx, query, userName, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			mediaKey      string
			fileName      string
			customName    string
			customPath    string
			itemType      int64
			sizeBytes     int64
			timestampUnix int64
		)
		if err := rows.Scan(&mediaKey, &fileName, &customName, &customPath, &itemType, &sizeBytes, &timestampUnix); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		// Display name: use 'name' if set, else use 'file_name'
		displayName := fileName
		if customName != "" {
			displayName = customName
		}

		if itemType == -1 {
			// This is a folder
			var folderPath string
			if dirPath == "" {
				folderPath = displayName
			} else {
				folderPath = dirPath + "/" + displayName
			}
			entries = append(entries, fs.NewDir(folderPath, time.Time{}))
		} else {
			// This is a file
			var remote string
			if dirPath == "" {
				remote = displayName
			} else {
				remote = dirPath + "/" + displayName
			}

			timestamp := convertUnixTimestamp(timestampUnix)
			entries = append(entries, &Object{
				fs:          f,
				remote:      remote,
				mediaKey:    mediaKey,
				size:        sizeBytes,
				modTime:     timestamp,
				userName:    userName,
				displayName: displayName,
				displayPath: dirPath,
			})
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	// Start background metadata population for this user if batching enabled
	if f.opt.BatchSize > 0 {
		go f.populateMetadata(userName)
	}

	return entries, nil
}

// populateMetadata loads size/modtime/mediaKey for files in batches
func (f *Fs) populateMetadata(userName string) {
	batch := f.opt.BatchSize
	if batch <= 0 {
		batch = 1000
	}

	offset := 0
	for {
		query := fmt.Sprintf(`
			SELECT media_key, file_name, COALESCE(name, '') as custom_name, COALESCE(path, '') as custom_path, size_bytes, utc_timestamp
			FROM %s
			ORDER BY file_name
			LIMIT $1 OFFSET $2
		`, f.opt.TableName)

		rows, err := f.db.Query(query, batch, offset)
		if err != nil {
			fs.Errorf(f, "mediavfs: populateMetadata query failed: %v", err)
			return
		}

		count := 0
		for rows.Next() {
			var mediaKey, fileName, customName, customPath string
			var sizeBytes int64
			var timestampUnix int64
			if err := rows.Scan(&mediaKey, &fileName, &customName, &customPath, &sizeBytes, &timestampUnix); err != nil {
				fs.Errorf(f, "mediavfs: populateMetadata scan failed: %v", err)
				continue
			}

			// compute display path/name as in listUserFiles
			var displayName, displayPath string
			// Display name: use 'name' if set, else use 'file_name'
			if customName != "" {
				displayName = customName
			} else {
				displayName = fileName
			}
			// Display path: use 'path' if set, else empty
			if customPath != "" {
				displayPath = strings.Trim(customPath, "/")
			} else {
				displayPath = ""
			}

			var fullPath string
			if displayPath != "" {
				fullPath = displayPath + "/" + displayName
			} else {
				fullPath = displayName
			}

			key := fullPath // No username prefix in single-user model
			obj := &Object{
				fs:          f,
				remote:      key,
				mediaKey:    mediaKey,
				size:        sizeBytes,
				modTime:     convertUnixTimestamp(timestampUnix),
				userName:    userName,
				displayName: displayName,
				displayPath: displayPath,
			}

			f.lazyMu.Lock()
			f.lazyMeta[key] = obj
			f.lazyMu.Unlock()

			count++
		}
		rows.Close()

		if count < batch {
			// finished
			return
		}
		offset += batch
	}
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Use configured user for per-user mounts
	userName := f.opt.User
	filePath := remote

	if filePath == "" {
		return nil, fs.ErrorIsDir
	}

	// Fast path: check if we've loaded metadata asynchronously
	f.lazyMu.RLock()
	if o, ok := f.lazyMeta[remote]; ok {
		f.lazyMu.RUnlock()
		return o, nil
	}
	f.lazyMu.RUnlock()

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

	rows, err := f.db.QueryContext(ctx, query, f.opt.User)
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
		// name=NULL, path=NULL     → /user_name/file_name
		// name=SET,  path=NULL     → /user_name/name
		// name=NULL, path=SET      → /user_name/path/file_name
		// name=SET,  path=SET      → /user_name/path/name
		var displayName, displayPath string

		// Display name: use 'name' if set, else use 'file_name'
		if customName != "" {
			displayName = customName
		} else {
			displayName = fileName
		}

		// Display path: use 'path' if set, else empty (strip slashes)
		if customPath != "" {
			displayPath = strings.Trim(customPath, "/")
		} else {
			displayPath = ""
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

// invalidateCaches clears all internal caches
func (f *Fs) invalidateCaches() {
	f.lazyMu.Lock()
	f.lazyMeta = make(map[string]*Object)
	f.lazyMu.Unlock()

	f.dirMu.Lock()
	for cacheKey := range f.dirCache {
		delete(f.dirCache, cacheKey)
	}
	f.dirMu.Unlock()
}

// ChangeNotify calls the passed function with a path that has had changes.
// The implementation must empty the channel and stop when it is closed.
func (f *Fs) ChangeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	go f.changeNotify(ctx, notify, newInterval)
}

func (f *Fs) changeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	// Initialize lastTimestamp from DB
	var lastTimestamp int64
	row := f.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(MAX(utc_timestamp),0) FROM %s WHERE user_name = $1", f.opt.TableName), f.opt.User)
	if err := row.Scan(&lastTimestamp); err != nil {
		fs.Errorf(f, "mediavfs: ChangeNotify unable to read initial timestamp: %v", err)
		lastTimestamp = 0
	}

	// Get initial interval
	dur, ok := <-newInterval
	if !ok {
		return
	}
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	// Get notify listener events channel (may be nil if listener failed to start)
	var notifyEvents <-chan MediaChangeEvent
	if f.notifyListener != nil {
		notifyEvents = f.notifyListener.Events()
	}

	for {
		select {
		case d, ok := <-newInterval:
			if !ok {
				ticker.Stop()
				return
			}
			fs.Infof(f, "mediavfs: ChangeNotify interval updated to %s", d)
			ticker.Reset(d)

		case event := <-notifyEvents:
			// Real-time notification from PostgreSQL
			fs.Infof(f, "mediavfs: Real-time notification received: %s for %s", event.Action, event.MediaKey)

			// Query the specific media item to get its path
			query := fmt.Sprintf(`
				SELECT file_name, COALESCE(name, '') as custom_name, COALESCE(path, '') as custom_path
				FROM %s WHERE media_key = $1 AND user_name = $2
			`, f.opt.TableName)

			var fileName, customName, customPath string
			err := f.db.QueryRowContext(ctx, query, event.MediaKey, f.opt.User).Scan(&fileName, &customName, &customPath)

			if event.Action == "DELETE" {
				// For deletes, we can't query the row anymore, so just invalidate caches
				f.invalidateCaches()
				// Notify root to trigger full refresh
				notify("", fs.EntryDirectory)
			} else if err == nil {
				// Build the display path
				displayName := customName
				if displayName == "" {
					displayName = fileName
				}
				displayPath := strings.Trim(customPath, "/")

				var fullPath string
				if displayPath != "" {
					fullPath = displayPath + "/" + displayName
					notify(displayPath, fs.EntryDirectory)
				} else {
					fullPath = displayName
				}
				notify(fullPath, fs.EntryObject)
				f.invalidateCaches()
			} else {
				fs.Debugf(f, "mediavfs: Could not find media_key %s: %v", event.MediaKey, err)
			}

		case <-ticker.C:
			// Query for rows newer than lastTimestamp
			query := fmt.Sprintf(`
				SELECT media_key, file_name, COALESCE(name, '') as custom_name, COALESCE(path, '') as custom_path, size_bytes, utc_timestamp
				FROM %s
				WHERE user_name = $1 AND utc_timestamp > $2
				ORDER BY utc_timestamp
			`, f.opt.TableName)

			rows, err := f.db.QueryContext(ctx, query, f.opt.User, lastTimestamp)
			if err != nil {
				fs.Errorf(f, "mediavfs: ChangeNotify query failed: %v", err)
				continue
			}

			maxTs := lastTimestamp
			changedPaths := make(map[string]fs.EntryType) // Collect unique paths to notify
			for rows.Next() {
				var mediaKey, fileName, customName, customPath string
				var sizeBytes, ts int64
				if err := rows.Scan(&mediaKey, &fileName, &customName, &customPath, &sizeBytes, &ts); err != nil {
					fs.Errorf(f, "mediavfs: ChangeNotify scan failed: %v", err)
					continue
				}

				if ts > maxTs {
					maxTs = ts
				}

				var displayName, displayPath string
				// Display name: use 'name' if set, else use 'file_name'
				if customName != "" {
					displayName = customName
				} else {
					displayName = fileName
				}
				// Display path: use 'path' if set, else empty
				if customPath != "" {
					displayPath = strings.Trim(customPath, "/")
				} else {
					displayPath = ""
				}

				var fullPath string
				if displayPath != "" {
					fullPath = displayPath + "/" + displayName
				} else {
					fullPath = displayName
				}

				// Collect unique paths to notify (avoid duplicate notifications)
				if displayPath != "" {
					changedPaths[displayPath] = fs.EntryDirectory
				}
				changedPaths[fullPath] = fs.EntryObject
			}
			rows.Close()

			// Send notifications for all changed paths
			for path, entryType := range changedPaths {
				notify(path, entryType)
			}

			// Invalidate caches if changes were detected
			if len(changedPaths) > 0 {
				// Clear our internal caches
				f.lazyMu.Lock()
				f.lazyMeta = make(map[string]*Object)
				f.lazyMu.Unlock()

				// Invalidate directory cache (single-user per database)
				f.dirMu.Lock()
				// Invalidate all directory caches since changes were detected
				for cacheKey := range f.dirCache {
					delete(f.dirCache, cacheKey)
					fs.Infof(f, "Invalidated dir cache for %s", cacheKey)
				}
				f.dirMu.Unlock()
			}

			lastTimestamp = maxTs
		}
	}
}

// Put uploads a file to Google Photos if upload is enabled
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if !f.opt.EnableUpload {
		return nil, errNotWritable
	}

	// Use configured user for per-user mounts
	userName := f.opt.User
	filePath := src.Remote()

	// Upload to Google Photos
	fs.Infof(f, "Uploading %s to Google Photos for user %s", filePath, userName)
	mediaKey, err := f.UploadWithProgress(ctx, src, in, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to Google Photos: %w", err)
	}

	// Parse path and name for database insert
	var displayPath, displayName string
	if strings.Contains(filePath, "/") {
		lastSlash := strings.LastIndex(filePath, "/")
		displayPath = filePath[:lastSlash]
		displayName = filePath[lastSlash+1:]
	} else {
		displayPath = ""
		displayName = filePath
	}

	// Ensure parent folders exist in database
	if displayPath != "" {
		if err := f.ensureFoldersExist(ctx, userName, displayPath); err != nil {
			return nil, fmt.Errorf("failed to create parent folders: %w", err)
		}
	}

	// Insert into database
	query := fmt.Sprintf(`
		INSERT INTO %s (media_key, file_name, name, path, size_bytes, utc_timestamp, user_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (media_key) DO UPDATE
		SET file_name = EXCLUDED.file_name,
		    name = EXCLUDED.name,
		    path = EXCLUDED.path,
		    size_bytes = EXCLUDED.size_bytes,
		    utc_timestamp = EXCLUDED.utc_timestamp,
		    user_name = EXCLUDED.user_name
	`, f.opt.TableName)

	modTime := src.ModTime(ctx).Unix()
	_, err = f.db.ExecContext(ctx, query, mediaKey, displayName, displayName, displayPath, src.Size(), modTime, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to insert into database: %w", err)
	}

	// Create and return the object
	obj := &Object{
		fs:          f,
		remote:      src.Remote(),
		mediaKey:    mediaKey,
		size:        src.Size(),
		modTime:     src.ModTime(ctx),
		userName:    userName,
		displayName: displayName,
		displayPath: displayPath,
	}

	fs.Infof(f, "Successfully uploaded %s with media key: %s", src.Remote(), mediaKey)
	return obj, nil
}

// PutStream uploads a file with unknown size to Google Photos if upload is enabled
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if !f.opt.EnableUpload {
		return nil, errNotWritable
	}
	return f.Put(ctx, in, src, options...)
}

// ensureFoldersExist creates all parent folders for a given path in the database
// Folders are stored at their PARENT's path with their name as file_name
// For example, to create folder "a/b/c":
//   - folder "a": path='', file_name='a'
//   - folder "b": path='a', file_name='b'
//   - folder "c": path='a/b', file_name='c'
// Uses in-memory cache to avoid redundant DB queries during bulk operations
func (f *Fs) ensureFoldersExist(ctx context.Context, userName string, folderPath string) error {
	if folderPath == "" {
		return nil
	}

	folderPath = strings.Trim(folderPath, "/")

	// Check if this exact folder path is already cached
	cacheKey := userName + ":" + folderPath
	f.folderCacheMu.RLock()
	if f.folderExistsCache[cacheKey] {
		f.folderCacheMu.RUnlock()
		return nil // Already verified/created
	}
	f.folderCacheMu.RUnlock()

	parts := strings.Split(folderPath, "/")

	// Build each level of the path and insert
	parentPath := ""
	for i, part := range parts {
		// The full path of this folder (for media_key)
		var fullPath string
		if i == 0 {
			fullPath = part
		} else {
			fullPath = parentPath + "/" + part
		}

		// Check cache for this specific folder level
		levelCacheKey := userName + ":" + fullPath
		f.folderCacheMu.RLock()
		exists := f.folderExistsCache[levelCacheKey]
		f.folderCacheMu.RUnlock()

		if !exists {
			mediaKey := fmt.Sprintf("folder:%s:%s", userName, fullPath)

			// Insert folder if not exists (type = -1 for folders)
			// path = parent's path, file_name = folder's name
			query := fmt.Sprintf(`
				INSERT INTO %s (media_key, file_name, path, type, user_name)
				VALUES ($1, $2, $3, -1, $4)
				ON CONFLICT (media_key) DO NOTHING
			`, f.opt.TableName)

			_, err := f.db.ExecContext(ctx, query, mediaKey, part, parentPath, userName)
			if err != nil {
				return fmt.Errorf("failed to create folder %s: %w", fullPath, err)
			}

			// Mark this folder as existing in cache
			f.folderCacheMu.Lock()
			f.folderExistsCache[levelCacheKey] = true
			f.folderCacheMu.Unlock()
		}

		// Update parentPath for next iteration
		parentPath = fullPath
	}

	return nil
}

// Mkdir creates a directory in the database (type = -1)
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	// Get the full path (in single-user mode, this is just the folder path)
	folderPath := strings.Trim(path.Join(f.root, dir), "/")
	if folderPath == "" {
		return nil // Root always exists
	}

	// Use configured user
	userName := f.opt.User

	// Create all folders in the path (including parents)
	if err := f.ensureFoldersExist(ctx, userName, folderPath); err != nil {
		return err
	}

	fs.Infof(nil, "mediavfs: created directory: %s", folderPath)
	return nil
}

// Rmdir deletes a folder from the database (only if empty)
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	// Get the full path (in single-user mode, this is just the folder path)
	folderPath := strings.Trim(path.Join(f.root, dir), "/")
	if folderPath == "" {
		return fmt.Errorf("cannot remove root directory")
	}

	// Use configured user
	userName := f.opt.User

	// Check if folder has any files (files have path = folderPath)
	checkFilesQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE user_name = $1 AND path = $2 AND type != -1
	`, f.opt.TableName)

	var count int
	err := f.db.QueryRowContext(ctx, checkFilesQuery, userName, folderPath).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check if folder is empty: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("directory not empty: %s", dir)
	}

	// Check for subfolders (subfolders have path = folderPath, type = -1)
	checkSubfoldersQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE user_name = $1 AND path = $2 AND type = -1
	`, f.opt.TableName)

	err = f.db.QueryRowContext(ctx, checkSubfoldersQuery, userName, folderPath).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for subfolders: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("directory not empty (has subfolders): %s", dir)
	}

	// Delete the folder row (folder's media_key is folder:user:fullPath)
	mediaKey := fmt.Sprintf("folder:%s:%s", userName, folderPath)
	deleteQuery := fmt.Sprintf(`
		DELETE FROM %s WHERE media_key = $1
	`, f.opt.TableName)

	_, err = f.db.ExecContext(ctx, deleteQuery, mediaKey)
	if err != nil {
		return fmt.Errorf("failed to remove folder: %w", err)
	}

	fs.Infof(f, "mediavfs: removed directory: %s", folderPath)
	return nil
}

// Move moves src to this remote
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	// In single-user mode, use f.opt.User as the username
	userName := f.opt.User
	dstPath := strings.Trim(remote, "/")

	fs.Infof(f, "Move called: src=%s, dst=%s for user %s", src.Remote(), remote, userName)

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

	// Ensure parent folders exist in database
	if newPath != "" {
		if err := f.ensureFoldersExist(ctx, userName, newPath); err != nil {
			return nil, fmt.Errorf("failed to create parent folders: %w", err)
		}
	}

	// Update the database
	query := fmt.Sprintf(`
		UPDATE %s
		SET name = $1, path = $2
		WHERE media_key = $3
	`, f.opt.TableName)

	_, err := f.db.ExecContext(ctx, query, newName, newPath, srcObj.mediaKey)
	if err != nil {
		fs.Infof(f, "Move failed: database update error: %v", err)
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	fs.Infof(f, "Move succeeded: updated %s to name=%s, path=%s", srcObj.mediaKey, newName, newPath)

	// Update the object in-place for real-time changes (no DB re-query)
	newObj := &Object{
		fs:          f,
		remote:      remote,
		mediaKey:    srcObj.mediaKey,
		size:        srcObj.size,
		modTime:     srcObj.modTime,
		userName:    userName,
		displayName: newName,
		displayPath: newPath,
	}

	fs.Infof(nil, "mediavfs: moved %s to %s (real-time update)", srcObj.remote, remote)

	return newObj, nil
}

// DirMove moves a directory within the same remote
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	_, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	// In single-user mode, use f.opt.User as the username
	// The remote paths are just the directory paths without username prefix
	userName := f.opt.User
	srcPath := strings.Trim(srcRemote, "/")
	dstPath := strings.Trim(dstRemote, "/")

	if srcPath == "" {
		return fmt.Errorf("cannot move root directory")
	}

	fs.Infof(nil, "mediavfs: DirMove from %s to %s for user %s", srcPath, dstPath, userName)

	// Ensure destination parent folders exist
	if strings.Contains(dstPath, "/") {
		parentPath := dstPath[:strings.LastIndex(dstPath, "/")]
		if err := f.ensureFoldersExist(ctx, userName, parentPath); err != nil {
			return fmt.Errorf("failed to create parent folders: %w", err)
		}
	}

	// Get the new folder name (last component of dstPath)
	dstFolderName := dstPath
	if strings.Contains(dstPath, "/") {
		dstFolderName = dstPath[strings.LastIndex(dstPath, "/")+1:]
	}

	// Get the new parent path (for the folder row)
	dstParentPath := ""
	if strings.Contains(dstPath, "/") {
		dstParentPath = dstPath[:strings.LastIndex(dstPath, "/")]
	}

	// Update the source folder row itself
	srcMediaKey := fmt.Sprintf("folder:%s:%s", userName, srcPath)
	dstMediaKey := fmt.Sprintf("folder:%s:%s", userName, dstPath)

	// Delete any existing folder at destination (in case of overwrite)
	deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE media_key = $1`, f.opt.TableName)
	_, _ = f.db.ExecContext(ctx, deleteQuery, dstMediaKey)

	// Update source folder to destination
	updateFolderQuery := fmt.Sprintf(`
		UPDATE %s
		SET media_key = $1, file_name = $2, path = $3
		WHERE media_key = $4
	`, f.opt.TableName)

	_, err := f.db.ExecContext(ctx, updateFolderQuery, dstMediaKey, dstFolderName, dstParentPath, srcMediaKey)
	if err != nil {
		return fmt.Errorf("failed to move folder row: %w", err)
	}

	// Update all items (files and subfolders) that have path = srcPath
	query1 := fmt.Sprintf(`
		UPDATE %s
		SET path = $1
		WHERE user_name = $2 AND path = $3
	`, f.opt.TableName)

	_, err = f.db.ExecContext(ctx, query1, dstPath, userName, srcPath)
	if err != nil {
		return fmt.Errorf("failed to move directory contents (exact match): %w", err)
	}

	// Update all items with path starting with srcPath/
	srcPathPrefix := srcPath + "/"
	query2 := fmt.Sprintf(`
		UPDATE %s
		SET path = $1 || SUBSTRING(path FROM $2)
		WHERE user_name = $3 AND path LIKE $4
	`, f.opt.TableName)

	_, err = f.db.ExecContext(ctx, query2, dstPath, len(srcPath)+1, userName, srcPathPrefix+"%")
	if err != nil {
		return fmt.Errorf("failed to move directory contents (prefix match): %w", err)
	}

	// Update media_keys for subfolder rows
	updateSubfolderKeysQuery := fmt.Sprintf(`
		UPDATE %s
		SET media_key = 'folder:' || $1 || ':' || path || '/' || file_name
		WHERE user_name = $1 AND type = -1 AND (path = $2 OR path LIKE $3)
	`, f.opt.TableName)

	_, err = f.db.ExecContext(ctx, updateSubfolderKeysQuery, userName, dstPath, dstPath+"/%")
	if err != nil {
		return fmt.Errorf("failed to update subfolder keys: %w", err)
	}

	fs.Infof(nil, "mediavfs: moved directory %s to %s", srcRemote, dstRemote)
	return nil
}

// Shutdown the backend, stopping background sync and closing connections
func (f *Fs) Shutdown(ctx context.Context) error {
	// Stop background sync if running
	if f.syncStop != nil {
		close(f.syncStop)
	}
	if f.db != nil {
		return f.db.Close()
	}
	return nil
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
	// Check if we have cached metadata for this media key FIRST
	cacheKey := o.mediaKey
	cachedMeta, found := o.fs.urlCache.get(cacheKey)

	var resolvedURL, etag string
	var fileSize int64

	if found {
		// Use cached metadata - skip API call entirely
		resolvedURL = cachedMeta.resolvedURL
		etag = cachedMeta.etag
		fileSize = cachedMeta.size
		fs.Infof(nil, "mediavfs: using cached URL for %s (TTL cache hit)", o.mediaKey)
	} else {
		// Cache miss - need to get download URL from API
		fs.Infof(nil, "mediavfs: cache miss for %s, fetching download URL from API", o.mediaKey)

		initialURL, err := o.fs.api.GetDownloadURL(ctx, o.mediaKey)
		if err != nil {
			return nil, fmt.Errorf("failed to get download URL: %w", err)
		}

		// Resolve URL and get ETag via HEAD request
		fs.Infof(nil, "mediavfs: resolving URL and fetching metadata for %s", o.mediaKey)

		// Retry HEAD request with exponential backoff for transient errors
		var headResp *http.Response
		var headErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				sleepTime := time.Duration(1<<uint(attempt-1)) * time.Second
				fs.Infof(nil, "mediavfs: retrying HEAD request after %v (attempt %d/3)", sleepTime, attempt+1)
				time.Sleep(sleepTime)
			}

			headReq, err := http.NewRequestWithContext(ctx, "HEAD", initialURL, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create HEAD request: %w", err)
			}
			headReq.Header.Set("User-Agent", "AndroidDownloadManager/13")

			headResp, headErr = o.fs.httpClient.Do(headReq)
			if headErr == nil {
				// Success - check if we should retry based on status code
				if headResp.StatusCode == http.StatusTooManyRequests ||
					headResp.StatusCode == http.StatusServiceUnavailable ||
					headResp.StatusCode >= 500 {
					headResp.Body.Close()
					headErr = fmt.Errorf("transient HTTP error: %s (status %d)", headResp.Status, headResp.StatusCode)
					continue
				}
				break // Success
			}
		}

		if headErr != nil {
			return nil, fmt.Errorf("failed to execute HEAD request after retries: %w", headErr)
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

		fs.Infof(nil, "mediavfs: cached URL for %s (TTL: %v)", o.mediaKey, cacheTTL)
	}

	// Now make the actual GET request to the resolved URL with retry logic
	var res *http.Response
	var getErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			sleepTime := time.Duration(1<<uint(attempt-1)) * time.Second
			fs.Infof(nil, "mediavfs: retrying GET request after %v (attempt %d/3)", sleepTime, attempt+1)
			time.Sleep(sleepTime)
		}

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
		res, getErr = o.fs.httpClient.Do(req)
		if getErr == nil {
			// Check status code - accept both 200 and 206
			if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusPartialContent {
				break // Success
			}

			// Check if we should retry based on status code
			if res.StatusCode == http.StatusTooManyRequests ||
				res.StatusCode == http.StatusServiceUnavailable ||
				res.StatusCode >= 500 {
				res.Body.Close()
				getErr = fmt.Errorf("transient HTTP error: %s (status %d)", res.Status, res.StatusCode)
				continue
			}

			// Permanent error - don't retry
			res.Body.Close()
			return nil, fmt.Errorf("HTTP error: %s (status %d)", res.Status, res.StatusCode)
		}
	}

	if getErr != nil {
		return nil, fmt.Errorf("failed to open file after retries: %w", getErr)
	}

	fs.Infof(nil, "mediavfs: opened %s with status %d", o.mediaKey, res.StatusCode)

	return res.Body, nil
}

// Update is not supported
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errNotWritable
}

// Remove deletes a file from Google Photos if delete is enabled
func (o *Object) Remove(ctx context.Context) error {
	if !o.fs.opt.EnableDelete {
		fs.Infof(o.fs, "Remove called on %s - delete disabled, file remains in database", o.Remote())
		return nil
	}

	fs.Infof(o.fs, "Removing %s from Google Photos (media_key: %s)", o.Remote(), o.mediaKey)

	// Delete from Google Photos
	if err := o.DeleteFromGPhotos(ctx); err != nil {
		return fmt.Errorf("failed to delete from Google Photos: %w", err)
	}

	// Delete from database
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE media_key = $1
	`, o.fs.opt.TableName)

	_, err := o.fs.db.ExecContext(ctx, query, o.mediaKey)
	if err != nil {
		return fmt.Errorf("failed to delete from database: %w", err)
	}

	fs.Infof(o.fs, "Successfully removed %s", o.Remote())
	return nil
}

// startBackgroundSync starts a goroutine that performs periodic syncs
func (f *Fs) startBackgroundSync() {
	interval := time.Duration(f.opt.SyncInterval) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second // Minimum 10 seconds
	}

	fs.Infof(f, "Starting background sync every %v", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-f.syncStop:
				fs.Infof(f, "Background sync stopped")
				return
			case <-ticker.C:
				fs.Infof(f, "Background sync: syncing from Google Photos...")
				if err := f.SyncFromGooglePhotos(context.Background(), f.opt.User); err != nil {
					fs.Errorf(f, "Background sync failed: %v", err)
				} else {
					fs.Infof(f, "Background sync completed successfully")
				}
			}
		}
	}()
}

// Check the interfaces are satisfied
var (
	_ fs.Fs       = (*Fs)(nil)
	_ fs.Object   = (*Object)(nil)
	_ fs.Mover    = (*Fs)(nil)
	_ fs.Shutdowner = (*Fs)(nil)
)
