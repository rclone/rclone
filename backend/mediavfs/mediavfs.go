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
			Help:     "URL of the token server for Google Photos authentication.\n\nNot required if master_token is provided.",
		}, {
			Name:     "master_token",
			Help:     "Google account master token (aas_et/...) for native authentication.\n\nThis allows native token generation without a separate token server.",
			Advanced: true,
		}, {
			Name:     "private_key_s",
			Help:     "Private key scalar (hex) for token binding.\n\nOptional. Only needed if token binding is required.",
			Advanced: true,
		}, {
			Name:     "android_id",
			Help:     "Android device ID for authentication.\n\nDefaults to a generic ID if not provided.",
			Default:  "",
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
	MasterToken    string `config:"master_token"`
	PrivateKeyS    string `config:"private_key_s"`
	AndroidID      string `config:"android_id"`
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
	dbConnStr   string // stored for lazy notify listener start
	httpClient  *http.Client
	api         *GPhotoAPI // Google Photos API client for download URLs
	urlCache *urlCache
	// lazyMeta stores metadata loaded asynchronously for large listings
	lazyMeta map[string]*Object
	lazyMu   sync.RWMutex
	// prefetchedDirs tracks which directories have been prefetched for NewObject
	prefetchedDirs map[string]bool
	prefetchMu     sync.RWMutex
	// dirCache caches directory listings to avoid reloading on every change
	dirCache map[string]*dirCacheEntry
	dirMu    sync.RWMutex
	// folderExistsCache tracks folders we've already created/verified in this session
	folderExistsCache map[string]bool
	folderCacheMu     sync.RWMutex
	// syncStop channel to stop background sync goroutine
	syncStop chan struct{}
	// notifyListener for PostgreSQL LISTEN/NOTIFY real-time updates (lazy started)
	notifyListener *NotifyListener
	notifyOnce     sync.Once
	// mountReady is closed when the mount is ready (ChangeNotify has been called)
	// Put operations wait for this before uploading
	mountReady     chan struct{}
	mountReadyOnce sync.Once
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

// convertUnixTimestamp converts a Unix timestamp (seconds, milliseconds, or microseconds) to time.Time
// Always returns second precision to match Precision() and avoid modtime comparison issues
func convertUnixTimestamp(timestamp int64) time.Time {
	// Detect timestamp precision and convert to seconds:
	// - Microseconds (> 10^15): divide by 1,000,000
	// - Milliseconds (> 10^12): divide by 1,000
	// - Seconds (< 10^12): use as-is
	// Current Unix time is ~1.7 billion (1.7 × 10^9) seconds
	if timestamp > 1000000000000000 { // > 10^15, likely microseconds
		return time.Unix(timestamp/1000000, 0)
	}
	if timestamp > 1000000000000 { // > 10^12, likely milliseconds
		return time.Unix(timestamp/1000, 0)
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
// Return ModTimeNotSupported to exclude ModTime from VFS cache fingerprints
// This prevents cache invalidation due to timestamp precision differences
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
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

	// Debug: log config options
	fs.Debugf(nil, "mediavfs: NewFs called with name=%s root=%s", name, root)
	fs.Debugf(nil, "mediavfs: Options: User=%s EnableUpload=%v EnableDelete=%v AutoSync=%v",
		opt.User, opt.EnableUpload, opt.EnableDelete, opt.AutoSync)

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
					fs.Debugf(nil, "mediavfs: preserving Range header on redirect: %s", rangeHeader)
				}
				// Preserve If-Range header (critical for ETag validation)
				if ifRangeHeader := originalReq.Header.Get("If-Range"); ifRangeHeader != "" {
					req.Header.Set("If-Range", ifRangeHeader)
				}
				// Preserve User-Agent
				if userAgent := originalReq.Header.Get("User-Agent"); userAgent != "" {
					req.Header.Set("User-Agent", userAgent)
				}
				fs.Debugf(nil, "mediavfs: following redirect from %s to %s",
					via[len(via)-1].URL.Redacted(), req.URL.Redacted())
			}
			return nil
		},
	}

	f := &Fs{
		name:              name,
		root:              root,
		opt:               *opt,
		db:                db,
		dbConnStr:         dbConnStr, // store for lazy notify listener start
		httpClient:        customClient,
		urlCache:          newURLCache(),
		lazyMeta:          make(map[string]*Object),
		prefetchedDirs:    make(map[string]bool),
		dirCache:          make(map[string]*dirCacheEntry),
		folderExistsCache: make(map[string]bool),
		syncStop:          make(chan struct{}),
		mountReady:        make(chan struct{}),
	}

	// Initialize Google Photos API client for download URLs
	// Use native auth if master_token is provided, otherwise fall back to token server
	if opt.MasterToken != "" {
		api, err := NewGPhotoAPIWithNativeAuth(opt.User, opt.TokenServerURL, opt.MasterToken, opt.PrivateKeyS, opt.AndroidID, customClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create API client with native auth: %w", err)
		}
		f.api = api
	} else if opt.TokenServerURL != "" {
		f.api = NewGPhotoAPI(opt.User, opt.TokenServerURL, customClient)
	} else {
		return nil, fmt.Errorf("either master_token or token_server_url must be provided")
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         false,
	}).Fill(ctx, f)

	// Enable ChangeNotify support so vfs can poll this backend
	f.features.ChangeNotify = f.ChangeNotify

	// Initialize database schema
	fs.Debugf(f, "Initializing database schema...")
	if err := f.InitializeDatabase(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Setup PostgreSQL LISTEN/NOTIFY trigger for real-time updates
	// (trigger is created once, listener is started lazily only for mount)
	if err := f.SetupNotifyTrigger(ctx); err != nil {
		fs.Errorf(f, "Failed to setup notify trigger (non-fatal): %v", err)
	}

	// Note: notifyListener is started lazily in ChangeNotify() - only for mount operations

	// Perform initial sync if needed
	if opt.User != "" {
		fs.Debugf(f, "Checking sync state for user: %s", opt.User)
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
			fs.Debugf(f, "User %s already synced", opt.User)
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
	// Signal mount is ready when first List is called
	f.mountReadyOnce.Do(func() {
		fs.Debugf(f, "Mount ready - List called")
		close(f.mountReady)
	})

	// Normalize dir - remove any trailing slashes
	dir = strings.Trim(dir, "/")
	root := strings.Trim(path.Join(f.root, dir), "/")

	// Check cache first
	cacheKey := root
	if cacheKey == "" {
		cacheKey = "." // Use "." for root
	}

	f.dirMu.RLock()
	if entry, ok := f.dirCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		f.dirMu.RUnlock()
		fs.Debugf(f, "List cache hit for %s", dir)
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

	fs.Debugf(f, "List cached %s with %d entries", dir, len(result))
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
	// Exclude trashed items (trash_timestamp > 0)
	// Only show canonical files (is_canonical = true or NULL for backwards compatibility)
	// Trim trailing slashes from path for comparison
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
		WHERE user_name = $1 AND TRIM(TRAILING '/' FROM COALESCE(path, '')) = $2
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
			AND (is_canonical IS NULL OR is_canonical = true)
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
		// Trim any slashes from the display name to prevent path issues
		displayName := strings.Trim(fileName, "/")
		if customName != "" {
			displayName = strings.Trim(customName, "/")
		}

		// Skip entries with empty display names - they would have the same path as the directory
		if displayName == "" {
			continue
		}

		// Calculate path relative to f.root for the remote field
		// dirPath is the full database path, but remote should be relative to f.root
		// Also trim any slashes to prevent double slashes
		relativeDirPath := strings.Trim(dirPath, "/")
		if f.root != "" && strings.HasPrefix(relativeDirPath, f.root) {
			relativeDirPath = strings.TrimPrefix(relativeDirPath, f.root)
			relativeDirPath = strings.Trim(relativeDirPath, "/")
		}

		if itemType == -1 {
			// This is a folder
			var folderPath string
			if relativeDirPath == "" {
				folderPath = displayName
			} else {
				folderPath = relativeDirPath + "/" + displayName
			}
			// Ensure no trailing slashes in folder path
			folderPath = strings.Trim(folderPath, "/")
			entries = append(entries, fs.NewDir(folderPath, time.Time{}))
		} else {
			// This is a file
			var remote string
			if relativeDirPath == "" {
				remote = displayName
			} else {
				remote = relativeDirPath + "/" + displayName
			}
			// Ensure no trailing slashes in file path
			remote = strings.Trim(remote, "/")

			timestamp := convertUnixTimestamp(timestampUnix)
			obj := &Object{
				fs:          f,
				remote:      remote,
				mediaKey:    mediaKey,
				size:        sizeBytes,
				modTime:     timestamp,
				userName:    userName,
				displayName: displayName,
				displayPath: dirPath,
			}
			entries = append(entries, obj)

			// Store in lazyMeta cache so NewObject can find it without a DB query
			f.lazyMu.Lock()
			f.lazyMeta[remote] = obj
			f.lazyMu.Unlock()
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

	// Note: lazyMeta is now populated directly in the loop above
	// No need for background populateMetadata as it's redundant and causes high CPU

	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Use configured user for per-user mounts
	userName := f.opt.User

	// Combine with root to get full path
	fullPath := remote
	if f.root != "" {
		fullPath = strings.Trim(f.root+"/"+remote, "/")
	}
	fullPath = strings.Trim(fullPath, "/")

	if fullPath == "" {
		return nil, fs.ErrorIsDir
	}

	// Fast path: check if we've loaded metadata in cache
	f.lazyMu.RLock()
	if o, ok := f.lazyMeta[remote]; ok {
		f.lazyMu.RUnlock()
		return o, nil
	}
	f.lazyMu.RUnlock()

	// Parse the path to get directory
	var dirPath string
	if idx := strings.LastIndex(fullPath, "/"); idx >= 0 {
		dirPath = fullPath[:idx]
	} else {
		dirPath = ""
	}

	// Check if we've already prefetched this directory
	// The prefetch key is the full database path (not relative)
	prefetchKey := dirPath
	f.prefetchMu.RLock()
	alreadyPrefetched := f.prefetchedDirs[prefetchKey]
	f.prefetchMu.RUnlock()

	if !alreadyPrefetched {
		// Prefetch the entire directory - this populates lazyMeta for all files in the dir
		// This is much faster than individual queries when copying many files
		_, err := f.listUserFiles(ctx, userName, dirPath)
		if err != nil {
			fs.Debugf(f, "NewObject prefetch for %s failed: %v", dirPath, err)
		}

		// Mark directory as prefetched
		f.prefetchMu.Lock()
		f.prefetchedDirs[prefetchKey] = true
		f.prefetchMu.Unlock()

		// Check lazyMeta again after prefetch
		f.lazyMu.RLock()
		if o, ok := f.lazyMeta[remote]; ok {
			f.lazyMu.RUnlock()
			return o, nil
		}
		f.lazyMu.RUnlock()
	}

	// Directory was prefetched but file not found - check if it's a folder
	// This is a single query for the specific file/folder
	// Only show canonical files (is_canonical = true or NULL for backwards compatibility)
	folderQuery := fmt.Sprintf(`
		SELECT type FROM %s
		WHERE user_name = $1
			AND TRIM(BOTH '/' FROM COALESCE(path, '')) || '/' || TRIM(BOTH '/' FROM COALESCE(NULLIF(name, ''), file_name)) = $2
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
			AND (is_canonical IS NULL OR is_canonical = true)
		LIMIT 1
	`, f.opt.TableName)

	var itemType int64
	err := f.db.QueryRowContext(ctx, folderQuery, userName, fullPath).Scan(&itemType)
	if err == nil {
		if itemType == -1 {
			return nil, fs.ErrorIsDir
		}
		// It's a file - this shouldn't happen if prefetch worked correctly
		// but handle it gracefully by doing a full query
	} else {
		// Not found at all
		return nil, fs.ErrorObjectNotFound
	}

	// Fallback: file exists but wasn't in cache (shouldn't normally happen)
	// Only show canonical files (is_canonical = true or NULL for backwards compatibility)
	query := fmt.Sprintf(`
		SELECT
			media_key,
			file_name,
			COALESCE(name, '') as custom_name,
			COALESCE(path, '') as custom_path,
			COALESCE(size_bytes, 0) as size_bytes,
			COALESCE(utc_timestamp, 0) as utc_timestamp
		FROM %s
		WHERE user_name = $1
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
			AND (is_canonical IS NULL OR is_canonical = true)
			AND TRIM(BOTH '/' FROM COALESCE(path, '')) || '/' || TRIM(BOTH '/' FROM COALESCE(NULLIF(name, ''), file_name)) = $2
		LIMIT 1
	`, f.opt.TableName)

	var (
		mediaKey      string
		dbFileName    string
		customName    string
		customPath    string
		sizeBytes     int64
		timestampUnix int64
	)

	err = f.db.QueryRowContext(ctx, query, userName, fullPath).Scan(
		&mediaKey, &dbFileName, &customName, &customPath, &sizeBytes, &timestampUnix,
	)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	timestamp := convertUnixTimestamp(timestampUnix)

	// Determine the display name
	displayName := strings.Trim(dbFileName, "/")
	if customName != "" {
		displayName = strings.Trim(customName, "/")
	}
	displayPath := strings.Trim(customPath, "/")

	obj := &Object{
		fs:          f,
		remote:      remote,
		mediaKey:    mediaKey,
		size:        sizeBytes,
		modTime:     timestamp,
		userName:    userName,
		displayName: displayName,
		displayPath: displayPath,
	}

	// Cache for future lookups
	f.lazyMu.Lock()
	f.lazyMeta[remote] = obj
	f.lazyMu.Unlock()

	return obj, nil
}

// invalidateDirCache invalidates only the cache for a specific directory path
func (f *Fs) invalidateDirCache(dirPath string) {
	cacheKey := dirPath
	if cacheKey == "" {
		cacheKey = "."
	}

	f.dirMu.Lock()
	delete(f.dirCache, cacheKey)
	f.dirMu.Unlock()
}

// removeFromDirCache removes a specific entry from a directory's cache
// Returns true if the entry was found and removed
func (f *Fs) removeFromDirCache(dirPath string, entryName string) bool {
	cacheKey := dirPath
	if cacheKey == "" {
		cacheKey = "."
	}

	f.dirMu.Lock()
	defer f.dirMu.Unlock()

	cached, exists := f.dirCache[cacheKey]
	if !exists || len(cached.entries) == 0 {
		return false
	}

	// Find and remove the entry
	newEntries := make(fs.DirEntries, 0, len(cached.entries))
	found := false
	for _, entry := range cached.entries {
		// Get the base name of the entry
		baseName := entry.Remote()
		if idx := strings.LastIndex(baseName, "/"); idx >= 0 {
			baseName = baseName[idx+1:]
		}
		if baseName != entryName {
			newEntries = append(newEntries, entry)
		} else {
			found = true
		}
	}

	if found {
		f.dirCache[cacheKey] = &dirCacheEntry{
			entries:   newEntries,
			expiresAt: cached.expiresAt,
		}
	}
	return found
}

// addToDirCache adds an entry to a directory's cache
// If the cache doesn't exist, this is a no-op (cache will be populated on next list)
func (f *Fs) addToDirCache(dirPath string, entry fs.DirEntry) {
	cacheKey := dirPath
	if cacheKey == "" {
		cacheKey = "."
	}

	f.dirMu.Lock()
	defer f.dirMu.Unlock()

	cached, exists := f.dirCache[cacheKey]
	if !exists {
		return // Cache doesn't exist, will be populated on next list
	}

	// Add the entry to the cache
	f.dirCache[cacheKey] = &dirCacheEntry{
		entries:   append(cached.entries, entry),
		expiresAt: cached.expiresAt,
	}
}

// ChangeNotify calls the passed function with a path that has had changes.
// The implementation must empty the channel and stop when it is closed.
func (f *Fs) ChangeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	// Signal that mount is ready - this is called by VFS when mount is set up
	f.mountReadyOnce.Do(func() {
		fs.Debugf(f, "Mount ready - ChangeNotify called")
		close(f.mountReady)
	})
	go f.changeNotify(ctx, notify, newInterval)
}

// startNotifyListener lazily starts the PostgreSQL notify listener (only for mount)
func (f *Fs) startNotifyListener(ctx context.Context) {
	f.notifyOnce.Do(func() {
		fs.Debugf(f, "Starting PostgreSQL notify listener (mount detected)")
		f.notifyListener = NewNotifyListener(f.dbConnStr, f.opt.User)
		if err := f.notifyListener.Start(ctx); err != nil {
			fs.Errorf(f, "Failed to start notify listener (falling back to polling): %v", err)
			f.notifyListener = nil
		}
	})
}

func (f *Fs) changeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	// Lazily start notify listener - only called during mount operations
	f.startNotifyListener(ctx)

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

	// Debounce settings for batch processing
	const debounceDelay = 200 * time.Millisecond
	var pendingEvents []MediaChangeEvent
	var debounceTimer *time.Timer
	var debounceChan <-chan time.Time

	// processPendingEvents handles batched events
	processPendingEvents := func() {
		if len(pendingEvents) == 0 {
			return
		}

		fs.Debugf(f, "mediavfs: Processing %d batched notifications", len(pendingEvents))

		// Collect unique directories to invalidate
		dirsToInvalidate := make(map[string]bool)
		hasDelete := false

		for _, event := range pendingEvents {
			if event.Action == "DELETE" {
				hasDelete = true
				continue
			}

			// Query the specific media item to get its path
			query := fmt.Sprintf(`
				SELECT COALESCE(path, '') as custom_path
				FROM %s WHERE media_key = $1 AND user_name = $2
			`, f.opt.TableName)

			var customPath string
			err := f.db.QueryRowContext(ctx, query, event.MediaKey, f.opt.User).Scan(&customPath)
			if err == nil {
				displayPath := strings.Trim(customPath, "/")
				dirsToInvalidate[displayPath] = true
			}
		}

		// Invalidate and notify for each unique directory (once per directory)
		if hasDelete {
			// For deletes, invalidate root
			f.invalidateDirCache("")
			notify("", fs.EntryDirectory)
		}

		for dir := range dirsToInvalidate {
			f.invalidateDirCache(dir)
			notify(dir, fs.EntryDirectory)
		}

		fs.Debugf(f, "mediavfs: Invalidated %d unique directories", len(dirsToInvalidate))

		// Clear pending events
		pendingEvents = nil
	}

	for {
		select {
		case d, ok := <-newInterval:
			if !ok {
				ticker.Stop()
				processPendingEvents() // Process any remaining events
				return
			}
			fs.Debugf(f, "mediavfs: ChangeNotify interval updated to %s", d)
			ticker.Reset(d)

		case event := <-notifyEvents:
			// Add to pending events (debounce)
			pendingEvents = append(pendingEvents, event)

			// Reset/start debounce timer
			if debounceTimer == nil {
				debounceTimer = time.NewTimer(debounceDelay)
				debounceChan = debounceTimer.C
			} else {
				// Reset timer if we're still within the debounce window
				if !debounceTimer.Stop() {
					select {
					case <-debounceTimer.C:
					default:
					}
				}
				debounceTimer.Reset(debounceDelay)
			}

		case <-debounceChan:
			// Debounce timer fired - process all pending events
			processPendingEvents()
			debounceTimer = nil
			debounceChan = nil

		case <-ticker.C:
			// Query for rows newer than lastTimestamp
			query := fmt.Sprintf(`
				SELECT media_key, file_name, COALESCE(name, '') as custom_name, COALESCE(path, '') as custom_path, COALESCE(size_bytes, 0) as size_bytes, COALESCE(utc_timestamp, 0) as utc_timestamp
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

			// Send notifications and invalidate only the affected directories
			affectedDirs := make(map[string]bool)
			for path, entryType := range changedPaths {
				notify(path, entryType)
				// Track which directories need invalidation
				if entryType == fs.EntryDirectory {
					affectedDirs[path] = true
				} else {
					// For files, invalidate their parent directory
					if idx := strings.LastIndex(path, "/"); idx > 0 {
						affectedDirs[path[:idx]] = true
					} else {
						affectedDirs[""] = true // Root directory
					}
				}
			}

			// Only invalidate the specific directories that changed
			for dir := range affectedDirs {
				f.invalidateDirCache(dir)
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

	// Wait for mount to be ready before uploading (max 5 seconds)
	// This ensures the filesystem is accessible before background uploads start
	select {
	case <-f.mountReady:
		// Mount is ready, proceed with upload
	case <-time.After(5 * time.Second):
		// Timeout - proceed anyway (handles non-mount usage like rclone copy)
		fs.Debugf(f, "Mount ready timeout - proceeding with upload")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Skip zero-byte files - Google Photos doesn't accept them
	// Return a dummy object to signal success so VFS removes the file from queue
	if src.Size() == 0 {
		fs.Infof(f, "Skipping zero-byte file: %s (Google Photos doesn't accept empty files)", src.Remote())
		// Return a dummy object representing the "uploaded" empty file
		// This tells VFS the upload succeeded and to remove it from the upload queue
		return &Object{
			fs:       f,
			remote:   src.Remote(),
			size:     0,
			modTime:  src.ModTime(ctx),
			mediaKey: "empty-file-skipped",
		}, nil
	}

	// Use configured user for per-user mounts
	userName := f.opt.User
	filePath := src.Remote()

	// Combine root with the file path to get full destination path
	fullPath := filePath
	if f.root != "" {
		fullPath = f.root + "/" + filePath
	}

	// Upload to Google Photos
	fs.Debugf(f, "Uploading %s to Google Photos for user %s", fullPath, userName)
	mediaKey, err := f.UploadWithProgress(ctx, src, in, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to Google Photos: %w", err)
	}

	// Parse path and name for database insert
	// file_name = original filename only (no path)
	// name = display name (same as file_name for uploads)
	// path = directory path
	fileName := path.Base(src.Remote()) // Original filename only
	var displayPath string
	if strings.Contains(fullPath, "/") {
		lastSlash := strings.LastIndex(fullPath, "/")
		displayPath = fullPath[:lastSlash]
	} else {
		displayPath = ""
	}

	// Ensure parent folders exist in database
	if displayPath != "" {
		if err := f.ensureFoldersExist(ctx, userName, displayPath); err != nil {
			return nil, fmt.Errorf("failed to create parent folders: %w", err)
		}
	}

	// Insert into database
	// file_name = original filename, name = display name (can be customized), path = folder path
	query := fmt.Sprintf(`
		INSERT INTO %s (media_key, file_name, name, path, size_bytes, utc_timestamp, user_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (media_key) DO UPDATE
		SET name = EXCLUDED.name,
		    path = EXCLUDED.path,
		    size_bytes = EXCLUDED.size_bytes,
		    utc_timestamp = EXCLUDED.utc_timestamp,
		    user_name = EXCLUDED.user_name
	`, f.opt.TableName)

	modTime := src.ModTime(ctx).Unix()
	_, err = f.db.ExecContext(ctx, query, mediaKey, fileName, fileName, displayPath, src.Size(), modTime, userName)
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
		displayName: fileName,
		displayPath: displayPath,
	}

	// Add to destination directory cache instead of invalidating
	f.addToDirCache(displayPath, obj)

	fs.Infof(f, "Successfully uploaded %s with media key: %s", fullPath, mediaKey)
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

	// Add the new folder to parent directory cache instead of invalidating
	parentPath := ""
	if strings.Contains(folderPath, "/") {
		parentPath = folderPath[:strings.LastIndex(folderPath, "/")]
	}
	f.addToDirCache(parentPath, fs.NewDir(folderPath, time.Time{}))

	fs.Debugf(nil, "mediavfs: created directory: %s", folderPath)
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
	// Exclude trashed items, trim trailing slashes from path
	checkFilesQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE user_name = $1 AND TRIM(TRAILING '/' FROM COALESCE(path, '')) = $2 AND type != -1
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
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
	// Exclude trashed items, trim trailing slashes from path
	checkSubfoldersQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE user_name = $1 AND TRIM(TRAILING '/' FROM COALESCE(path, '')) = $2 AND type = -1
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
	`, f.opt.TableName)

	err = f.db.QueryRowContext(ctx, checkSubfoldersQuery, userName, folderPath).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for subfolders: %w", err)
	}

	if count > 0 {
		// Check if any subfolder contains visible items
		// If all subfolders are empty (only contain hidden items), we can delete them
		hasVisibleSubfolderContent := false
		subfolderQuery := fmt.Sprintf(`
			SELECT media_key FROM %s
			WHERE user_name = $1 AND TRIM(TRAILING '/' FROM COALESCE(path, '')) = $2 AND type = -1
				AND (trash_timestamp IS NULL OR trash_timestamp = 0)
		`, f.opt.TableName)
		subRows, err := f.db.QueryContext(ctx, subfolderQuery, userName, folderPath)
		if err != nil {
			return fmt.Errorf("failed to query subfolders: %w", err)
		}
		defer subRows.Close()

		var emptySubfolders []string
		for subRows.Next() {
			var subMediaKey string
			if err := subRows.Scan(&subMediaKey); err != nil {
				continue
			}
			// Extract subfolder path from media_key (format: folder:user:path)
			parts := strings.SplitN(subMediaKey, ":", 3)
			if len(parts) != 3 {
				continue
			}
			subfolderPath := parts[2]

			// Check if this subfolder has any visible content
			visibleQuery := fmt.Sprintf(`
				SELECT COUNT(*) FROM %s
				WHERE user_name = $1 AND TRIM(TRAILING '/' FROM COALESCE(path, '')) = $2
					AND (trash_timestamp IS NULL OR trash_timestamp = 0)
			`, f.opt.TableName)
			var visibleCount int
			f.db.QueryRowContext(ctx, visibleQuery, userName, subfolderPath).Scan(&visibleCount)
			if visibleCount > 0 {
				hasVisibleSubfolderContent = true
				break
			}
			emptySubfolders = append(emptySubfolders, subMediaKey)
		}

		if hasVisibleSubfolderContent {
			return fmt.Errorf("directory not empty (has subfolders): %s", dir)
		}

		// Delete empty subfolders
		for _, subKey := range emptySubfolders {
			delQuery := fmt.Sprintf(`DELETE FROM %s WHERE media_key = $1`, f.opt.TableName)
			f.db.ExecContext(ctx, delQuery, subKey)
		}
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

	// Remove the folder from parent directory's cache
	parentPath := ""
	folderName := folderPath
	if strings.Contains(folderPath, "/") {
		parentPath = folderPath[:strings.LastIndex(folderPath, "/")]
		folderName = folderPath[strings.LastIndex(folderPath, "/")+1:]
	}
	f.removeFromDirCache(parentPath, folderName)

	// Remove from folderExistsCache
	cacheKey := userName + ":" + folderPath
	f.folderCacheMu.Lock()
	delete(f.folderExistsCache, cacheKey)
	f.folderCacheMu.Unlock()

	fs.Debugf(f, "mediavfs: removed directory: %s", folderPath)
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

	fs.Debugf(f, "Move: %s -> %s", src.Remote(), remote)

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
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

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

	// Update caches directly instead of invalidating (avoids full re-fetch)
	// Remove from source directory cache
	f.removeFromDirCache(srcObj.displayPath, srcObj.displayName)
	// Add to destination directory cache
	f.addToDirCache(newPath, newObj)

	fs.Debugf(nil, "Move completed: %s -> %s", srcObj.remote, remote)

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

	fs.Debugf(nil, "DirMove: %s -> %s", srcPath, dstPath)

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
	// Trim trailing slashes from path for comparison
	query1 := fmt.Sprintf(`
		UPDATE %s
		SET path = $1
		WHERE user_name = $2 AND TRIM(TRAILING '/' FROM COALESCE(path, '')) = $3
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
	// Exclude trashed items
	updateSubfolderKeysQuery := fmt.Sprintf(`
		UPDATE %s
		SET media_key = 'folder:' || $1 || ':' || path || '/' || file_name
		WHERE user_name = $1 AND type = -1 AND (path = $2 OR path LIKE $3)
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
	`, f.opt.TableName)

	_, err = f.db.ExecContext(ctx, updateSubfolderKeysQuery, userName, dstPath, dstPath+"/%")
	if err != nil {
		return fmt.Errorf("failed to update subfolder keys: %w", err)
	}

	// Invalidate caches for source and destination parent directories
	srcParentPath := ""
	if strings.Contains(srcPath, "/") {
		srcParentPath = srcPath[:strings.LastIndex(srcPath, "/")]
	}
	f.invalidateDirCache(srcParentPath) // Source parent where folder was listed

	f.invalidateDirCache(dstParentPath) // Destination parent where folder now appears

	// Also invalidate caches for the source and destination directories themselves
	f.invalidateDirCache(srcPath)
	f.invalidateDirCache(dstPath)

	// Update folderExistsCache - remove old path, add new path
	f.folderCacheMu.Lock()
	delete(f.folderExistsCache, userName+":"+srcPath)
	f.folderExistsCache[userName+":"+dstPath] = true
	f.folderCacheMu.Unlock()

	fs.Debugf(nil, "DirMove completed: %s -> %s", srcRemote, dstRemote)
	return nil
}

// Shutdown the backend, stopping background sync and closing connections
func (f *Fs) Shutdown(ctx context.Context) error {
	// Stop background sync if running
	if f.syncStop != nil {
		close(f.syncStop)
	}

	// Stop the notify listener
	if f.notifyListener != nil {
		if err := f.notifyListener.Stop(); err != nil {
			fs.Errorf(f, "Failed to stop notify listener: %v", err)
		}
	}

	// Close database connection
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
		fs.Debugf(nil, "URL cache hit for %s", o.mediaKey)
	} else {
		// Cache miss - need to get download URL from API
		fs.Debugf(nil, "URL cache miss for %s", o.mediaKey)

		initialURL, err := o.fs.api.GetDownloadURL(ctx, o.mediaKey)
		if err != nil {
			// If media not found (404), delete from database and return not found error
			if errors.Is(err, ErrMediaNotFound) {
				fs.Debugf(nil, "mediavfs: media item %s not found in Google Photos, removing from database", o.mediaKey)
				deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE media_key = $1`, o.fs.opt.TableName)
				_, delErr := o.fs.db.ExecContext(ctx, deleteQuery, o.mediaKey)
				if delErr != nil {
					fs.Errorf(nil, "mediavfs: failed to delete missing media %s from database: %v", o.mediaKey, delErr)
				} else {
					// Invalidate cache for the directory
					o.fs.removeFromDirCache(o.displayPath, o.displayName)
				}
				return nil, fs.ErrorObjectNotFound
			}
			return nil, fmt.Errorf("failed to get download URL: %w", err)
		}

		// Resolve URL and get ETag via HEAD request
		fs.Debugf(nil, "Resolving URL for %s", o.mediaKey)

		// Retry HEAD request with exponential backoff for transient errors
		var headResp *http.Response
		var headErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				sleepTime := time.Duration(1<<uint(attempt-1)) * time.Second
				fs.Debugf(nil, "Retrying HEAD after %v", sleepTime)
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

		fs.Debugf(nil, "Cached URL for %s", o.mediaKey)
	}

	// Now make the actual GET request to the resolved URL with retry logic
	var res *http.Response
	var getErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			sleepTime := time.Duration(1<<uint(attempt-1)) * time.Second
			fs.Debugf(nil, "Retrying GET after %v", sleepTime)
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

	fs.Debugf(nil, "Opened %s", o.mediaKey)

	return res.Body, nil
}

