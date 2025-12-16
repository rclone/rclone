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
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	DBConnection string `config:"db_connection"`
	DownloadURL  string `config:"download_url"`
	TableName    string `config:"table_name"`
	BatchSize    int    `config:"batch_size"`
}

// Fs represents a connection to the media database
type Fs struct {
	name        string
	root        string
	opt         Options
	features    *fs.Features
	db          *sql.DB
	httpClient  *http.Client
	urlCache    *urlCache
	virtualDirs map[string]bool // Track virtual directories created in memory
	vdirMu      sync.RWMutex    // Mutex for virtualDirs
	// lazyMeta stores metadata loaded asynchronously for large listings
	lazyMeta map[string]*Object
	lazyMu   sync.RWMutex
	// dirCache caches directory listings to avoid reloading on every change
	dirCache map[string]*dirCacheEntry
	dirMu    sync.RWMutex
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
		name:        name,
		root:        root,
		opt:         *opt,
		db:          db,
		httpClient:  customClient,
		urlCache:    newURLCache(),
		virtualDirs: make(map[string]bool),
		lazyMeta:    make(map[string]*Object),
		dirCache:    make(map[string]*dirCacheEntry),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         false,
	}).Fill(ctx, f)

	// Enable ChangeNotify support so vfs can poll this backend
	f.features.ChangeNotify = f.ChangeNotify

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
		fs.Debugf(f, "List cache hit for %s", dir)
		return entry.entries, nil
	}
	f.dirMu.RUnlock()

	// Cache miss or expired, load from database
	var result fs.DirEntries
	if root == "" {
		result, err = f.listUsers(ctx)
	} else {
		userName, subPath := splitUserPath(root)
		if subPath == "" {
			result, err = f.listUserFiles(ctx, userName, "")
		} else {
			result, err = f.listUserFiles(ctx, userName, subPath)
		}
	}

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

// listUsers returns a list of all unique usernames as directories
func (f *Fs) listUsers(ctx context.Context) (entries fs.DirEntries, err error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT user_name
		FROM %s
		WHERE user_name IS NOT NULL
		ORDER BY user_name
	`, f.opt.TableName)

	rows, err := f.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var userName string
		if err := rows.Scan(&userName); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		entries = append(entries, fs.NewDir(userName, time.Time{}))
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return entries, nil
}

// listUserFiles lists files and directories for a specific user and path
func (f *Fs) listUserFiles(ctx context.Context, userName string, dirPath string) (entries fs.DirEntries, err error) {
	// Fast initial query: get identifying fields only so listing is quick
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
	defer rows.Close()

	// Track directories we've already added
	dirsSeen := make(map[string]bool)

	// Normalize dirPath for comparison
	dirPath = strings.Trim(dirPath, "/")
	var dirPrefix string
	if dirPath != "" {
		dirPrefix = dirPath + "/"
	}

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

		// Determine the display name and path
		var displayName, displayPath string

		if customName != "" {
			// Use custom name if set
			displayName = customName
		} else {
			// Extract name from fileName (handles paths in file_name)
			_, displayName = extractPathAndName(fileName)
		}

		if customPath != "" {
			// Use custom path if set
			displayPath = strings.Trim(customPath, "/")
		} else {
			// Extract path from fileName if it contains "/"
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

		// Check if this file is in the current directory or a subdirectory
		if dirPath == "" {
			// We're at the root of the user's directory
			// Check if file is directly in root or in a subdirectory
			if strings.Contains(fullPath, "/") {
				// This is in a subdirectory - add all intermediate directories
				pathParts := strings.Split(fullPath, "/")
				for i := 0; i < len(pathParts)-1; i++ { // -1 because last part is the filename
					dirName := strings.Join(pathParts[:i+1], "/")
					if !dirsSeen[dirName] {
						entries = append(entries, fs.NewDir(userName+"/"+dirName, time.Time{}))
						dirsSeen[dirName] = true
					}
				}
			} else {
				// This is a file directly in the root
				remote := userName + "/" + fullPath
				entries = append(entries, &Object{
					fs:          f,
					remote:      remote,
					mediaKey:    mediaKey,
					size:        sizeBytes,
					modTime:     timestamp,
					userName:    userName,
					displayName: displayName,
					displayPath: displayPath,
				})
			}
		} else {
			// We're in a specific subdirectory
			// Check if file is in this directory or a deeper subdirectory
			if !strings.HasPrefix(fullPath, dirPrefix) {
				continue // Not in this directory
			}

			remainder := strings.TrimPrefix(fullPath, dirPrefix)
			if strings.Contains(remainder, "/") {
				// This is in a subdirectory
				subDir := strings.SplitN(remainder, "/", 2)[0]
				fullSubDir := dirPath + "/" + subDir
				if !dirsSeen[fullSubDir] {
					entries = append(entries, fs.NewDir(userName+"/"+fullSubDir, time.Time{}))
					dirsSeen[fullSubDir] = true
				}
			} else {
				// This is a file directly in this directory
				remote := userName + "/" + fullPath
				entries = append(entries, &Object{
					fs:          f,
					remote:      remote,
					mediaKey:    mediaKey,
					size:        sizeBytes,
					modTime:     timestamp,
					userName:    userName,
					displayName: displayName,
					displayPath: displayPath,
				})
			}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating files: %w", err)
	}

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

	// Start background metadata population for this user if batching enabled
	if f.opt.BatchSize > 0 {
		go f.populateMetadata(userName)
	}

	return entries, nil
}

// populateMetadata loads size/modtime/mediaKey for a user's files in batches
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
			WHERE user_name = $1
			ORDER BY file_name
			LIMIT $2 OFFSET $3
		`, f.opt.TableName)

		rows, err := f.db.Query(query, userName, batch, offset)
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

			var fullPath string
			if displayPath != "" {
				fullPath = displayPath + "/" + displayName
			} else {
				fullPath = displayName
			}

			key := userName + "/" + fullPath
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
	userName, filePath := splitUserPath(remote)
	if userName == "" || filePath == "" {
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

// ChangeNotify calls the passed function with a path that has had changes.
// The implementation must empty the channel and stop when it is closed.
func (f *Fs) ChangeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	go f.changeNotify(ctx, notify, newInterval)
}

