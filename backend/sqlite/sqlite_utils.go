package sqlite

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
)

// An Fs is a representation of a remote SQLite Fs
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	db       *sql.DB
}

// An File is a representation of an actual file row in the database
type File struct {
	Filename string
	Type     string
	Content  string
	Size     int64
	ModTime  int64
	SHA1     string
}

// An Object on the remote SQLite Fs
type Object struct {
	fs      *Fs       // what this object is part of
	info    File      // info about the object
	remote  string    // The remote path
	size    int64     // size of the object
	modTime time.Time // modification time of the object
	sha1    string    // SHA-1 of the object content
}

// Options represent the configuration of the SQLite backend
type Options struct {
	SqliteFile string
}

// Schema for the files table
const schema = `
CREATE TABLE IF NOT EXISTS files (
    filename TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    content BLOB NOT NULL DEFAULT '',  -- Using BLOB for large file contents
    size INTEGER NOT NULL DEFAULT 0,
    mod_time INTEGER NOT NULL, -- Store as Unix timestamp
    sha1 TEXT NOT NULL DEFAULT '',
    CONSTRAINT type_check CHECK (type IN ('file', 'dir'))
);

-- Index to improve directory listing performance
CREATE INDEX IF NOT EXISTS idx_filename_prefix ON files(filename);
`

func getConnection(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return db, nil
}

// initSqlite initializes the SQLite database with the required schema
func initSqlite(db *sql.DB) error {
	// Check if table exists first
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='files'").Scan(&tableName)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	// Only create schema if table doesn't exist
	if err == sql.ErrNoRows {
		_, err = db.Exec(schema)
		if err != nil {
			return fmt.Errorf("failed to initialize database schema: %w", err)
		}
	}
	return nil
}

func (f *Fs) findFile(fullPath string) (*File, error) {
	fs.Debugf(nil, "[findFile] fullPath: %q", fullPath)
	var err error
	if f.db == nil {
		f.db, err = getConnection(f.opt.SqliteFile)
		if err != nil {
			return nil, err
		}
	}
	row := f.db.QueryRow("SELECT filename, type, content, size, mod_time, sha1 FROM files WHERE filename = ?", fullPath)
	err = row.Err()
	if row == nil || err == sql.ErrNoRows {
		return nil, nil
	}

	file := &File{}
	err = row.Scan(&file.Filename, &file.Type, &file.Content, &file.Size, &file.ModTime, &file.SHA1)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (f *Fs) fileExists(fullPath string) bool {
	file, err := f.findFile(fullPath)
	if err != nil {
		return false
	}
	return file != nil
}

func (f *Fs) getFiles(fullPath string) (*[]File, error) {
	var err error
	if f.db == nil {
		f.db, err = getConnection(f.opt.SqliteFile)
		if err != nil {
			return nil, err
		}
	}
	rows, err := f.db.Query("SELECT filename, type, content, size, mod_time, sha1 FROM files WHERE filename LIKE ?", fullPath+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}

	var files []File
	fileOrDirExists := false
	for rows.Next() {
		var file File
		err := rows.Scan(&file.Filename, &file.Type, &file.Content, &file.Size, &file.ModTime, &file.SHA1)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		dir := path.Dir(file.Filename) // Extract directory path and filename
		if dir == fullPath {
			files = append(files, file)
			fileOrDirExists = true
		} else if file.Filename == fullPath {
			fileOrDirExists = true
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	err = rows.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close rows: %w", err)
	}
	if !fileOrDirExists {
		return nil, fs.ErrorDirNotFound
	}

	return &files, nil
}

func (f *Fs) hasFiles(fullPath string) (bool, error) {
	files, err := f.getFiles(fullPath)
	if err != nil {
		return false, err
	}
	return len(*files) > 0, nil
}

func (f *Fs) mkDir(fullPath string) error {
	parts := strings.Split(fullPath, "/")

	for i, part := range parts {
		if part == "" {
			continue
		}
		dir := strings.Join(parts[:i+1], "/")
		if f.fileExists(dir) {
			continue
		}
		fs.Debugf(nil, "[mkdirTree] creating directory: %q part: %q", dir, part)

		_, err := f.db.Exec("INSERT OR REPLACE INTO files (filename, type, mod_time) VALUES (?, ?, ?)", dir, "dir", time.Now().UnixMilli())
		if err != nil {
			return fmt.Errorf("failed to insert directory: %w", err)
		}
	}
	return nil
}

func (f *Fs) rmDir(fullPath string) error {
	fs.Debugf(nil, "[rmdir] fullPath: %q", fullPath)
	var err error
	// Check if directory is empty
	result, err := f.hasFiles(fullPath)
	if err != nil {
		return err
	}
	if result {
		return fs.ErrorDirectoryNotEmpty
	}

	// Check if directory exists
	file, err := f.findFile(fullPath)
	if err != nil {
		return err
	}
	if file == nil {
		return fs.ErrorDirNotFound
	}

	_, err = f.db.Exec("DELETE FROM files WHERE filename = ?", fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete directory: %w", err)
	}
	return nil
}

func (f *Fs) putFile(in io.Reader, fullPath string, modTime time.Time) (*File, error) {
	fs.Debugf(nil, "[putFile] fullPath: %q", fullPath)

	content, err := func() (string, error) {
		data, err := io.ReadAll(in)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}()
	if err != nil {
		return nil, err
	}

	file := &File{
		Filename: fullPath,
		Type:     "file",
		ModTime:  modTime.UnixMilli(),
		Content:  content,
	}

	file.calculateMetadata()

	_, err = f.db.Exec("INSERT OR REPLACE INTO files (filename, type, content, size, mod_time, sha1) VALUES (?, ?, ?, ?, ?, ?)", file.Filename, file.Type, file.Content, file.Size, file.ModTime, file.SHA1)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (f *Fs) remove(fullPath string) error {
	fs.Debugf(nil, "[remove] fullPath: %q", fullPath)
	_, err := f.db.Exec("DELETE FROM files WHERE filename = ?", fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Calculate size and SHA1 hash for a file
func (f *File) calculateMetadata() {
	// Calculate size from content in bytes
	f.Size = int64(len(f.Content))
	// f.Size = int64(len([]byte(f.Content)))

	// Create a new SHA-1 hash object
	hasher := sha1.New()
	// Write the input string to the hasher
	hasher.Write([]byte(f.Content))
	f.SHA1 = hex.EncodeToString(hasher.Sum(nil))
}

// fullPath constructs a full, absolute path from an Fs root relative path,
func (f *Fs) fullPath(part string) string {
	return path.Join(f.root, part)
}
