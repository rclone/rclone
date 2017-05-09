// +build linux darwin freebsd

package mount

import (
	"errors"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs"
	"golang.org/x/net/context"
)

var errClosedFileHandle = errors.New("Attempt to use closed file handle")

// WriteFileHandle is an open for write handle on a File
type WriteFileHandle struct {
	*mountlib.WriteFileHandle
}

// Check interface satisfied
var _ fusefs.Handle = (*WriteFileHandle)(nil)

// Check interface satisfied
var _ fusefs.HandleWriter = (*WriteFileHandle)(nil)

// Write data to the file handle
func (fh *WriteFileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) (err error) {
	defer fs.Trace(fh, "len=%d, offset=%d", len(req.Data), req.Offset)("written=%d, err=%v", &resp.Size, &err)
	n, err := fh.WriteFileHandle.Write(req.Data, req.Offset)
	if err != nil {
		return translateError(err)
	}
	resp.Size = int(n)
	return nil
}

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
func (fh *WriteFileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) (err error) {
	defer fs.Trace(fh, "")("err=%v", &err)
	return translateError(fh.WriteFileHandle.Flush())
}

var _ fusefs.HandleReleaser = (*WriteFileHandle)(nil)

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *WriteFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) (err error) {
	defer fs.Trace(fh, "")("err=%v", &err)
	return translateError(fh.WriteFileHandle.Release())
}
