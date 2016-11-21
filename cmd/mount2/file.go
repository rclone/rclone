// +build linux darwin freebsd

package mount2

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/vfs"
)

// File represents a file
type File struct {
	h vfs.Handle
}

// Create a new File
func newFile(h vfs.Handle) *File {
	return &File{
		h: h,
	}
}

// Check interface satistfied
var _ nodefs.File = (*File)(nil)

// Called upon registering the filehandle in the inode.
func (f *File) SetInode(i *nodefs.Inode) {
}

// The String method is for debug printing.
func (f *File) String() string {
	return fmt.Sprintf("fh=%p(%s)", f, f.h.Node().Path())
}

// Wrappers around other File implementations, should return
// the inner file here.
func (f *File) InnerFile() nodefs.File {
	return nil
}

func (f *File) Read(dest []byte, off int64) (res fuse.ReadResult, code fuse.Status) {
	var n int
	var err error
	defer log.Trace(f, "off=%d", off)("n=%d, status=%v", &n, &code)
	n, err = f.h.ReadAt(dest, off)
	if err == io.EOF {
		err = nil
	}
	return fuse.ReadResultData(dest[:n]), translateError(err)
}

func (f *File) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	var n int
	var err error
	defer log.Trace(f, "off=%d", off)("n=%d, status=%v", &n, &code)
	n, err = f.h.WriteAt(data, off)
	return uint32(n), translateError(err)
}

// Flush is called for close() call on a file descriptor. In
// case of duplicated descriptor, it may be called more than
// once for a file.
func (f *File) Flush() fuse.Status {
	return translateError(f.h.Flush())
}

// This is called to before the file handle is forgotten. This
// method has no return value, so nothing can synchronizes on
// the call. Any cleanup that requires specific synchronization or
// could fail with I/O errors should happen in Flush instead.
func (f *File) Release() {
	f.h.Release()
}

func (f *File) Fsync(flags int) (code fuse.Status) {
	return translateError(f.h.Sync())
}

// The methods below may be called on closed files, due to
// concurrency.  In that case, you should return EBADF.
func (f *File) Truncate(size uint64) (code fuse.Status) {
	defer log.Trace(f, "size=%d", size)("status=%v", &code)
	return translateError(f.h.Truncate(int64(size)))
}

func (f *File) GetAttr(out *fuse.Attr) (code fuse.Status) {
	defer log.Trace(f, "")("attr=%v, status=%v", &out, &code)
	setAttr(f.h.Node(), out)
	return fuse.OK
}

func (f *File) Chown(uid uint32, gid uint32) fuse.Status {
	return translateError(f.h.Chown(int(uid), int(gid)))
}

func (f *File) Chmod(perms uint32) fuse.Status {
	return translateError(f.h.Chmod(os.FileMode(perms)))
}

func (f *File) Utimens(atime *time.Time, mtime *time.Time) (code fuse.Status) {
	defer log.Trace(f, "atime=%v, mtime=%v", atime, mtime)("status=%v", &code)
	if mtime == nil {
		return fuse.OK
	}
	return translateError(f.h.Node().SetModTime(*mtime))
}

func (f *File) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	return fuse.ENOSYS
}

func (f *File) Flock(flags int) fuse.Status {
	return fuse.ENOSYS
}
