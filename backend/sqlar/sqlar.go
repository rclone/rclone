package sqlar

import (
	"bytes"
	"compress/zlib"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
	"golang.org/x/sync/semaphore"
)

const (
	// Unix file mode bits
	modeDir  = 0o40755
	modeFile = 0o100644

	// memBudget is the maximum bytes held in memory across all concurrent
	// Put/Update operations awaiting compression. 256 MiB lets hundreds of
	// small files compress in parallel while capping peak usage for large files.
	memBudget = int64(256 * 1024 * 1024)
)

var commandHelp = []fs.CommandHelp{{
	Name:  "vacuum",
	Short: "Compact the archive, reclaiming space from deleted files.",
	Long: `After deleting files, the archive file doesn't shrink automatically.
Run vacuum to rebuild the database and reclaim unused space.

    rclone backend vacuum remote:
`,
}}

// metadataInfo describes the metadata keys supported by the sqlar backend.
var metadataInfo = &fs.MetadataInfo{
	System: map[string]fs.MetadataHelp{
		"mtime": {
			Help:    "Time of last modification",
			Type:    "RFC 3339",
			Example: "2006-01-02T15:04:05Z",
		},
		"mode": {
			Help:    "File permissions (octal Unix mode)",
			Type:    "octal",
			Example: "0644",
		},
	},
	Help: `Sqlar stores file modification time and Unix permissions as metadata.`,
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:         "sqlar",
		Description:  "SQLite Archive",
		NewFs:        NewFs,
		CommandHelp:  commandHelp,
		MetadataInfo: metadataInfo,
		Options: []fs.Option{
			{
				Name:     "path",
				Help:     "Path to the SQLite archive file.",
				Required: true,
			},
			{
				Name:     "compression_level",
				Help:     "Deflate compression level (-1 to 9). -1 = zlib default, 0 = none, 9 = best compression.",
				Default:  -1,
				Advanced: true,
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Path             string `config:"path"`
	CompressionLevel int    `config:"compression_level"`
}

// Fs represents a SQLite Archive filesystem
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	db       *sql.DB
	mu       sync.Mutex          // serialize SQLite writes (single-writer constraint)
	sem      *semaphore.Weighted // bounds concurrent in-memory compression buffers
}

// Object represents a file in the archive
type Object struct {
	fs      *Fs
	remote  string
	name    string // full path in sqlar table
	mode    int
	modTime time.Time
	size    int64                // uncompressed size
	mu      sync.Mutex           // protects hashes
	hashes  map[hash.Type]string // lazily computed on first Hash() call
}

// Directory represents a directory in the archive
type Directory struct {
	fs      *Fs
	remote  string
	name    string // full path in sqlar table
	modTime time.Time
}

// Fs returns read only access to the Fs that this object is part of
func (d *Directory) Fs() fs.Info { return d.fs }

// String returns the name
func (d *Directory) String() string { return d.remote }

// Remote returns the remote path
func (d *Directory) Remote() string { return d.remote }

// ModTime returns the modification date of the directory
func (d *Directory) ModTime(ctx context.Context) time.Time { return d.modTime }

// Size returns the size of the directory (always 0)
func (d *Directory) Size() int64 { return 0 }

// Items returns the count of items in this directory (always -1 for unknown)
func (d *Directory) Items() int64 { return -1 }

// ID returns the internal ID of this directory (empty for sqlar)
func (d *Directory) ID() string { return "" }

// SetModTime sets the modification time of the directory
func (d *Directory) SetModTime(ctx context.Context, t time.Time) error {
	d.fs.mu.Lock()
	defer d.fs.mu.Unlock()
	_, err := d.fs.db.ExecContext(ctx,
		"UPDATE sqlar SET mtime = ? WHERE name = ? AND mode & 0x4000 != 0",
		t.Unix(), d.name)
	if err != nil {
		return fmt.Errorf("sqlar Directory.SetModTime: %w", err)
	}
	d.modTime = t.Truncate(time.Second)
	return nil
}

// fullPath returns the full path in the sqlar table for a remote path.
func (f *Fs) fullPath(remote string) string {
	if f.root == "" {
		return remote
	}
	if remote == "" {
		return f.root
	}
	return f.root + "/" + remote
}

// remotePath returns the remote path relative to f.root for a sqlar name.
func (f *Fs) remotePath(name string) string {
	if f.root == "" {
		return name
	}
	return strings.TrimPrefix(name, f.root+"/")
}

// dirGlob returns GLOB patterns for listing direct children of dir.
// Direct children = match \ exclude.
func dirGlob(dir string) (match, exclude string) {
	if dir == "" {
		return "*", "*/*"
	}
	return dir + "/*", dir + "/*/*"
}

// treeGlob returns a GLOB pattern for all descendants of dir.
func treeGlob(dir string) string {
	if dir == "" {
		return "*"
	}
	return dir + "/*"
}

// modeFromSrc returns the file mode to use when writing an object.
// If src carries a "mode" metadata key the permission bits are taken from it;
// otherwise base (e.g. modeFile) is returned unchanged.
func modeFromSrc(ctx context.Context, src fs.ObjectInfo, base int) int {
	meta, err := fs.GetMetadata(ctx, src)
	if err != nil || meta == nil {
		return base
	}
	modeStr, ok := meta["mode"]
	if !ok {
		return base
	}
	m, err := strconv.ParseInt(modeStr, 8, 64)
	if err != nil {
		fs.Debugf(src, "sqlar: ignoring invalid mode %q in metadata: %v", modeStr, err)
		return base
	}
	return (base &^ 0o7777) | (int(m) & 0o7777)
}

// mtimeFromSrc returns the modification time to use when writing an object.
// If src carries an "mtime" metadata key (RFC 3339) it takes precedence over
// src.ModTime(ctx); otherwise the ObjectInfo ModTime is returned.
func mtimeFromSrc(ctx context.Context, src fs.ObjectInfo) time.Time {
	meta, err := fs.GetMetadata(ctx, src)
	if err != nil || meta == nil {
		return src.ModTime(ctx)
	}
	mtimeStr, ok := meta["mtime"]
	if !ok {
		return src.ModTime(ctx)
	}
	t, err := time.Parse(time.RFC3339, mtimeStr)
	if err != nil {
		fs.Debugf(src, "sqlar: ignoring invalid mtime %q in metadata: %v", mtimeStr, err)
		return src.ModTime(ctx)
	}
	return t
}

// ------------------------------------------------------------------
// Fs interface
// ------------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("sqlar root '%s' at '%s'", f.root, f.opt.Path)
}

