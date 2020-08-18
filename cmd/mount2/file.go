// +build linux darwin,amd64

package mount2

import (
	"context"
	"fmt"
	"io"
	"syscall"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
)

// FileHandle is a resource identifier for opened files. Usually, a
// FileHandle should implement some of the FileXxxx interfaces.
//
// All of the FileXxxx operations can also be implemented at the
// InodeEmbedder level, for example, one can implement NodeReader
// instead of FileReader.
//
// FileHandles are useful in two cases: First, if the underlying
// storage systems needs a handle for reading/writing. This is the
// case with Unix system calls, which need a file descriptor (See also
// the function `NewLoopbackFile`). Second, it is useful for
// implementing files whose contents are not tied to an inode. For
// example, a file like `/proc/interrupts` has no fixed content, but
// changes on each open call. This means that each file handle must
// have its own view of the content; this view can be tied to a
// FileHandle. Files that have such dynamic content should return the
// FOPEN_DIRECT_IO flag from their `Open` method. See directio_test.go
// for an example.
type FileHandle struct {
	h    vfs.Handle
	fsys *FS
}

// Create a new FileHandle
func newFileHandle(h vfs.Handle, fsys *FS) *FileHandle {
	return &FileHandle{
		h:    h,
		fsys: fsys,
	}
}

// Check interface satistfied
var _ fusefs.FileHandle = (*FileHandle)(nil)

// The String method is for debug printing.
func (f *FileHandle) String() string {
	return fmt.Sprintf("fh=%p(%s)", f, f.h.Node().Path())
}

// Read data from a file. The data should be returned as
// ReadResult, which may be constructed from the incoming
// `dest` buffer.
func (f *FileHandle) Read(ctx context.Context, dest []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	var n int
	var err error
	defer log.Trace(f, "off=%d", off)("n=%d, off=%d, errno=%v", &n, &off, &errno)
	n, err = f.h.ReadAt(dest, off)
	if err == io.EOF {
		err = nil
	}
	return fuse.ReadResultData(dest[:n]), translateError(err)
}

var _ fusefs.FileReader = (*FileHandle)(nil)

// Write the data into the file handle at given offset. After
// returning, the data will be reused and may not referenced.
func (f *FileHandle) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	var n int
	var err error
	defer log.Trace(f, "off=%d", off)("n=%d, off=%d, errno=%v", &n, &off, &errno)
	n, err = f.h.WriteAt(data, off)
	return uint32(n), translateError(err)
}

var _ fusefs.FileWriter = (*FileHandle)(nil)

// Flush is called for the close(2) call on a file descriptor. In case
// of a descriptor that was duplicated using dup(2), it may be called
// more than once for the same FileHandle.
func (f *FileHandle) Flush(ctx context.Context) syscall.Errno {
	return translateError(f.h.Flush())
}

var _ fusefs.FileFlusher = (*FileHandle)(nil)

// Release is called to before a FileHandle is forgotten. The
// kernel ignores the return value of this method,
// so any cleanup that requires specific synchronization or
// could fail with I/O errors should happen in Flush instead.
func (f *FileHandle) Release(ctx context.Context) syscall.Errno {
	return translateError(f.h.Release())
}

var _ fusefs.FileReleaser = (*FileHandle)(nil)

// Fsync is a signal to ensure writes to the Inode are flushed
// to stable storage.
func (f *FileHandle) Fsync(ctx context.Context, flags uint32) (errno syscall.Errno) {
	return translateError(f.h.Sync())
}

var _ fusefs.FileFsyncer = (*FileHandle)(nil)

// Getattr reads attributes for an Inode. The library will ensure that
// Mode and Ino are set correctly. For files that are not opened with
// FOPEN_DIRECTIO, Size should be set so it can be read correctly.  If
// returning zeroed permissions, the default behavior is to change the
// mode of 0755 (directory) or 0644 (files). This can be switched off
// with the Options.NullPermissions setting. If blksize is unset, 4096
// is assumed, and the 'blocks' field is set accordingly.
func (f *FileHandle) Getattr(ctx context.Context, out *fuse.AttrOut) (errno syscall.Errno) {
	defer log.Trace(f, "")("attr=%v, errno=%v", &out, &errno)
	f.fsys.setAttrOut(f.h.Node(), out)
	return 0
}

var _ fusefs.FileGetattrer = (*FileHandle)(nil)

// Setattr sets attributes for an Inode.
func (f *FileHandle) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	defer log.Trace(f, "in=%v", in)("attr=%v, errno=%v", &out, &errno)
	var err error
	f.fsys.setAttrOut(f.h.Node(), out)
	size, ok := in.GetSize()
	if ok {
		err = f.h.Truncate(int64(size))
		if err != nil {
			return translateError(err)
		}
		out.Attr.Size = size
	}
	mtime, ok := in.GetMTime()
	if ok {
		err = f.h.Node().SetModTime(mtime)
		if err != nil {
			return translateError(err)
		}
		out.Attr.Mtime = uint64(mtime.Unix())
		out.Attr.Mtimensec = uint32(mtime.Nanosecond())
	}
	return 0
}

var _ fusefs.FileSetattrer = (*FileHandle)(nil)