func (f *Fs) changeNotify(ctx context.Context, notify func(string, fs.EntryType), newInterval <-chan time.Duration) {
	// Initialize lastTimestamp from DB
	var lastTimestamp int64
	row := f.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(MAX(utc_timestamp),0) FROM %s", f.opt.TableName))
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

	for {
		select {
		case d, ok := <-newInterval:
			if !ok {
				ticker.Stop()
				return
			}
			fs.Debugf(f, "mediavfs: ChangeNotify interval updated to %s", d)
			ticker.Reset(d)

		case <-ticker.C:
			// Query for rows newer than lastTimestamp
			query := fmt.Sprintf(`
				SELECT media_key, file_name, COALESCE(name, '') as custom_name, COALESCE(path, '') as custom_path, size_bytes, utc_timestamp, user_name
				FROM %s
				WHERE utc_timestamp > $1
				ORDER BY utc_timestamp
			`, f.opt.TableName)

			rows, err := f.db.QueryContext(ctx, query, lastTimestamp)
			if err != nil {
				fs.Errorf(f, "mediavfs: ChangeNotify query failed: %v", err)
				continue
			}

			maxTs := lastTimestamp
			changedUsers := make(map[string]bool)
			changedPaths := make(map[string]fs.EntryType) // Collect unique paths to notify
			for rows.Next() {
				var mediaKey, fileName, customName, customPath, uName string
				var sizeBytes, ts int64
				if err := rows.Scan(&mediaKey, &fileName, &customName, &customPath, &sizeBytes, &ts, &uName); err != nil {
					fs.Errorf(f, "mediavfs: ChangeNotify scan failed: %v", err)
					continue
				}

				if ts > maxTs {
					maxTs = ts
				}

				changedUsers[uName] = true

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

				var fullPath string
				if displayPath != "" {
					fullPath = displayPath + "/" + displayName
				} else {
					fullPath = displayName
				}

				// Collect unique paths to notify (avoid duplicate notifications)
				if displayPath != "" {
					changedPaths[uName+"/"+displayPath] = fs.EntryDirectory
				}
				changedPaths[uName+"/"+fullPath] = fs.EntryObject
			}
			rows.Close()

			// Send notifications for all changed paths
			for path, entryType := range changedPaths {
				notify(path, entryType)
			}

			// Invalidate caches if changes were detected
			if len(changedUsers) > 0 {
				// Clear our internal caches
				f.lazyMu.Lock()
				f.lazyMeta = make(map[string]*Object)
				f.lazyMu.Unlock()

				// Invalidate directory cache for affected users
				f.dirMu.Lock()
				for userName := range changedUsers {
					// Invalidate user's root directory and any subdirectories that might be affected
					for cacheKey := range f.dirCache {
						if strings.HasPrefix(cacheKey, userName+"/") || cacheKey == userName {
							delete(f.dirCache, cacheKey)
							fs.Debugf(f, "Invalidated dir cache for %s", cacheKey)
						}
					}
				}
				// Also invalidate root cache in case new users were added
				delete(f.dirCache, ".")
				f.dirMu.Unlock()
			}

			lastTimestamp = maxTs
		}
	}
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

	fs.Debugf(nil, "mediavfs: created virtual directory: %s", fullPath)
	return nil // Virtual directories, always succeed
}