// Precision of timestamps
func (f *Fs) Precision() time.Duration { return time.Second }

// Hashes returns the supported hash sets
func (f *Fs) Hashes() hash.Set { return hash.NewHashSet(hash.MD5, hash.SHA1) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	db, err := openDB(opt.Path)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	root = strings.Trim(root, "/")
	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		db:   db,
		sem:  semaphore.NewWeighted(memBudget),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		WriteDirSetModTime:      true,
		NoMultiThreading:        true,
		ReadMetadata:            true,
		WriteMetadata:           true,
	}).Fill(ctx, f)

	if root != "" {
		var mode int
		err := db.QueryRowContext(ctx,
			"SELECT mode FROM sqlar WHERE name = ?", root).Scan(&mode)
		if err == nil && mode&0o40000 == 0 {
			// Root is a file, adjust root to parent
			f.root = path.Dir(root)
			if f.root == "." {
				f.root = ""
			}
			return f, fs.ErrorIsFile
		}
	}
	return f, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	name := f.fullPath(remote)
	var mode int
	var mtime, sz int64
	err := f.db.QueryRowContext(ctx,
		"SELECT mode, mtime, sz FROM sqlar WHERE name = ?", name).Scan(&mode, &mtime, &sz)
	if err == sql.ErrNoRows {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlar NewObject: %w", err)
	}
	if mode&0o40000 != 0 {
		return nil, fs.ErrorIsDir
	}
	return &Object{
		fs:      f,
		remote:  remote,
		name:    name,
		mode:    mode,
		modTime: time.Unix(mtime, 0),
		size:    sz,
	}, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	return list.WithListP(ctx, dir, f)
}

