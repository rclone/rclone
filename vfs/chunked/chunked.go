// Package chunked provides an infinite chunked file abstraction from
// the VFS.
//
// This can be used in the vfs layer to make chunked files, and in
// something like rclone serve nbd.
package chunked

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdfs "io/fs"
	"os"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

const (
	infoName           = "info.json" // name of chunk info file
	minChunkBits       = 4           // min size of chunk is 16 bytes
	maxChunkBits       = 30          // max size of chunk is 1 GB
	defaultChunkBits   = 16          // 64k chunks by default
	maxBufferChunks    = 1024        // default number of chunks in read buffer
	maxDirtyChunks     = 128         // default number of chuns in write buffer
	currentInfoVersion = 1           // version of the info file
)

// Info is serialized to the directory
type Info struct {
	Version   int    // version of chunk file
	Comment   string // note about this file
	Size      int64  // current size of the file
	ChunkBits uint   // number of bits in the chunk
	ChunkSize int    // must be power of two (1 << ChunkBits)
}

// File stores info about the file
type File struct {
	// these are read-only after creation so no locking required

	vfs       *vfs.VFS // underlying VFS
	dir       string   // path to directory
	chunkSize int      // size of a chunk 1 << info.ChunkBits
	mask      int64    // mask an offset onto a chunk boundary ^(chunkSize-1)
	chunkMask int64    // mask an offset into an intra chunk index (chunkSize-1)

	mu          sync.Mutex // lock for info
	opens       int        // number of file handles open on this File
	accessed    time.Time  // time file was last opened or closed
	valid       bool       // true if the info is valid
	info        Info       // info about the file
	infoRemote  string     // path to info object
	sizeChanged time.Time  // when the size was changed
}

// New creates a new chunked file at dir.
func New(vfs *vfs.VFS, dir string) (cf *File) {
	cf = &File{
		vfs:        vfs,
		dir:        dir,
		infoRemote: path.Join(dir, infoName),
	}
	return cf
}

// Open - open an existing file or create a new one with bits chunksize
//
// if create is not set then it will error if the file does not exist
//
// if bits is 0 then it uses the default value.
//
// Call Close() to show that you are no longer using this file.
//
// Open and Close can be called multiple times on one *File
func (cf *File) Open(create bool, bits uint) (err error) {
	cf.mu.Lock()
	defer cf.mu.Unlock()

	if bits == 0 {
		bits = defaultChunkBits
	}
	if bits < minChunkBits {
		return fmt.Errorf("chunk bits %d too small, must be >= %d", bits, minChunkBits)
	}
	if bits > maxChunkBits {
		return fmt.Errorf("chunk bits %d too large, must be <= %d", bits, maxChunkBits)
	}

	if !cf.valid {
		err = cf._readInfo()
		if err != nil && (!create || !errors.Is(err, stdfs.ErrNotExist)) {
			return fmt.Errorf("failed to open chunked file: read info failed: %w", err)
		}
		if err != nil {
			cf.info = Info{
				Size:      0,
				ChunkBits: bits,
				ChunkSize: 1 << bits,
				Version:   currentInfoVersion,
				Comment:   "rclone chunked file",
			}
			err = cf._writeInfo()
			if err != nil && err != fs.ErrorObjectNotFound {
				return fmt.Errorf("failed to open chunked file: write info failed: %w", err)
			}
		}
		cf.valid = true
		cf._updateChunkBits()
	}

	// Show another open
	cf.accessed = time.Now()
	cf.opens++
	return nil
}

// Close this *File
//
// It also writes the size out if it has changed and flushes the
// buffers.
//
// Open and Close can be called multiple times on one *File
func (cf *File) Close() error {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.accessed = time.Now()
	if cf.opens <= 0 {
		return errors.New("unbalanced open/close on File")
	}
	cf.opens--
	return cf._sync()
}

// sets all the constants which depend on cf.info.ChunkBits
//
// call with mu held
func (cf *File) _updateChunkBits() {
	cf.chunkSize = 1 << cf.info.ChunkBits
	cf.chunkMask = int64(cf.chunkSize - 1)
	cf.mask = ^cf.chunkMask
	cf.info.ChunkSize = cf.chunkSize
}

