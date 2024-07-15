//go:build linux

package mount

import (
	"context"
	"os"
	"syscall"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
)

// File represents a file
type File struct {
	*vfs.File
	fsys *FS
}

// Check interface satisfied
var _ fusefs.Node = (*File)(nil)

// Attr fills out the attributes for the file
func (f *File) Attr(ctx context.Context, a *fuse.Attr) (err error) {
	defer log.Trace(f, "")("a=%+v, err=%v", a, &err)
	a.Valid = time.Duration(f.fsys.opt.AttrTimeout)
	modTime := f.File.ModTime()
	Size := uint64(f.File.Size())
	Blocks := (Size + 511) / 512
	a.Gid = f.VFS().Opt.GID
	a.Uid = f.VFS().Opt.UID
	a.Mode = os.FileMode(f.VFS().Opt.FilePerms)
	a.Size = Size
	a.Atime = modTime
	a.Mtime = modTime
	a.Ctime = modTime
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
	handle, err := f.File.Open(int(req.Flags))
	if err != nil {
		return nil, translateError(err)
	}

	// If size unknown then use direct io to read
	if entry := handle.Node().DirEntry(); entry != nil && entry.Size() < 0 {
		resp.Flags |= fuse.OpenDirectIO
	}
	if f.fsys.opt.DirectIO {
		resp.Flags |= fuse.OpenDirectIO
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

// Getxattr gets an extended attribute by the given name from the
// node.
//
// If there is no xattr by that name, returns fuse.ErrNoXattr.
func (f *File) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	return syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeGetxattrer = (*File)(nil)

// Listxattr lists the extended attributes recorded for the node.
func (f *File) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	return syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeListxattrer = (*File)(nil)

// Setxattr sets an extended attribute with the given name and
// value for the node.
func (f *File) Setxattr(ctx context.Context, req *fuse.SetxattrRequest) error {
	return syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeSetxattrer = (*File)(nil)

// Removexattr removes an extended attribute for the name.
//
// If there is no xattr by that name, returns fuse.ErrNoXattr.
func (f *File) Removexattr(ctx context.Context, req *fuse.RemovexattrRequest) error {
	return syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeRemovexattrer = (*File)(nil)