// ListP lists the objects and directories of the Fs starting from dir
// non recursively into out.
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	fullDir := f.fullPath(dir)

	// Check directory exists (for non-root dirs, check for explicit entry;
	// for any dir, check for children)
	if fullDir != "" {
		var exists int
		err := f.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlar WHERE name = ? OR name GLOB ?",
			fullDir, fullDir+"/*").Scan(&exists)
		if err != nil {
			return fmt.Errorf("sqlar List check: %w", err)
		}
		if exists == 0 {
			return fs.ErrorDirNotFound
		}
	}

	// match/*  selects all descendants; NOT match/*/* excludes deeper entries,
	// leaving only direct children.
	match, exclude := dirGlob(fullDir)
	rows, err := f.db.QueryContext(ctx,
		"SELECT name, mode, mtime, sz FROM sqlar WHERE name GLOB ? AND name NOT GLOB ?",
		match, exclude)
	if err != nil {
		return fmt.Errorf("sqlar List: %w", err)
	}
	defer func() { _ = rows.Close() }()
	helper := list.NewHelper(callback)
	for rows.Next() {
		var entryName string
		var mode int
		var mtime, sz int64
		if err := rows.Scan(&entryName, &mode, &mtime, &sz); err != nil {
			return fmt.Errorf("sqlar List scan: %w", err)
		}
		remote := f.remotePath(entryName)
		if mode&0o40000 != 0 {
			d := &Directory{
				fs:      f,
				remote:  remote,
				name:    entryName,
				modTime: time.Unix(mtime, 0),
			}
			if err := helper.Add(d); err != nil {
				return err
			}
		} else {
			obj := &Object{
				fs:      f,
				remote:  remote,
				name:    entryName,
				mode:    mode,
				modTime: time.Unix(mtime, 0),
				size:    sz,
			}
			if err := helper.Add(obj); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlar List rows: %w", err)
	}
	return helper.Flush()
}