// Rmdir is not supported
// Rmdir is allowed for directory operations but doesn't delete from database
// (directories are virtual user folders that shouldn't be removed)
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "Rmdir called on %s - allowed for directory operations, directory remains in database", dir)
	// Don't actually delete from database - just allow the operation
	return nil
}

// Move moves src to this remote
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	fs.Debugf(f, "Move called: src=%s, dst=%s", src.Remote(), remote)

	// Check that both source and destination are for the same user
	srcUser, _ := splitUserPath(src.Remote())
	dstUser, dstPath := splitUserPath(remote)

	if srcUser != dstUser {
		fs.Debugf(f, "Move failed: cross-user move not allowed")
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
			fs.Debugf(nil, "mediavfs: removed virtual directory after file move: %s", vdirPath)
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
		fs.Debugf(f, "Move failed: database update error: %v", err)
		return nil, fmt.Errorf("failed to move file: %w", err)
	}

	fs.Debugf(f, "Move succeeded: updated %s to name=%s, path=%s", srcObj.mediaKey, newName, newPath)

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

	fs.Debugf(nil, "mediavfs: moved %s to %s (real-time update)", srcObj.remote, remote)

	return newObj, nil
}

// DirMove moves a directory within the same remote
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	_, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	// Check that both source and destination are for the same user
	srcUser, srcPath := splitUserPath(srcRemote)
	dstUser, dstPath := splitUserPath(dstRemote)

	if srcUser != dstUser {
		return errCrossUser
	}

	// Normalize paths
	srcPath = strings.Trim(srcPath, "/")
	dstPath = strings.Trim(dstPath, "/")

	if srcPath == "" {
		return fmt.Errorf("cannot move root directory")
	}

	// Update virtual directories
	f.vdirMu.Lock()
	// Remove old virtual directory
	if f.virtualDirs[srcRemote] {
		delete(f.virtualDirs, srcRemote)
	}
	// Add new virtual directory
	f.virtualDirs[dstRemote] = true
	f.vdirMu.Unlock()

	// Update all files in the database that have paths starting with srcPath
	srcPathPrefix := srcPath + "/"

	// First, update files with exact path match
	query1 := fmt.Sprintf(`
		UPDATE %s
		SET path = $1
		WHERE user_name = $2 AND path = $3
	`, f.opt.TableName)

	_, err := f.db.ExecContext(ctx, query1, dstPath, dstUser, srcPath)
	if err != nil {
		return fmt.Errorf("failed to move directory (exact match): %w", err)
	}

	// Then, update files with path prefix match
	query2 := fmt.Sprintf(`
		UPDATE %s
		SET path = REPLACE(path, $1, $2)
		WHERE user_name = $3 AND path LIKE $4
	`, f.opt.TableName)

	_, err = f.db.ExecContext(ctx, query2, srcPathPrefix, dstPath+"/", dstUser, srcPathPrefix+"%")
	if err != nil {
		return fmt.Errorf("failed to move directory (prefix match): %w", err)
	}

	fs.Debugf(nil, "mediavfs: moved directory %s to %s", srcRemote, dstRemote)
	return nil
}

// Shutdown the backend
func (f *Fs) Shutdown(ctx context.Context) error {
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
		fs.Debugf(nil, "mediavfs: using cached URL and ETag for %s", o.mediaKey)
	} else {
		// First time accessing this file - resolve URL and get ETag via HEAD request
		fs.Debugf(nil, "mediavfs: resolving URL and fetching metadata for %s", o.mediaKey)

		// Retry HEAD request with exponential backoff for transient errors
		var headResp *http.Response
		var headErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				sleepTime := time.Duration(1<<uint(attempt-1)) * time.Second
				fs.Debugf(nil, "mediavfs: retrying HEAD request after %v (attempt %d/3)", sleepTime, attempt+1)
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

		fs.Debugf(nil, "mediavfs: cached URL and ETag=%s for %s", etag, o.mediaKey)
	}

	// Now make the actual GET request to the resolved URL with retry logic
	var res *http.Response
	var getErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			sleepTime := time.Duration(1<<uint(attempt-1)) * time.Second
			fs.Debugf(nil, "mediavfs: retrying GET request after %v (attempt %d/3)", sleepTime, attempt+1)
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

	fs.Debugf(nil, "mediavfs: opened %s with status %d", o.mediaKey, res.StatusCode)

	return res.Body, nil
}

// Update is not supported
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errNotWritable
}

// Remove is allowed for move operations but doesn't delete from database
// (VFS layer may do copy+delete for renames)
func (o *Object) Remove(ctx context.Context) error {
	fs.Debugf(o.fs, "Remove called on %s - allowed for move operations, file remains in database", o.Remote())
	// Don't actually delete from database - just allow the operation
	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
	_ fs.Mover  = (*Fs)(nil)
)
