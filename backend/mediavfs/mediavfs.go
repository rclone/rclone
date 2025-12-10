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
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         false,
	}).Fill(ctx, f)

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

	// If root is empty, list all users
	if root == "" {
		return f.listUsers(ctx)
	}

	userName, subPath := splitUserPath(root)

	// If only username is specified, list top-level items for that user
	if subPath == "" {
		return f.listUserFiles(ctx, userName, "")
	}

	// List files in a specific directory
	return f.listUserFiles(ctx, userName, subPath)
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
	// Query to get all files for the user (no LIMIT - load all files)
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
				// This is in a subdirectory
				subDir := strings.SplitN(fullPath, "/", 2)[0]
				if !dirsSeen[subDir] {
					entries = append(entries, fs.NewDir(userName+"/"+subDir, time.Time{}))
					dirsSeen[subDir] = true
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

	fs.Debugf(nil, "mediavfs: created virtual directory: %s", fullPath)
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

	fs.Debugf(nil, "mediavfs: moved %s to %s (real-time update)", srcObj.remote, remote)

	return newObj, nil
}

// DirMove is not supported
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	return fs.ErrorCantDirMove
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

		fs.Debugf(nil, "mediavfs: cached URL and ETag=%s for %s", etag, o.mediaKey)
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

	fs.Debugf(nil, "mediavfs: opened %s with status %d", o.mediaKey, res.StatusCode)

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
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
	_ fs.Mover  = (*Fs)(nil)
)