// Update replaces the data in the object
// For Google Photos, we can't truly update - if sizes match, skip; otherwise error
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// If the file already exists with the same size, consider it up-to-date
	// Google Photos doesn't support true updates, and modtime will always differ
	if o.size == src.Size() {
		fs.Debugf(o, "File exists with same size (%d bytes), skipping update", o.size)
		return nil
	}

	// Sizes differ - can't update in place for Google Photos
	// User would need to delete and re-upload
	return fmt.Errorf("cannot update existing file (size differs: local=%d remote=%d) - delete and re-upload instead", src.Size(), o.size)
}

// Remove deletes a file - checks for duplicates first to prevent data loss
// If file has duplicates (same dedup_key), only hides locally (trash_timestamp = -1)
// If file is unique, deletes from Google Photos server and database
func (o *Object) Remove(ctx context.Context) error {
	if !o.fs.opt.EnableDelete {
		fs.Debugf(o.fs, "Remove disabled for %s", o.Remote())
		return nil
	}

	fs.Debugf(o.fs, "Checking duplicates before removing %s", o.Remote())

	// Get the dedup_key for this file
	var dedupKey string
	dedupQuery := fmt.Sprintf(`SELECT COALESCE(dedup_key, '') FROM %s WHERE media_key = $1`, o.fs.opt.TableName)
	err := o.fs.db.QueryRowContext(ctx, dedupQuery, o.mediaKey).Scan(&dedupKey)
	if err != nil {
		return fmt.Errorf("failed to get dedup_key: %w", err)
	}

	// Count how many files share this dedup_key (excluding already trashed files)
	var duplicateCount int
	if dedupKey != "" {
		countQuery := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s
			WHERE dedup_key = $1
			AND (trash_timestamp IS NULL OR trash_timestamp = 0)
		`, o.fs.opt.TableName)
		err = o.fs.db.QueryRowContext(ctx, countQuery, dedupKey).Scan(&duplicateCount)
		if err != nil {
			return fmt.Errorf("failed to count duplicates: %w", err)
		}
	} else {
		// No dedup_key means we can't check for duplicates, treat as unique
		duplicateCount = 1
	}

	// Remove from caches first
	o.fs.removeFromDirCache(o.displayPath, o.displayName)
	o.fs.lazyMu.Lock()
	delete(o.fs.lazyMeta, o.remote)
	o.fs.lazyMu.Unlock()

	if duplicateCount <= 1 {
		// This is the only copy - safe to delete from Google Photos
		fs.Debugf(o.fs, "File %s is unique (no duplicates), deleting from Google Photos", o.Remote())

		if dedupKey != "" {
			// Delete from Google Photos
			if err := o.fs.DeleteFromGPhotos(ctx, []string{dedupKey}, o.userName); err != nil {
				fs.Errorf(o.fs, "Failed to delete from Google Photos: %v", err)
				// Still mark as deleted locally even if Google delete fails
			}
		}

		// Delete from database
		deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE media_key = $1`, o.fs.opt.TableName)
		_, err = o.fs.db.ExecContext(ctx, deleteQuery, o.mediaKey)
		if err != nil {
			return fmt.Errorf("failed to delete from database: %w", err)
		}

		fs.Infof(o.fs, "Deleted %s from Google Photos and database", o.Remote())
	} else {
		// Multiple copies exist - only hide locally to prevent data loss
		fs.Debugf(o.fs, "File %s has %d copies, hiding locally only (not deleting from Google Photos)", o.Remote(), duplicateCount)

		// Mark for local deletion by setting trash_timestamp = -1 and path = NULL
		// Setting path = NULL ensures the file doesn't appear in parent directory listings,
		// allowing parent folders to be removed without "folder not empty" errors
		query := fmt.Sprintf(`
			UPDATE %s SET trash_timestamp = -1, path = NULL
			WHERE media_key = $1
		`, o.fs.opt.TableName)

		_, err = o.fs.db.ExecContext(ctx, query, o.mediaKey)
		if err != nil {
			return fmt.Errorf("failed to mark for deletion: %w", err)
		}

		fs.Infof(o.fs, "Hidden %s locally (has %d duplicates, not deleted from Google Photos)", o.Remote(), duplicateCount)
	}

	return nil
}

