// Package sqlar provides an interface to SQLite Archive files.
package sqlar

import (
	"bytes"
	"compress/zlib"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sync"

	sqlite3 "github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed" // Embedded SQLite WASM binary
)

// errBlobTooLarge is returned by insertBlobStreaming when the blob exceeds
// SQLite's maximum blob/string size (SQLITE_MAX_LENGTH, default ~1 GiB).
var errBlobTooLarge = errors.New("file too large: SQLite blobs are limited to ~1 GiB")

const schema = `CREATE TABLE IF NOT EXISTS sqlar(
  name TEXT PRIMARY KEY,
  mode INT,
  mtime INT,
  sz INT,
  data BLOB
)`

// sharedDB is a reference-counted wrapper around a *sql.DB so that multiple
// Fs instances pointing at the same archive file share one connection pool.
// This is critical on Windows where SQLite uses mandatory file locks and
// multiple independent pools competing for the same file causes deadlocks.
type sharedDB struct {
	db   *sql.DB
	mu   sync.Mutex // serializes SQLite writes (single-writer constraint)
	refs int
}

var (
	dbCacheMu sync.Mutex
	dbCache   = make(map[string]*sharedDB)
)

// openDB opens (or reuses) a SQLite database for the given file path.
// Call closeDB when done; the underlying *sql.DB is closed when the last
// reference is released.
func openDB(filePath string) (*sharedDB, error) {
	dbCacheMu.Lock()
	defer dbCacheMu.Unlock()

	if s, ok := dbCache[filePath]; ok {
		s.refs++
		return s, nil
	}

	const dsn = "?_pragma=busy_timeout(5000)" +
		"&_pragma=page_size(512)" +
		"&_pragma=synchronous(NORMAL)"
	db, err := driver.Open("file:" + filePath + dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %q: %w", filePath, err)
	}
	// Allow enough pooled connections so that concurrent blob reads
	// (which hold a *sql.Conn for streaming) don't starve writers.
	// SQLite's busy_timeout and the Fs.mu mutex handle write serialization.
	db.SetMaxOpenConns(4)

	s := &sharedDB{db: db, refs: 1}
	dbCache[filePath] = s
	return s, nil
}

// closeDB decrements the reference count and closes the underlying database
// when the last reference is released.
func closeDB(s *sharedDB, filePath string) error {
	dbCacheMu.Lock()
	defer dbCacheMu.Unlock()

	s.refs--
	if s.refs > 0 {
		return nil
	}
	delete(dbCache, filePath)
	return s.db.Close()
}

// initSchema creates the sqlar table if it doesn't exist.
func initSchema(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create sqlar table: %w", err)
	}
	return nil
}

// deflateCompress compresses data using zlib (RFC 1950), which is the format
// used by SQLite's sqlar_compress() function and therefore compatible with
// sqlite3 -A. Per SQLAR convention, returns the original data if compression
// doesn't reduce size (caller detects this when sz == len(data)).
func deflateCompress(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib writer: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return nil, fmt.Errorf("failed to compress: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finish compression: %w", err)
	}
	compressed := buf.Bytes()
	if len(compressed) >= len(data) {
		return data, nil
	}
	return compressed, nil
}

// blobReadSeekCloser holds a streaming read handle to a SQLite blob together
// with the *sql.Conn that keeps the underlying connection checked out.
// Close must be called when done; it closes the blob first then the connection.
type blobReadSeekCloser struct {
	conn *sql.Conn
	blob *sqlite3.Blob
}

func (b *blobReadSeekCloser) Read(p []byte) (int, error) {
	return b.blob.Read(p)
}

func (b *blobReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return b.blob.Seek(offset, whence)
}

func (b *blobReadSeekCloser) Close() error {
	blobErr := b.blob.Close()
	connErr := b.conn.Close()
	if blobErr != nil {
		return blobErr
	}
	return connErr
}

// openBlobReader opens a streaming read handle for the named sqlar entry.
// It returns the sz column value (original file size), the stored blob size,
// and an io.ReadSeekCloser that streams directly from the SQLite blob without
// loading the entire content into memory. The caller must Close the reader.
func openBlobReader(ctx context.Context, db *sql.DB, name string) (sz, blobLen int64, rc io.ReadSeekCloser, err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("sqlar: get conn: %w", err)
	}

	var rowid int64
	if err = conn.QueryRowContext(ctx,
		"SELECT rowid, sz, length(data) FROM sqlar WHERE name = ?", name,
	).Scan(&rowid, &sz, &blobLen); err != nil {
		_ = conn.Close()
		return 0, 0, nil, err
	}

	var blob *sqlite3.Blob
	if rawErr := conn.Raw(func(c any) error {
		dc, ok := c.(driver.Conn)
		if !ok {
			return fmt.Errorf("sqlar: unexpected conn type %T", c)
		}
		b, openErr := dc.Raw().OpenBlob("main", "sqlar", "data", rowid, false)
		if openErr != nil {
			return openErr
		}
		blob = b
		return nil
	}); rawErr != nil {
		_ = conn.Close()
		return 0, 0, nil, rawErr
	}

	return sz, blobLen, &blobReadSeekCloser{conn: conn, blob: blob}, nil
}

// insertBlobStreaming inserts a row into the sqlar table using incremental blob
// I/O. Rather than binding the entire blob as a SQL parameter (which requires
// copying the full content into the SQLite WASM heap at once), this function:
//   - inserts a zeroblob placeholder of the required size in a transaction
//   - streams data from r into the blob 1 MiB at a time via sqlite3_blob_write
//
// sz is the original file size stored in the sz column.
// blobSize is the number of bytes to write to the data column; it equals sz
// when stored uncompressed, and is less than sz when stored compressed.
func insertBlobStreaming(ctx context.Context, db *sql.DB, name string, mode int, mtime, sz, blobSize int64, r io.Reader) (int64, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("sqlar: get conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlar: begin tx: %w", err)
	}

	result, err := tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO sqlar (name, mode, mtime, sz, data) VALUES (?, ?, ?, ?, zeroblob(?))",
		name, mode, mtime, sz, blobSize)
	if err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sqlite3.TOOBIG) {
			return 0, errBlobTooLarge
		}
		return 0, err
	}
	rowid, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	err = conn.Raw(func(c any) error {
		dc, ok := c.(driver.Conn)
		if !ok {
			return fmt.Errorf("sqlar: unexpected conn type %T", c)
		}
		blob, blobErr := dc.Raw().OpenBlob("main", "sqlar", "data", rowid, true)
		if blobErr != nil {
			return blobErr
		}
		defer func() { _ = blob.Close() }()
		_, err := blob.ReadFrom(r)
		return err
	})
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlar: commit: %w", err)
	}
	return rowid, nil
}