// makeChunkFileName makes a remote name for the chunk
func (cf *File) makeChunkFileName(off int64) string {
	if off&cf.chunkMask != 0 {
		panic("makeChunkFileName: non chunk aligned offset")
	}
	cf.mu.Lock()
	off >>= cf.info.ChunkBits
	Bits := 64 - cf.info.ChunkBits
	cf.mu.Unlock()
	Bytes := Bits >> 3
	// round up
	if Bits&7 != 0 {
		Bytes += 1
	}

	// Format to correct number of bytes
	// offS = "01234567"
	offS := fmt.Sprintf("%0*X", 2*Bytes, off)

	// Now interpolated / except for the last
	var out bytes.Buffer
	if cf.dir != "" {
		out.WriteString(cf.dir)
		out.WriteRune('/')
	}
	// out = "path/to/file/"
	for i := uint(0); i < Bytes-1; i++ {
		out.WriteString(offS[i*2 : i*2+2])
		out.WriteRune('/')
	}
	// out = "path/to/file/01/23/45/"
	// now add full string
	out.WriteString(offS)
	// out = "path/to/file/01/23/45/01234567"
	out.WriteString(".bin")
	// out = "path/to/file/01/23/45/01234567.bin"
	return out.String()
}

// readInfo writes the ChunkInfo to the object
//
// if it wasn't found then it returns fs.ErrorObjectNotFound
//
// Call with mu held
func (cf *File) _readInfo() (err error) {
	content, err := cf.vfs.ReadFile(cf.infoRemote)
	if err != nil {
		return fmt.Errorf("failed to find chunk info file %q: %w", cf.infoRemote, err)
	}
	err = json.Unmarshal(content, &cf.info)
	if err != nil {
		return fmt.Errorf("failed to decode chunk info file %q: %w", cf.infoRemote, err)
	}
	if cf.info.Version > currentInfoVersion {
		return fmt.Errorf("don't understand version %d info files (current version in %d)", cf.info.Version, currentInfoVersion)
	}
	if cf.info.ChunkBits < minChunkBits {
		return fmt.Errorf("chunk bits %d too small, must be >= %d", cf.info.ChunkBits, minChunkBits)
	}
	if cf.info.ChunkBits > maxChunkBits {
		return fmt.Errorf("chunk bits %d too large, must be <= %d", cf.info.ChunkBits, maxChunkBits)
	}
	return nil
}

// _writeInfo writes the ChunkInfo to the object
//
// call with mu held
func (cf *File) _writeInfo() (err error) {
	content, err := json.Marshal(&cf.info)
	if err != nil {
		return fmt.Errorf("failed to encode chunk info file %q: %w", cf.infoRemote, err)
	}
	err = cf.vfs.WriteFile(cf.infoRemote, content, 0600)
	if err != nil {
		return fmt.Errorf("failed to write chunk info file %q: %w", cf.infoRemote, err)
	}
	// show size is now unchanged
	cf.sizeChanged = time.Time{}
	return nil
}

// _writeSize writes the ChunkInfo if the size has changed
//
// call with mu held
func (cf *File) _writeSize() (err error) {
	if cf.sizeChanged.IsZero() {
		return nil
	}
	return cf._writeInfo()
}

// zeroBytes zeroes n bytes at the start of buf, or until the end of
// buf, whichever comes first.  It returns the number of bytes it
// wrote.
func zeroBytes(buf []byte, n int) int {
	if n > len(buf) {
		n = len(buf)
	}
	for i := 0; i < n; i++ {
		buf[i] = 0
	}
	return n
}

// Read bytes from the chunk at chunkStart from offset off in the
// chunk.
//
// Return number of bytes read
func (cf *File) chunkReadAt(b []byte, chunkStart int64, off int64) (n int, err error) {
	defer log.Trace(nil, "size=%d, chunkStart=%016x, off=%d", len(b), chunkStart, off)("n=%d, err=%v", &n, &err)
	fileName := cf.makeChunkFileName(chunkStart)
	if endPos := int64(cf.chunkSize) - off; endPos < int64(len(b)) {
		b = b[:endPos]
	}
	file, err := cf.vfs.Open(fileName)
	// If file doesn't exist, it is zero
	if errors.Is(err, stdfs.ErrNotExist) {
		return zeroBytes(b, len(b)), nil
	} else if err != nil {
		return 0, err
	}
	defer fs.CheckClose(file, &err)
	n, err = file.ReadAt(b, off)
	if err == io.EOF && off+int64(n) >= int64(cf.chunkSize) {
		err = nil
	}
	return
}

