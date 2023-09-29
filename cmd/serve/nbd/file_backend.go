// Implements an nbd.Backend for serving from the VFS.

package nbd

import (
	"fmt"
	"os"

	"github.com/rclone/gonbdserver/nbd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"golang.org/x/net/context"
)

// Backend for a single file
type fileBackend struct {
	file vfs.Handle
	ec   *nbd.ExportConfig
}

// Create Backend for a single file
type fileBackendFactory struct {
	s        *NBD
	vfs      *vfs.VFS
	filePath string
	perms    int
}

// WriteAt implements Backend.WriteAt
func (fb *fileBackend) WriteAt(ctx context.Context, b []byte, offset int64, fua bool) (n int, err error) {
	defer log.Trace(logPrefix, "size=%d, off=%d", len(b), offset)("n=%d, err=%v", &n, &err)
	n, err = fb.file.WriteAt(b, offset)
	if err != nil || !fua {
		return n, err
	}
	err = fb.file.Sync()
	if err != nil {
		return 0, err
	}
	return n, err
}

// ReadAt implements Backend.ReadAt
func (fb *fileBackend) ReadAt(ctx context.Context, b []byte, offset int64) (n int, err error) {
	defer log.Trace(logPrefix, "size=%d, off=%d", len(b), offset)("n=%d, err=%v", &n, &err)
	return fb.file.ReadAt(b, offset)
}

// TrimAt implements Backend.TrimAt
func (fb *fileBackend) TrimAt(ctx context.Context, length int, offset int64) (n int, err error) {
	defer log.Trace(logPrefix, "size=%d, off=%d", length, offset)("n=%d, err=%v", &n, &err)
	return length, nil
}

// Flush implements Backend.Flush
func (fb *fileBackend) Flush(ctx context.Context) (err error) {
	defer log.Trace(logPrefix, "")("err=%v", &err)
	return nil
}

// Close implements Backend.Close
func (fb *fileBackend) Close(ctx context.Context) (err error) {
	defer log.Trace(logPrefix, "")("err=%v", &err)
	err = fb.file.Close()
	return nil
}

// Geometry implements Backend.Geometry
func (fb *fileBackend) Geometry(ctx context.Context) (size uint64, minBS uint64, prefBS uint64, maxBS uint64, err error) {
	defer log.Trace(logPrefix, "")("size=%d, minBS=%d, prefBS=%d, maxBS=%d, err=%v", &size, &minBS, &prefBS, &maxBS, &err)
	fi, err := fb.file.Stat()
	if err != nil {
		err = fmt.Errorf("failed read info about open backing file: %w", err)
		return
	}
	size = uint64(fi.Size())
	minBS = fb.ec.MinimumBlockSize
	prefBS = fb.ec.PreferredBlockSize
	maxBS = fb.ec.MaximumBlockSize
	err = nil
	return
}

// HasFua implements Backend.HasFua
func (fb *fileBackend) HasFua(ctx context.Context) (fua bool) {
	defer log.Trace(logPrefix, "")("fua=%v", &fua)
	return true
}

// HasFlush implements Backend.HasFua
func (fb *fileBackend) HasFlush(ctx context.Context) (flush bool) {
	defer log.Trace(logPrefix, "")("flush=%v", &flush)
	return true
}

// open the backing file
func (fbf *fileBackendFactory) open() (vfs.Handle, error) {
	return fbf.vfs.OpenFile(fbf.filePath, fbf.perms, 0700)
}

// New generates a new file backend
func (fbf *fileBackendFactory) newBackend(ctx context.Context, ec *nbd.ExportConfig) (nbd.Backend, error) {
	fd, err := fbf.open()
	if err != nil {
		return nil, fmt.Errorf("failed to open backing file: %w", err)
	}
	fb := &fileBackend{
		file: fd,
		ec:   ec,
	}
	return fb, nil
}

// Generate a file backend factory
func (s *NBD) newFileBackendFactory(ctx context.Context) (bf backendFactory, err error) {
	perms := os.O_RDWR
	if s.vfs.Opt.ReadOnly {
		perms = os.O_RDONLY
	}
	fbf := &fileBackendFactory{
		s:        s,
		vfs:      s.vfs,
		perms:    perms,
		filePath: s.leaf,
	}
	// Try opening the file so we get errors now rather than later when they are more difficult to report.
	fd, err := fbf.open()
	if err != nil {
		return nil, fmt.Errorf("failed to open backing file: %w", err)
	}
	defer fs.CheckClose(fd, &err)
	return fbf, nil
}

// Check interfaces
var (
	_ nbd.Backend    = (*fileBackend)(nil)
	_ backendFactory = (*fileBackendFactory)(nil)
)
