// +build linux darwin freebsd

package mount

import (
	"os"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/cmd/mountlib"
	"github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/vfs"
	"golang.org/x/net/context"
)

// File represents a file
type File struct {
	*vfs.File
}

// Check interface satisfied
var _ fusefs.Node = (*File)(nil)

// Attr fills out the attributes for the file
func (f *File) Attr(ctx context.Context, a *fuse.Attr) (err error) {
	defer log.Trace(f, "")("a=%+v, err=%v", a, &err)
	a.Valid = mountlib.AttrTimeout
	modTime := f.File.ModTime()
	Size := uint64(f.File.Size())
	Blocks := (Size + 511) / 512
	a.Gid = f.VFS().Opt.GID
	a.Uid = f.VFS().Opt.UID
	a.Mode = f.VFS().Opt.FilePerms
	a.Size = Size
	a.Atime = modTime
	a.Mtime = modTime
	a.Ctime = modTime
	a.Crtime = modTime
	a.Blocks = Blocks
	return nil
}

// Check interface satisfied
var _ fusefs.NodeSetattrer = (*File)(nil)

// Setattr handles attribute changes from FUSE. Currently supports ModTime and Size only
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) (err error) {
	defer log.Trace(f, "a=%+v", req)("err=%v", &err)
	if !f.VFS().Opt.NoModTime {
		if req.Valid.Mtime() {
			err = f.File.SetModTime(req.Mtime)
		} else if req.Valid.MtimeNow() {
			err = f.File.SetModTime(time.Now())
		}
	}
	if req.Valid.Size() {
		err = f.File.Truncate(int64(req.Size))
	}
	return translateError(err)
}

// Check interface satisfied
var _ fusefs.NodeOpener = (*File)(nil)

// Open the file for read or write
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fh fusefs.Handle, err error) {
	defer log.Trace(f, "flags=%v", req.Flags)("fh=%v, err=%v", &fh, &err)

	// fuse flags are based off syscall flags as are os flags, so
	// should be compatible
	//
	// we seem to be missing O_CREATE here so add it in to allow
	// file creation
	handle, err := f.File.Open(int(req.Flags) | os.O_CREATE)
	if err != nil {
		return nil, translateError(err)
	}

	// See if seeking is supported and set FUSE hint accordingly
	if _, err = handle.Seek(0, 1); err != nil {
		resp.Flags |= fuse.OpenNonSeekable
	}

	return &FileHandle{handle}, nil
}

// Check interface satisfied
var _ fusefs.NodeFsyncer = (*File)(nil)

// Fsync the file
//
// Note that we don't do anything except return OK
func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) (err error) {
	defer log.Trace(f, "")("err=%v", &err)
	return nil
}
