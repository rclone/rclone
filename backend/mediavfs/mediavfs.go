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
)

var (
	errNotWritable = errors.New("mediavfs is read-only except for move/rename operations")
	errCrossUser   = errors.New("cannot move files between different users")
)

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
	name       string
	root       string
	opt        Options
	features   *fs.Features
	db         *sql.DB
	httpClient *http.Client
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

	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		db:         db,
		httpClient: fshttp.NewClient(ctx),
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
	// Query to get all files for the user
	query := fmt.Sprintf(`
		SELECT
			media_key,
			file_name,
			COALESCE(name, file_name) as display_name,
			COALESCE(path, '') as display_path,
			size_bytes,
			utc_timestamp
		FROM %s
		WHERE user_name = $1
		ORDER BY display_path, display_name
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
			mediaKey    string
			fileName    string
			displayName string
			displayPath string
			sizeBytes   int64
			timestamp   time.Time
		)

		if err := rows.Scan(&mediaKey, &fileName, &displayName, &displayPath, &sizeBytes, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		// Construct the full path
		var fullPath string
		displayPath = strings.Trim(displayPath, "/")
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
			COALESCE(name, file_name) as display_name,
			COALESCE(path, '') as display_path,
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
			mediaKey    string
			fileName    string
			displayName string
			displayPath string
			sizeBytes   int64
			timestamp   time.Time
		)

		if err := rows.Scan(&mediaKey, &fileName, &displayName, &displayPath, &sizeBytes, &timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		// Construct the full path
		var fullPath string
		displayPath = strings.Trim(displayPath, "/")
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

// Mkdir is not supported (directories are virtual)
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
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

	// Return new object
	return f.NewObject(ctx, remote)
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

// Open opens the file for reading with ETag support and intelligent range handling
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Build the download URL
	url := fmt.Sprintf("%s/%s", o.fs.opt.DownloadURL, o.mediaKey)

	// Check if we need seeking capability
	needsSeek := false
	for _, opt := range options {
		if _, ok := opt.(*fs.SeekOption); ok {
			needsSeek = true
			break
		}
	}

	// Use seekable reader if seeking is needed, otherwise use optimized streaming reader
	if needsSeek {
		return newSeekableHTTPReader(ctx, url, o.fs.httpClient, o.size, options)
	}

	// For simple streaming without seeking, use the optimized reader
	// If there's a range option, the intelligent reader will handle it
	if hasRangeOption(options) {
		return newHTTPReader(ctx, url, o.fs.httpClient, o.size, options)
	}

	// For simple full-file reads, use the optimized reader
	return newOptimizedHTTPReader(ctx, url, o.fs.httpClient, options)
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