// startBackgroundSync starts a goroutine that performs periodic syncs
func (f *Fs) startBackgroundSync() {
	interval := time.Duration(f.opt.SyncInterval) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second // Minimum 10 seconds
	}

	fs.Debugf(f, "Starting background sync every %v", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-f.syncStop:
				return
			case <-ticker.C:
				// Note: We intentionally do NOT process pending deletions here.
				// Files marked with trash_timestamp = -1 are hidden locally only.
				// We don't delete from Google Photos to prevent data loss
				// (deleting one file might remove all duplicates).

				// Sync from Google Photos
				if err := f.SyncFromGooglePhotos(context.Background(), f.opt.User); err != nil {
					fs.Errorf(f, "Background sync failed: %v", err)
				}
			}
		}
	}()
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order. If
// callback returns an error then the listing will stop
// immediately.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	// Signal mount is ready when ListR is called
	f.mountReadyOnce.Do(func() {
		fs.Debugf(f, "Mount ready - ListR called")
		close(f.mountReady)
	})

	userName := f.opt.User
	rootPath := strings.Trim(path.Join(f.root, dir), "/")

	fs.Debugf(f, "ListR called for dir=%s rootPath=%s", dir, rootPath)

	// Query ALL files and folders recursively under rootPath
	// Files have path starting with rootPath (or equal to rootPath)
	// This is a single query that gets everything
	// Only show canonical files (is_canonical = true or NULL for backwards compatibility)
	var query string
	var rows *sql.Rows
	var err error

	if rootPath == "" {
		// List everything for this user
		query = fmt.Sprintf(`
			SELECT
				media_key,
				file_name,
				COALESCE(name, '') as custom_name,
				COALESCE(path, '') as custom_path,
				COALESCE(type, 0) as item_type,
				COALESCE(size_bytes, 0) as size_bytes,
				COALESCE(utc_timestamp, 0) as utc_timestamp
			FROM %s
			WHERE user_name = $1
				AND (trash_timestamp IS NULL OR trash_timestamp = 0)
				AND (is_canonical IS NULL OR is_canonical = true)
			ORDER BY path, type ASC, file_name ASC
		`, f.opt.TableName)
		rows, err = f.db.QueryContext(ctx, query, userName)
	} else {
		// List everything under rootPath (path = rootPath OR path starts with rootPath/)
		query = fmt.Sprintf(`
			SELECT
				media_key,
				file_name,
				COALESCE(name, '') as custom_name,
				COALESCE(path, '') as custom_path,
				COALESCE(type, 0) as item_type,
				COALESCE(size_bytes, 0) as size_bytes,
				COALESCE(utc_timestamp, 0) as utc_timestamp
			FROM %s
			WHERE user_name = $1
				AND (TRIM(TRAILING '/' FROM COALESCE(path, '')) = $2
				     OR COALESCE(path, '') LIKE $3)
				AND (trash_timestamp IS NULL OR trash_timestamp = 0)
				AND (is_canonical IS NULL OR is_canonical = true)
			ORDER BY path, type ASC, file_name ASC
		`, f.opt.TableName)
		rows, err = f.db.QueryContext(ctx, query, userName, rootPath, rootPath+"/%")
	}

	if err != nil {
		return fmt.Errorf("ListR query failed: %w", err)
	}
	defer rows.Close()

	// Collect entries in batches to send to callback
	var entries fs.DirEntries
	seenDirs := make(map[string]bool) // Track directories we've already added
	const batchSize = 1000

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
			return fmt.Errorf("ListR scan failed: %w", err)
		}

		// Display name: use 'name' if set, else use 'file_name'
		displayName := strings.Trim(fileName, "/")
		if customName != "" {
			displayName = strings.Trim(customName, "/")
		}

		if displayName == "" {
			continue
		}

		// Calculate path relative to f.root
		dbPath := strings.Trim(customPath, "/")
		var relativePath string
		if f.root == "" {
			relativePath = dbPath
		} else if dbPath == f.root {
			relativePath = ""
		} else if strings.HasPrefix(dbPath, f.root+"/") {
			relativePath = strings.TrimPrefix(dbPath, f.root+"/")
		} else if !strings.HasPrefix(dbPath, f.root) {
			// Path doesn't match our root, skip
			continue
		} else {
			relativePath = dbPath
		}

		// Build the remote path
		var remote string
		if relativePath == "" {
			remote = displayName
		} else {
			remote = relativePath + "/" + displayName
		}

		// Also add parent directories that we haven't seen yet
		if relativePath != "" {
			parts := strings.Split(relativePath, "/")
			for i := range parts {
				dirPath := strings.Join(parts[:i+1], "/")
				if !seenDirs[dirPath] {
					seenDirs[dirPath] = true
					entries = append(entries, fs.NewDir(dirPath, time.Time{}))
				}
			}
		}

		if itemType == -1 {
			// This is a folder
			if !seenDirs[remote] {
				seenDirs[remote] = true
				entries = append(entries, fs.NewDir(remote, time.Time{}))
			}
		} else {
			// This is a file
			timestamp := convertUnixTimestamp(timestampUnix)
			obj := &Object{
				fs:          f,
				remote:      remote,
				mediaKey:    mediaKey,
				size:        sizeBytes,
				modTime:     timestamp,
				userName:    userName,
				displayName: displayName,
				displayPath: dbPath,
			}
			entries = append(entries, obj)

			// Store in lazyMeta cache for fast NewObject lookups
			f.lazyMu.Lock()
			f.lazyMeta[remote] = obj
			f.lazyMu.Unlock()
		}

		// Send batch to callback when we reach batchSize
		if len(entries) >= batchSize {
			err = callback(entries)
			if err != nil {
				return err
			}
			entries = nil
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("ListR iteration error: %w", err)
	}

	// Send remaining entries
	if len(entries) > 0 {
		err = callback(entries)
		if err != nil {
			return err
		}
	}

	fs.Debugf(f, "ListR completed")
	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs         = (*Fs)(nil)
	_ fs.Object     = (*Object)(nil)
	_ fs.Mover      = (*Fs)(nil)
	_ fs.Shutdowner = (*Fs)(nil)
	_ fs.ListRer    = (*Fs)(nil)
)