// ReadAt reads len(b) bytes from the File starting at byte offset off. It
// returns the number of bytes read and the error, if any. ReadAt always
// returns a non-nil error when n < len(b). At end of file, that error is
// io.EOF.
func (cf *File) ReadAt(b []byte, off int64) (n int, err error) {
	cf.mu.Lock()
	size := cf.info.Size
	cf.mu.Unlock()
	if off >= size {
		return 0, io.EOF
	}
	isEOF := false
	if bytesToEnd := size - off; bytesToEnd < int64(len(b)) {
		b = b[:bytesToEnd]
		isEOF = true
	}
	for n < len(b) {
		chunkStart := off & cf.mask
		end := n + cf.chunkSize
		if end > len(b) {
			end = len(b)
		}
		var nn int
		nn, err = cf.chunkReadAt(b[n:end], chunkStart, off-chunkStart)
		n += nn
		off += int64(nn)
		if err != nil {
			break
		}
	}
	if err == nil && isEOF {
		err = io.EOF
	}
	return
}

// Write b to the chunk at chunkStart at offset off
//
// Return number of bytes written
func (cf *File) chunkWriteAt(b []byte, chunkStart int64, off int64) (n int, err error) {
	defer log.Trace(nil, "size=%d, chunkStart=%016x, off=%d", len(b), chunkStart, off)("n=%d, err=%v", &n, &err)
	fileName := cf.makeChunkFileName(chunkStart)
	err = cf.vfs.MkdirAll(path.Dir(fileName), 0700)
	if err != nil {
		return 0, err
	}
	file, err := cf.vfs.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return 0, err
	}
	defer fs.CheckClose(file, &err)
	// Make the file full size if we can
	if cf.vfs.Opt.CacheMode >= vfscommon.CacheModeWrites {
		err = file.Truncate(int64(cf.chunkSize))
		if err != nil {
			return 0, err
		}
	}
	if endPos := int64(cf.chunkSize) - off; endPos < int64(len(b)) {
		b = b[:endPos]
	}
	return file.WriteAt(b, off)
}

// WriteAt writes len(b) bytes to the File starting at byte offset off. It
// returns the number of bytes written and an error, if any. WriteAt returns a
// non-nil error when n != len(b).
func (cf *File) WriteAt(b []byte, off int64) (n int, err error) {
	for n < len(b) {
		chunkStart := off & cf.mask
		var nn int
		end := n + cf.chunkSize
		if end > len(b) {
			end = len(b)
		}
		nn, err = cf.chunkWriteAt(b[n:end], chunkStart, off-chunkStart)
		n += nn
		off += int64(nn)
		if err != nil {
			break
		}
	}
	// Write new size if needed
	cf.mu.Lock()
	size := cf.info.Size
	if off > size {
		cf.info.Size = off // extend the file if necessary
		cf.sizeChanged = time.Now()
	}
	cf.mu.Unlock()
	return
}

// Size reads the current size of the file
func (cf *File) Size() int64 {
	cf.mu.Lock()
	if !cf.valid {
		err := cf._readInfo()
		if err != nil {
			fs.Errorf(cf.dir, "Failed to read size: %v", err)
		}
	}
	size := cf.info.Size
	cf.mu.Unlock()
	return size
}

// Truncate sets the current size of the file
//
// FIXME it doesn't delete any data...
func (cf *File) Truncate(size int64) error {
	cf.mu.Lock()
	if cf.info.Size != size {
		cf.info.Size = size
		cf.sizeChanged = time.Now()
	}
	cf.mu.Unlock()
	return nil
}

// _sync writes any pending data to disk by flushing the write queue
//
// call with the lock held
func (cf *File) _sync() error {
	err := cf._writeSize()
	// FIXME need a VFS function to flush everything to disk
	return err
}

// Sync writes any pending data to disk by flushing the write queue
func (cf *File) Sync() error {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	return cf._sync()
}

// Remove removes all the data in the file
func (cf *File) Remove() error {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	if !cf.valid {
		return nil
	}
	if cf.opens > 0 {
		return errors.New("can't delete chunked file when it is open")
	}
	cf.valid = false
	_ = cf._sync()

	// Purge all the files
	// FIXME should get this into the VFS as RemoveAll
	err := operations.Purge(context.TODO(), cf.vfs.Fs(), cf.dir)
	cf.vfs.FlushDirCache()

	return err
}
