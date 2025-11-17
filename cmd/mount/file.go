//go:build linux

package mount

import (
	"context"
	"os"
	"syscall"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsmeta"
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
	a.Mode = f.File.Mode() &^ os.ModeAppend
	a.Size = Size
	a.Atime = modTime
	a.Mtime = modTime
	a.Ctime = modTime
	opt := f.VFS().Opt
	if opt.PersistMetadataEnabled() {
		if m, err2 := f.VFS().LoadMetadata(ctx, f.Path(), false); err2 == nil {
			if opt.PersistMetadataIncludes(vfscommon.MetadataFieldMode) && m.Mode != nil {
				perm := os.FileMode(*m.Mode) & os.ModePerm
				a.Mode = (a.Mode & os.ModeType) | perm
			}
			if opt.PersistMetadataIncludes(vfscommon.MetadataFieldOwner) {
				if m.UID != nil {
					a.Uid = *m.UID
				}
				if m.GID != nil {
					a.Gid = *m.GID
				}
			}
			if opt.PersistMetadataIncludes(vfscommon.MetadataFieldTimes) {
				if m.Mtime != nil {
					a.Mtime = m.Mtime.UTC()
				}
				if m.Atime != nil {
					a.Atime = m.Atime.UTC()
				}
			}
		}
	}

	a.Blocks = Blocks
	return nil
}

// Check interface satisfied
var _ fusefs.NodeSetattrer = (*File)(nil)

// Setattr handles attribute changes from FUSE. Currently supports ModTime and Size only
func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) (err error) {
	defer log.Trace(f, "a=%+v", req)("err=%v", &err)
	opt := f.VFS().Opt

	if req.Valid.Size() {
		if e := f.File.Truncate(int64(req.Size)); err == nil {
			err = e
		}
	}

	if !opt.NoModTime {
		if req.Valid.Mtime() {
			if e := f.File.SetModTime(req.Mtime); err == nil {
				err = e
			}
		} else if req.Valid.MtimeNow() {
			if e := f.File.SetModTime(time.Now()); err == nil {
				err = e
			}
		}
	}

	if opt.PersistMetadataEnabled() {
		var m vfsmeta.Meta
		changed := false
		if opt.PersistMetadataIncludes(vfscommon.MetadataFieldMode) && req.Valid.Mode() {
			v := uint32(req.Mode)
			m.Mode = &v
			changed = true
		}
		if opt.PersistMetadataIncludes(vfscommon.MetadataFieldOwner) && req.Valid.Uid() {
			v := req.Uid
			m.UID = &v
			changed = true
		}
		if opt.PersistMetadataIncludes(vfscommon.MetadataFieldOwner) && req.Valid.Gid() {
			v := req.Gid
			m.GID = &v
			changed = true
		}
		if opt.PersistMetadataIncludes(vfscommon.MetadataFieldTimes) && (req.Valid.Atime() || req.Valid.AtimeNow()) {
			t := req.Atime
			if req.Valid.AtimeNow() {
				t = time.Now()
			}
			m.Atime = &t
			changed = true
		}
		if opt.PersistMetadataIncludes(vfscommon.MetadataFieldTimes) && (req.Valid.Mtime() || req.Valid.MtimeNow()) {
			t := req.Mtime
			if req.Valid.MtimeNow() {
				t = time.Now()
			}
			m.Mtime = &t
			changed = true
		}
		if changed {
			if err2 := f.VFS().SaveMetadata(ctx, f.Path(), false, m); err2 != nil {
				fs.Debugf(f, "persist metadata failed: %v", err2)
			}
			_ = f.fsys.server.InvalidateNodeAttr(f)
		}
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

var _ fusefs.NodeReadlinker = (*File)(nil)

// Readlink read symbolic link target.
func (f *File) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (ret string, err error) {
	defer log.Trace(f, "")("ret=%v, err=%v", &ret, &err)
	return f.VFS().Readlink(f.Path())
}
