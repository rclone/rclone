// Implements an nbd.Backend for serving from a chunked file in the VFS.

package nbd

import (
	"errors"
	"fmt"

	"github.com/rclone/gonbdserver/nbd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs/chunked"
	"golang.org/x/net/context"
)

// Backend for a single chunked file
type chunkedBackend struct {
	file *chunked.File
	ec   *nbd.ExportConfig
}

// Create Backend for a single chunked file
type chunkedBackendFactory struct {
	s    *NBD
	file *chunked.File
}

// WriteAt implements Backend.WriteAt
func (cb *chunkedBackend) WriteAt(ctx context.Context, b []byte, offset int64, fua bool) (n int, err error) {
	defer log.Trace(logPrefix, "size=%d, off=%d", len(b), offset)("n=%d, err=%v", &n, &err)
	n, err = cb.file.WriteAt(b, offset)
	if err != nil || !fua {
		return n, err
	}
	err = cb.file.Sync()
	if err != nil {
		return 0, err
	}
	return n, err
}

// ReadAt implements Backend.ReadAt
func (cb *chunkedBackend) ReadAt(ctx context.Context, b []byte, offset int64) (n int, err error) {
	defer log.Trace(logPrefix, "size=%d, off=%d", len(b), offset)("n=%d, err=%v", &n, &err)
	return cb.file.ReadAt(b, offset)
}

// TrimAt implements Backend.TrimAt
func (cb *chunkedBackend) TrimAt(ctx context.Context, length int, offset int64) (n int, err error) {
	defer log.Trace(logPrefix, "size=%d, off=%d", length, offset)("n=%d, err=%v", &n, &err)
	return length, nil
}

// Flush implements Backend.Flush
func (cb *chunkedBackend) Flush(ctx context.Context) (err error) {
	defer log.Trace(logPrefix, "")("err=%v", &err)
	return nil
}

// Close implements Backend.Close
func (cb *chunkedBackend) Close(ctx context.Context) (err error) {
	defer log.Trace(logPrefix, "")("err=%v", &err)
	err = cb.file.Close()
	return nil
}

// Geometry implements Backend.Geometry
func (cb *chunkedBackend) Geometry(ctx context.Context) (size uint64, minBS uint64, prefBS uint64, maxBS uint64, err error) {
	defer log.Trace(logPrefix, "")("size=%d, minBS=%d, prefBS=%d, maxBS=%d, err=%v", &size, &minBS, &prefBS, &maxBS, &err)
	size = uint64(cb.file.Size())
	minBS = cb.ec.MinimumBlockSize
	prefBS = cb.ec.PreferredBlockSize
	maxBS = cb.ec.MaximumBlockSize
	err = nil
	return
}

// HasFua implements Backend.HasFua
func (cb *chunkedBackend) HasFua(ctx context.Context) (fua bool) {
	defer log.Trace(logPrefix, "")("fua=%v", &fua)
	return true
}

// HasFlush implements Backend.HasFua
func (cb *chunkedBackend) HasFlush(ctx context.Context) (flush bool) {
	defer log.Trace(logPrefix, "")("flush=%v", &flush)
	return true
}

// New generates a new chunked backend
func (cbf *chunkedBackendFactory) newBackend(ctx context.Context, ec *nbd.ExportConfig) (nbd.Backend, error) {
	err := cbf.file.Open(false, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open chunked file: %w", err)
	}
	cb := &chunkedBackend{
		file: cbf.file,
		ec:   ec,
	}
	return cb, nil
}

// Generate a chunked backend factory
func (s *NBD) newChunkedBackendFactory(ctx context.Context) (bf backendFactory, err error) {
	create := s.opt.Create > 0
	if s.vfs.Opt.ReadOnly && create {
		return nil, errors.New("can't create files with --read-only")
	}
	file := chunked.New(s.vfs, s.leaf)
	err = file.Open(create, s.log2ChunkSize)
	if err != nil {
		return nil, fmt.Errorf("failed to open chunked file: %w", err)
	}
	defer fs.CheckClose(file, &err)
	var truncateSize fs.SizeSuffix
	if create {
		if file.Size() == 0 {
			truncateSize = s.opt.Create
		}
	} else {
		truncateSize = s.opt.Resize
	}
	if truncateSize > 0 {
		err = file.Truncate(int64(truncateSize))
		if err != nil {
			return nil, fmt.Errorf("failed to create chunked file: %w", err)
		}
		fs.Logf(logPrefix, "Size of network block device is now %v", truncateSize)
	}
	return &chunkedBackendFactory{
		s:    s,
		file: file,
	}, nil
}

// Check interfaces
var (
	_ nbd.Backend    = (*chunkedBackend)(nil)
	_ backendFactory = (*chunkedBackendFactory)(nil)
)
