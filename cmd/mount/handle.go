//go:build linux

package mount

import (
	"context"
	"io"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
)

// FileHandle is an open for read file handle on a File
type FileHandle struct {
	vfs.Handle
}

// Check interface satisfied
var _ fusefs.HandleReader = (*FileHandle)(nil)

// Read from the file handle
func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) (err error) {
	var n int
	defer log.Trace(fh, "len=%d, offset=%d", req.Size, req.Offset)("read=%d, err=%v", &n, &err)
	data := resp.Data[:req.Size]
	n, err = fh.Handle.ReadAt(data, req.Offset)
	resp.Data = data[:n]
	if err == io.EOF {
		err = nil
	}
	return translateError(err)
}

// Check interface satisfied
var _ fusefs.HandleWriter = (*FileHandle)(nil)

// Write data to the file handle
func (fh *FileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) (err error) {
	defer log.Trace(fh, "len=%d, offset=%d", len(req.Data), req.Offset)("written=%d, err=%v", &resp.Size, &err)
	n, err := fh.Handle.WriteAt(req.Data, req.Offset)
	if err != nil {
		return translateError(err)
	}
	resp.Size = n
	return nil
}

// Check interface satisfied
var _ fusefs.HandleFlusher = (*FileHandle)(nil)

// Flush is called on each close() of a file descriptor. So if a
// filesystem wants to return write errors in close() and the file has
// cached dirty data, this is a good place to write back data and
// return any errors. Since many applications ignore close() errors
// this is not always useful.
//
// NOTE: The flush() method may be called more than once for each
// open(). This happens if more than one file descriptor refers to an
// opened file due to dup(), dup2() or fork() calls. It is not
// possible to determine if a flush is final, so each flush should be
// treated equally. Multiple write-flush sequences are relatively
// rare, so this shouldn't be a problem.
//
// Filesystems shouldn't assume that flush will always be called after
// some writes, or that if will be called at all.
func (fh *FileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) (err error) {
	defer log.Trace(fh, "")("err=%v", &err)
	return translateError(fh.Handle.Flush())
}

var _ fusefs.HandleReleaser = (*FileHandle)(nil)

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) (err error) {
	defer log.Trace(fh, "")("err=%v", &err)
	return translateError(fh.Handle.Release())
}
