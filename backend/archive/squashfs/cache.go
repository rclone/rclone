package squashfs

// Could just be using bare object Open with RangeRequest which
// would transfer the minimum amount of data but may be slower.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/diskfs/go-diskfs/backend"
	"github.com/rclone/rclone/vfs"
)

// Cache file handles for accessing the file
type cache struct {
	node  vfs.Node
	fhsMu sync.Mutex
	fhs   []cacheHandle
}

// A cached file handle
type cacheHandle struct {
	offset int64
	fh     vfs.Handle
}

// Make a new cache
func newCache(node vfs.Node) *cache {
	return &cache{
		node: node,
	}
}

// Get a vfs.Handle from the pool or open one
//
// This tries to find an open file handle which doesn't require seeking.
func (c *cache) open(off int64) (fh vfs.Handle, err error) {
	c.fhsMu.Lock()
	defer c.fhsMu.Unlock()

	if len(c.fhs) > 0 {
		// Look for exact match first
		for i, cfh := range c.fhs {
			if cfh.offset == off {
				// fs.Debugf(nil, "CACHE MATCH")
				c.fhs = append(c.fhs[:i], c.fhs[i+1:]...)
				return cfh.fh, nil

			}
		}
		// fs.Debugf(nil, "CACHE MISS")
		// Just take the first one if not found
		cfh := c.fhs[0]
		c.fhs = c.fhs[1:]
		return cfh.fh, nil
	}

	fh, err = c.node.Open(os.O_RDONLY)
	if err != nil {
		return nil, fmt.Errorf("failed to open squashfs archive: %w", err)
	}

	return fh, nil
}

// Close a vfs.Handle or return it to the pool
//
// off should be the offset the file handle would read from without seeking
func (c *cache) close(fh vfs.Handle, off int64) {
	c.fhsMu.Lock()
	defer c.fhsMu.Unlock()

	c.fhs = append(c.fhs, cacheHandle{
		offset: off,
		fh:     fh,
	})
}

// ReadAt reads len(p) bytes into p starting at offset off in the underlying
// input source. It returns the number of bytes read (0 <= n <= len(p)) and any
// error encountered.
//
// When ReadAt returns n < len(p), it returns a non-nil error explaining why
// more bytes were not returned. In this respect, ReadAt is stricter than Read.
//
// Even if ReadAt returns n < len(p), it may use all of p as scratch
// space during the call. If some data is available but not len(p) bytes,
// ReadAt blocks until either all the data is available or an error occurs.
// In this respect ReadAt is different from Read.
//
// If the n = len(p) bytes returned by ReadAt are at the end of the input
// source, ReadAt may return either err == EOF or err == nil.
//
// If ReadAt is reading from an input source with a seek offset, ReadAt should
// not affect nor be affected by the underlying seek offset.
//
// Clients of ReadAt can execute parallel ReadAt calls on the same input
// source.
//
// Implementations must not retain p.
func (c *cache) ReadAt(p []byte, off int64) (n int, err error) {
	fh, err := c.open(off)
	if err != nil {
		return n, err
	}
	defer func() {
		c.close(fh, off+int64(len(p)))
	}()
	// fs.Debugf(nil, "ReadAt(p[%d], off=%d, fh=%p)", len(p), off, fh)
	return fh.ReadAt(p, off)
}

var errCacheNotImplemented = errors.New("internal error: squashfs cache doesn't implement method")

// WriteAt method dummy stub to satisfy interface
func (c *cache) WriteAt(p []byte, off int64) (n int, err error) {
	return 0, errCacheNotImplemented
}

// Seek method dummy stub to satisfy interface
func (c *cache) Seek(offset int64, whence int) (int64, error) {
	return 0, errCacheNotImplemented
}

// Read method dummy stub to satisfy interface
func (c *cache) Read(p []byte) (n int, err error) {
	return 0, errCacheNotImplemented
}

func (c *cache) Stat() (fs.FileInfo, error) {
	return nil, errCacheNotImplemented
}

// Close the file
func (c *cache) Close() (err error) {
	c.fhsMu.Lock()
	defer c.fhsMu.Unlock()

	// Close any open file handles
	for i := range c.fhs {
		fh := &c.fhs[i]
		newErr := fh.fh.Close()
		if err == nil {
			err = newErr
		}
	}
	c.fhs = nil
	return err
}

// Sys returns OS-specific file for ioctl calls via fd
func (c *cache) Sys() (*os.File, error) {
	return nil, errCacheNotImplemented
}

// Writable returns file for read-write operations
func (c *cache) Writable() (backend.WritableFile, error) {
	return nil, errCacheNotImplemented
}

// check interfaces
var _ backend.Storage = (*cache)(nil)