// ListR lists the objects and directories recursively
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	fullDir := f.fullPath(dir)

	// Check directory exists for non-root paths.
	if fullDir != "" {
		var exists int
		err := f.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlar WHERE name = ? OR name GLOB ?",
			fullDir, fullDir+"/*").Scan(&exists)
		if err != nil {
			return fmt.Errorf("sqlar ListR check: %w", err)
		}
		if exists == 0 {
			return fs.ErrorDirNotFound
		}
	}

	pattern := treeGlob(fullDir)
	rows, err := f.db.QueryContext(ctx,
		"SELECT name, mode, mtime, sz FROM sqlar WHERE name GLOB ?", pattern)
	if err != nil {
		return fmt.Errorf("sqlar ListR: %w", err)
	}
	defer func() { _ = rows.Close() }()
	helper := list.NewHelper(callback)
	for rows.Next() {
		var entryName string
		var mode int
		var mtime, sz int64
		if err := rows.Scan(&entryName, &mode, &mtime, &sz); err != nil {
			return fmt.Errorf("sqlar ListR scan: %w", err)
		}
		remote := f.remotePath(entryName)
		if mode&0o40000 != 0 {
			d := &Directory{
				fs:      f,
				remote:  remote,
				name:    entryName,
				modTime: time.Unix(mtime, 0),
			}
			if err := helper.Add(d); err != nil {
				return err
			}
		} else {
			obj := &Object{
				fs:      f,
				remote:  remote,
				name:    entryName,
				mode:    mode,
				modTime: time.Unix(mtime, 0),
				size:    sz,
			}
			if err := helper.Add(obj); err != nil {
				return err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlar ListR rows: %w", err)
	}
	return helper.Flush()
}

// mkdirAll creates directory entries for all ancestors of name.
func (f *Fs) mkdirAll(ctx context.Context, dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	// Walk up to root, collecting dirs to create
	var dirs []string
	for d := dir; d != "" && d != "."; d = path.Dir(d) {
		if d == "." {
			break
		}
		dirs = append(dirs, d)
	}
	// Create from topmost down (ignore conflicts with existing entries)
	for i := len(dirs) - 1; i >= 0; i-- {
		_, err := f.db.ExecContext(ctx,
			"INSERT OR IGNORE INTO sqlar (name, mode, mtime, sz, data) VALUES (?, ?, ?, 0, NULL)",
			dirs[i], modeDir, time.Now().Unix())
		if err != nil {
			return fmt.Errorf("sqlar mkdir %q: %w", dirs[i], err)
		}
	}
	return nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	name := f.fullPath(remote)
	modTime := mtimeFromSrc(ctx, src)
	mode := modeFromSrc(ctx, src, modeFile)
	sz := src.Size()

	var blobReader io.Reader
	var blobSize int64
	if f.opt.CompressionLevel == 0 && sz >= 0 {
		// Fully streaming: no in-memory buffer, skip semaphore.
		blobReader = in
		blobSize = sz
	} else {
		// Acquire semaphore to bound peak memory across concurrent transfers.
		// Unknown size (-1) acquires the full budget to be conservative.
		weight := sz
		if weight <= 0 || weight > memBudget {
			weight = memBudget
		}
		if err := f.sem.Acquire(ctx, weight); err != nil {
			return nil, fmt.Errorf("sqlar Put acquire: %w", err)
		}
		defer f.sem.Release(weight)

		data, err := io.ReadAll(in)
		if err != nil {
			return nil, fmt.Errorf("sqlar Put read: %w", err)
		}
		if sz < 0 {
			sz = int64(len(data))
		}
		compressed, err := deflateCompress(data, f.opt.CompressionLevel)
		if err != nil {
			return nil, err
		}
		blobSize = int64(len(compressed))
		blobReader = bytes.NewReader(compressed)
	}

	// Hold the write lock only for the SQLite write (single-writer constraint).
	f.mu.Lock()
	defer f.mu.Unlock()

	if parent := path.Dir(name); parent != "" && parent != "." {
		if err := f.mkdirAll(ctx, parent); err != nil {
			return nil, err
		}
	}

	if _, err := insertBlobStreaming(ctx, f.db, name, mode, modTime.Unix(), sz, blobSize, blobReader); err != nil {
		if errors.Is(err, errBlobTooLarge) {
			return nil, fserrors.NoRetryError(fmt.Errorf("sqlar Put: %w", err))
		}
		return nil, fmt.Errorf("sqlar Put: %w", err)
	}

	return &Object{
		fs:      f,
		remote:  remote,
		name:    name,
		mode:    mode,
		modTime: modTime.Truncate(time.Second),
		size:    sz,
	}, nil
}

// PutStream uploads with unknown size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	name := f.fullPath(dir)
	if name == "" {
		return nil // root always exists
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mkdirAll(ctx, name)
}

// Rmdir removes an empty directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	name := f.fullPath(dir)
	if name == "" {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	var mode int
	err := f.db.QueryRowContext(ctx,
		"SELECT mode FROM sqlar WHERE name = ?", name).Scan(&mode)
	if err == sql.ErrNoRows {
		return fs.ErrorDirNotFound
	}
	if err != nil {
		return fmt.Errorf("sqlar Rmdir mode: %w", err)
	}
	if mode&0o40000 == 0 {
		return fs.ErrorDirNotFound
	}

	// Check for children
	pattern := name + "/*"
	var count int
	err = f.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlar WHERE name GLOB ?", pattern).Scan(&count)
	if err != nil {
		return fmt.Errorf("sqlar Rmdir count: %w", err)
	}
	if count > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	result, err := f.db.ExecContext(ctx,
		"DELETE FROM sqlar WHERE name = ? AND mode & 0x4000 != 0", name)
	if err != nil {
		return fmt.Errorf("sqlar Rmdir: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fs.ErrorDirNotFound
	}
	return nil
}

// ------------------------------------------------------------------
// Optional Fs features
// ------------------------------------------------------------------

// applyMetadataSet returns mode and modTime, overriding with ci.MetadataSet
// values when present. base is the starting mode.
func applyMetadataSet(ctx context.Context, base int, baseTime time.Time) (int, time.Time) {
	ci := fs.GetConfig(ctx)
	if ci.MetadataSet == nil {
		return base, baseTime
	}
	mode, modTime := base, baseTime
	if modeStr, ok := ci.MetadataSet["mode"]; ok {
		if m, err := strconv.ParseInt(modeStr, 8, 64); err == nil {
			mode = (base &^ 0o7777) | (int(m) & 0o7777)
		}
	}
	if mtimeStr, ok := ci.MetadataSet["mtime"]; ok {
		if t, err := time.Parse(time.RFC3339, mtimeStr); err == nil {
			modTime = t
		}
	}
	return mode, modTime
}

// Copy src to this remote using server-side copy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	dstName := f.fullPath(remote)

	mode, modTime := applyMetadataSet(ctx, srcObj.mode, srcObj.modTime)

	f.mu.Lock()
	defer f.mu.Unlock()

	if parent := path.Dir(dstName); parent != "" && parent != "." {
		if err := f.mkdirAll(ctx, parent); err != nil {
			return nil, err
		}
	}

	_, err := f.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO sqlar (name, mode, mtime, sz, data)
		 SELECT ?, ?, ?, sz, data FROM sqlar WHERE name = ?`,
		dstName, mode, modTime.Unix(), srcObj.name)
	if err != nil {
		return nil, fmt.Errorf("sqlar Copy: %w", err)
	}
	return f.NewObject(ctx, remote)
}

// Move src to this remote using server-side move
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	dstName := f.fullPath(remote)

	mode, modTime := applyMetadataSet(ctx, srcObj.mode, srcObj.modTime)

	f.mu.Lock()
	defer f.mu.Unlock()

	if parent := path.Dir(dstName); parent != "" && parent != "." {
		if err := f.mkdirAll(ctx, parent); err != nil {
			return nil, err
		}
	}

	_, err := f.db.ExecContext(ctx,
		"UPDATE sqlar SET name = ?, mode = ?, mtime = ? WHERE name = ?",
		dstName, mode, modTime.Unix(), srcObj.name)
	if err != nil {
		return nil, fmt.Errorf("sqlar Move: %w", err)
	}
	return f.NewObject(ctx, remote)
}

// DirMove moves src, srcRemote to this remote at dstRemote
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}
	if srcFs.opt.Path != f.opt.Path {
		return fs.ErrorCantDirMove
	}

	srcPrefix := srcFs.fullPath(srcRemote)
	dstPrefix := f.fullPath(dstRemote)

	// Check destination doesn't exist
	var count int
	err := f.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlar WHERE name = ? OR name GLOB ?",
		dstPrefix, dstPrefix+"/*").Scan(&count)
	if err != nil {
		return fmt.Errorf("sqlar DirMove check: %w", err)
	}
	if count > 0 {
		return fs.ErrorDirExists
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if parent := path.Dir(dstPrefix); parent != "" && parent != "." {
		if err := f.mkdirAll(ctx, parent); err != nil {
			return err
		}
	}

	// Rename the directory entry itself and all descendants.
	// Use SQLite's length() (character count) not Go's len() (byte count)
	// so that multi-byte Unicode paths are handled correctly.
	_, err = f.db.ExecContext(ctx,
		`UPDATE sqlar SET name = ? || substr(name, length(?) + 1)
		 WHERE name = ? OR name GLOB ?`,
		dstPrefix, srcPrefix, srcPrefix, srcPrefix+"/*")
	if err != nil {
		return fmt.Errorf("sqlar DirMove: %w", err)
	}
	return nil
}

// Purge deletes all files in the directory
func (f *Fs) Purge(ctx context.Context, dir string) error {
	prefix := f.fullPath(dir)
	f.mu.Lock()
	defer f.mu.Unlock()
	var result sql.Result
	var err error
	if prefix == "" {
		result, err = f.db.ExecContext(ctx, "DELETE FROM sqlar")
	} else {
		result, err = f.db.ExecContext(ctx,
			"DELETE FROM sqlar WHERE name = ? OR name GLOB ?",
			prefix, prefix+"/*")
	}
	if err != nil {
		return fmt.Errorf("sqlar Purge: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fs.ErrorDirNotFound
	}
	return nil
}

// DirSetModTime sets the modification time on a directory
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	name := f.fullPath(dir)
	if name == "" {
		return nil // root has no entry
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	result, err := f.db.ExecContext(ctx,
		"UPDATE sqlar SET mtime = ? WHERE name = ? AND mode & 0x4000 != 0",
		modTime.Unix(), name)
	if err != nil {
		return fmt.Errorf("sqlar DirSetModTime: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fs.ErrorDirNotFound
	}
	return nil
}

// About returns info about the archive
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	fi, err := os.Stat(f.opt.Path)
	if err != nil {
		return nil, fmt.Errorf("sqlar About stat: %w", err)
	}
	used := fi.Size()
	var count int64
	err = f.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlar WHERE mode & 0x4000 = 0").Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("sqlar About count: %w", err)
	}
	return &fs.Usage{
		Used:    &used,
		Objects: &count,
	}, nil
}

// Shutdown closes the database connection cleanly, checkpointing the WAL.
func (f *Fs) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	db := f.db
	f.db = nil
	f.mu.Unlock()
	if db == nil {
		return nil
	}
	return closeDB(db)
}

// Command runs backend-specific commands
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (any, error) {
	switch name {
	case "vacuum":
		f.mu.Lock()
		defer f.mu.Unlock()
		// Checkpoint WAL, vacuum, then checkpoint again to consolidate
		// everything into a single clean file.
		if _, err := f.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			return nil, fmt.Errorf("sqlar vacuum pre-checkpoint: %w", err)
		}
		if _, err := f.db.ExecContext(ctx, "VACUUM"); err != nil {
			return nil, fmt.Errorf("sqlar vacuum: %w", err)
		}
		if _, err := f.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			return nil, fmt.Errorf("sqlar vacuum post-checkpoint: %w", err)
		}
		return nil, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// ------------------------------------------------------------------
// Object interface
// ------------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info { return o.fs }

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// String returns a string representation
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }

// Size returns the uncompressed size
func (o *Object) Size() int64 { return o.size }

// Hash returns the hash of the object, computing and caching it on first call.
// All supported hashes are computed in a single pass to avoid redundant DB reads.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty == hash.None {
		return "", nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.hashes != nil {
		val, ok := o.hashes[ty]
		if !ok {
			return "", hash.ErrUnsupported
		}
		return val, nil
	}
	r, err := o.Open(ctx)
	if err != nil {
		return "", err
	}
	hashes, err := hash.StreamTypes(r, o.fs.Hashes())
	_ = r.Close()
	if err != nil {
		return "", fmt.Errorf("sqlar Hash: %w", err)
	}
	o.hashes = hashes
	val, ok := hashes[ty]
	if !ok {
		return "", hash.ErrUnsupported
	}
	return val, nil
}

// Storable returns true
func (o *Object) Storable() bool { return true }

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	o.fs.mu.Lock()
	defer o.fs.mu.Unlock()
	_, err := o.fs.db.ExecContext(ctx,
		"UPDATE sqlar SET mtime = ? WHERE name = ?",
		modTime.Unix(), o.name)
	if err != nil {
		return fmt.Errorf("sqlar SetModTime: %w", err)
	}
	o.modTime = modTime.Truncate(time.Second)
	return nil
}

// compressedReadCloser wraps a zlib reader over a blob handle, closing both on Close.
type compressedReadCloser struct {
	zlib io.ReadCloser
	blob io.ReadSeekCloser
}

func (c *compressedReadCloser) Read(p []byte) (int, error) { return c.zlib.Read(p) }
func (c *compressedReadCloser) Close() error {
	zlibErr := c.zlib.Close()
	blobErr := c.blob.Close()
	if zlibErr != nil {
		return zlibErr
	}
	return blobErr
}

// limitedRC wraps an io.ReadCloser with a byte count limit.
type limitedRC struct {
	r  io.ReadCloser
	lr io.LimitedReader
}

func (l *limitedRC) Read(p []byte) (int, error) { return l.lr.Read(p) }
func (l *limitedRC) Close() error               { return l.r.Close() }

// Open opens the Object for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	sz, blobLen, blobRC, err := openBlobReader(ctx, o.fs.db, o.name)
	if err == sql.ErrNoRows {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlar Open: %w", err)
	}

	compressed := sz != blobLen

	var r io.ReadCloser
	if compressed {
		zlibR, zlibErr := zlib.NewReader(blobRC)
		if zlibErr != nil {
			_ = blobRC.Close()
			return nil, fmt.Errorf("sqlar Open zlib: %w", zlibErr)
		}
		r = &compressedReadCloser{zlib: zlibR, blob: blobRC}
	} else {
		r = blobRC
	}

	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, limit = x.Decode(sz)
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	if offset > 0 {
		if !compressed {
			// Uncompressed: seek directly within the blob — O(1), no data copied.
			if _, seekErr := blobRC.Seek(offset, io.SeekStart); seekErr != nil {
				_ = r.Close()
				return nil, fmt.Errorf("sqlar Open seek: %w", seekErr)
			}
		} else {
			// Compressed: must decompress and discard up to offset.
			if _, skipErr := io.CopyN(io.Discard, r, offset); skipErr != nil && skipErr != io.EOF {
				_ = r.Close()
				return nil, fmt.Errorf("sqlar Open skip: %w", skipErr)
			}
		}
	}

	if limit >= 0 {
		return &limitedRC{r: r, lr: io.LimitedReader{R: r, N: limit}}, nil
	}
	return r, nil
}

// Update replaces the object content
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	modTime := mtimeFromSrc(ctx, src)
	mode := modeFromSrc(ctx, src, o.mode)
	sz := src.Size()

	var blobReader io.Reader
	var blobSize int64
	if o.fs.opt.CompressionLevel == 0 && sz >= 0 {
		blobReader = in
		blobSize = sz
	} else {
		weight := sz
		if weight <= 0 || weight > memBudget {
			weight = memBudget
		}
		if err := o.fs.sem.Acquire(ctx, weight); err != nil {
			return fmt.Errorf("sqlar Update acquire: %w", err)
		}
		defer o.fs.sem.Release(weight)

		data, err := io.ReadAll(in)
		if err != nil {
			return fmt.Errorf("sqlar Update read: %w", err)
		}
		if sz < 0 {
			sz = int64(len(data))
		}
		compressed, err := deflateCompress(data, o.fs.opt.CompressionLevel)
		if err != nil {
			return err
		}
		blobSize = int64(len(compressed))
		blobReader = bytes.NewReader(compressed)
	}

	o.fs.mu.Lock()
	defer o.fs.mu.Unlock()

	if _, err := insertBlobStreaming(ctx, o.fs.db, o.name, mode, modTime.Unix(), sz, blobSize, blobReader); err != nil {
		if errors.Is(err, errBlobTooLarge) {
			return fserrors.NoRetryError(fmt.Errorf("sqlar Update: %w", err))
		}
		return fmt.Errorf("sqlar Update: %w", err)
	}
	o.size = sz
	o.mode = mode
	o.modTime = modTime.Truncate(time.Second)
	o.hashes = nil
	return nil
}

// Metadata returns metadata for the object.
// Keys returned: "mtime" (RFC 3339) and "mode" (octal Unix file mode).
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	return fs.Metadata{
		"mtime": o.modTime.Format(time.RFC3339),
		"mode":  fmt.Sprintf("%0o", o.mode),
	}, nil
}

// SetMetadata sets metadata on the object.
// Supported keys: "mtime" (RFC 3339 time) and "mode" (octal Unix permissions).
// File-type bits in mode are preserved; only the lower 12 permission bits are updated.
func (o *Object) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	mode := o.mode
	modTime := o.modTime

	if modeStr, ok := metadata["mode"]; ok {
		m, err := strconv.ParseInt(modeStr, 8, 64)
		if err != nil {
			return fmt.Errorf("sqlar SetMetadata: invalid mode %q: %w", modeStr, err)
		}
		mode = (o.mode &^ 0o7777) | (int(m) & 0o7777)
	}
	if mtimeStr, ok := metadata["mtime"]; ok {
		t, err := time.Parse(time.RFC3339, mtimeStr)
		if err != nil {
			return fmt.Errorf("sqlar SetMetadata: invalid mtime %q: %w", mtimeStr, err)
		}
		modTime = t
	}

	o.fs.mu.Lock()
	defer o.fs.mu.Unlock()
	_, err := o.fs.db.ExecContext(ctx,
		"UPDATE sqlar SET mode = ?, mtime = ? WHERE name = ?",
		mode, modTime.Unix(), o.name)
	if err != nil {
		return fmt.Errorf("sqlar SetMetadata: %w", err)
	}
	o.mode = mode
	o.modTime = modTime.Truncate(time.Second)
	return nil
}

// Remove deletes the object
func (o *Object) Remove(ctx context.Context) error {
	o.fs.mu.Lock()
	defer o.fs.mu.Unlock()
	_, err := o.fs.db.ExecContext(ctx,
		"DELETE FROM sqlar WHERE name = ?", o.name)
	if err != nil {
		return fmt.Errorf("sqlar Remove: %w", err)
	}
	return nil
}

// ------------------------------------------------------------------
// Interface checks
// ------------------------------------------------------------------

var (
	_ fs.Fs             = (*Fs)(nil)
	_ fs.Copier         = (*Fs)(nil)
	_ fs.Mover          = (*Fs)(nil)
	_ fs.DirMover       = (*Fs)(nil)
	_ fs.Purger         = (*Fs)(nil)
	_ fs.PutStreamer    = (*Fs)(nil)
	_ fs.ListRer        = (*Fs)(nil)
	_ fs.ListPer        = (*Fs)(nil)
	_ fs.Abouter        = (*Fs)(nil)
	_ fs.DirSetModTimer = (*Fs)(nil)
	_ fs.Commander      = (*Fs)(nil)
	_ fs.Shutdowner     = (*Fs)(nil)
	_ fs.Object         = (*Object)(nil)
	_ fs.Metadataer     = (*Object)(nil)
	_ fs.SetMetadataer  = (*Object)(nil)
	_ fs.Directory      = (*Directory)(nil)
	_ fs.SetModTimer    = (*Directory)(nil)
)
