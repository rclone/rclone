//go:build unix

package nfs

import (
	"context"
	"os"
	"path"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsmeta"
	"github.com/willscott/go-nfs/file"
)

// setSys sets the Sys() call up for the vfs.Node passed in
//
// The billy abstraction layer does not extend to exposing `uid` and `gid`
// ownership of files. If ownership is important to your file system, you
// will need to ensure that the `os.FileInfo` meets additional constraints.
// In particular, the `Sys()` escape hatch is queried by this library, and
// if your file system populates a [`syscall.Stat_t`](https://golang.org/pkg/syscall/#Stat_t)
// concrete struct, the ownership specified in that object will be used.
// It can also return a file.FileInfo which is easier to manage cross platform
func setSys(fi os.FileInfo) {
	node, ok := fi.(vfs.Node)
	if !ok {
		fs.Errorf(fi, "internal error: %T is not a vfs.Node", fi)
		return
	}
	vv := node.VFS()
	opt := vv.Opt
	// Set the UID and GID for the node passed in from the VFS defaults.
	stat := file.FileInfo{
		Nlink:  1,
		UID:    opt.UID,
		GID:    opt.GID,
		Fileid: node.Inode(), // without this mounting doesn't work on Linux
	}
	if opt.PersistMetadataEnabled() && opt.PersistMetadataIncludes(vfscommon.MetadataFieldOwner) {
		if m, err := vv.LoadMetadata(context.TODO(), node.Path(), fi.IsDir()); err == nil {
			if m.UID != nil {
				stat.UID = *m.UID
			}
			if m.GID != nil {
				stat.GID = *m.GID
			}
		}
	}
	node.SetSys(&stat)
}

// FS is our wrapper around the VFS to properly support billy.Filesystem interface
type FS struct {
	vfs *vfs.VFS
}

// ReadDir implements read dir
func (f *FS) ReadDir(path string) (dir []os.FileInfo, err error) {
	defer log.Trace(path, "")("items=%d, err=%v", &dir, &err)
	dir, err = f.vfs.ReadDir(path)
	if err != nil {
		return nil, err
	}
	opt := f.vfs.Opt
	for i, fi := range dir {
		setSys(fi)
		if opt.PersistMetadataEnabled() {
			if n, ok := fi.(vfs.Node); ok {
				if m, err2 := f.vfs.LoadMetadata(context.TODO(), n.Path(), fi.IsDir()); err2 == nil {
					dir[i] = withOverlayFileInfo(fi, m)
				}
			}
		}
	}
	return dir, nil
}

// Create implements creating new files
func (f *FS) Create(filename string) (node billy.File, err error) {
	defer log.Trace(filename, "")("%v, err=%v", &node, &err)
	return f.vfs.Create(filename)
}

// Open opens a file
func (f *FS) Open(filename string) (node billy.File, err error) {
	defer log.Trace(filename, "")("%v, err=%v", &node, &err)
	return f.vfs.Open(filename)
}

// OpenFile opens a file
func (f *FS) OpenFile(filename string, flag int, perm os.FileMode) (node billy.File, err error) {
	defer log.Trace(filename, "flag=0x%X, perm=%v", flag, perm)("%v, err=%v", &node, &err)
	return f.vfs.OpenFile(filename, flag, perm)
}

// Stat gets the file stat
func (f *FS) Stat(filename string) (fi os.FileInfo, err error) {
	defer log.Trace(filename, "")("fi=%v, err=%v", &fi, &err)
	fi, err = f.vfs.Stat(filename)
	if err != nil {
		return nil, err
	}
	setSys(fi)
	// Overlay POSIX metadata on mode and times if available
	if f.vfs.Opt.PersistMetadataEnabled() {
		if m, err2 := f.vfs.LoadMetadata(context.TODO(), filename, fi.IsDir()); err2 == nil {
			fi = withOverlayFileInfo(fi, m)
		}
	}
	return fi, nil
}

// overlayFileInfo wraps os.FileInfo to override Mode and ModTime based on posix meta
type overlayFileInfo struct {
	os.FileInfo
	modeOverride  *os.FileMode
	mtimeOverride *time.Time
}

func (o overlayFileInfo) Mode() os.FileMode {
	if o.modeOverride != nil {
		return *o.modeOverride
	}
	return o.FileInfo.Mode()
}

func (o overlayFileInfo) ModTime() time.Time {
	if o.mtimeOverride != nil {
		return *o.mtimeOverride
	}
	return o.FileInfo.ModTime()
}

func withOverlayFileInfo(fi os.FileInfo, m vfsmeta.Meta) os.FileInfo {
	var om *os.FileMode
	var mt *time.Time
	if m.Mode != nil {
		perm := os.FileMode(*m.Mode) & os.ModePerm
		mode := (fi.Mode() & os.ModeType) | perm
		om = &mode
	}
	if m.Mtime != nil {
		t := m.Mtime.UTC()
		if !t.IsZero() {
			mt = &t
		}
	}
	if om == nil && mt == nil {
		return fi
	}
	return overlayFileInfo{FileInfo: fi, modeOverride: om, mtimeOverride: mt}
}

// Rename renames a file
func (f *FS) Rename(oldpath, newpath string) (err error) {
	defer log.Trace(oldpath, "newpath=%q", newpath)("err=%v", &err)
	return f.vfs.Rename(oldpath, newpath)
}

// Remove deletes a file
func (f *FS) Remove(filename string) (err error) {
	defer log.Trace(filename, "")("err=%v", &err)
	return f.vfs.Remove(filename)
}

// Join joins path elements
func (f *FS) Join(elem ...string) string {
	return path.Join(elem...)
}

// TempFile is not implemented
func (f *FS) TempFile(dir, prefix string) (node billy.File, err error) {
	defer log.Trace(dir, "prefix=%q", prefix)("node=%v, err=%v", &node, &err)
	return nil, os.ErrInvalid
}

// MkdirAll creates a directory and all the ones above it
// it does not redirect to VFS.MkDirAll because that one doesn't
// honor the permissions
func (f *FS) MkdirAll(filename string, perm os.FileMode) (err error) {
	defer log.Trace(filename, "perm=%v", perm)("err=%v", &err)
	parts := strings.Split(filename, "/")
	for i := range parts {
		current := strings.Join(parts[:i+1], "/")
		_, err := f.Stat(current)
		if err == vfs.ENOENT {
			err = f.vfs.Mkdir(current, perm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Lstat gets the stats for symlink
func (f *FS) Lstat(filename string) (fi os.FileInfo, err error) {
	defer log.Trace(filename, "")("fi=%v, err=%v", &fi, &err)
	fi, err = f.vfs.Stat(filename)
	if err != nil {
		return nil, err
	}
	setSys(fi)
	if f.vfs.Opt.PersistMetadataEnabled() {
		if m, err2 := f.vfs.LoadMetadata(context.TODO(), filename, fi.IsDir()); err2 == nil {
			fi = withOverlayFileInfo(fi, m)
		}
	}
	return fi, nil
}

// Symlink creates a link pointing to target
func (f *FS) Symlink(target, link string) (err error) {
	defer log.Trace(target, "link=%q", link)("err=%v", &err)
	return f.vfs.Symlink(target, link)
}

// Readlink reads the contents of link
func (f *FS) Readlink(link string) (result string, err error) {
	defer log.Trace(link, "")("result=%q, err=%v", &result, &err)
	return f.vfs.Readlink(link)
}

// Chmod changes the file modes
func (f *FS) Chmod(name string, mode os.FileMode) (err error) {
	defer log.Trace(name, "mode=%v", mode)("err=%v", &err)
	node, err := f.vfs.Stat(name)
	if err != nil {
		return err
	}
	opt := f.vfs.Opt
	if node.IsDir() {
		if opt.PersistMetadataIncludes(vfscommon.MetadataFieldMode) {
			v := uint32(mode)
			m := vfsmeta.Meta{Mode: &v}
			_ = f.vfs.SaveMetadata(context.TODO(), name, true, m)
		}
		return nil
	}
	file, err := f.vfs.Open(name)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fs.Logf(f, "Error while closing file: %e", err)
		}
	}()
	err = file.Chmod(mode)
	// Mask Chmod not implemented
	if err == vfs.ENOSYS {
		err = nil
	}
	if err == nil && opt.PersistMetadataIncludes(vfscommon.MetadataFieldMode) {
		v := uint32(mode)
		m := vfsmeta.Meta{Mode: &v}
		_ = f.vfs.SaveMetadata(context.TODO(), name, false, m)
	}
	return err
}

// Lchown changes the owner of symlink
func (f *FS) Lchown(name string, uid, gid int) (err error) {
	defer log.Trace(name, "uid=%d, gid=%d", uid, gid)("err=%v", &err)
	return f.Chown(name, uid, gid)
}

// Chown changes owner of the file
func (f *FS) Chown(name string, uid, gid int) (err error) {
	defer log.Trace(name, "uid=%d, gid=%d", uid, gid)("err=%v", &err)
	opt := f.vfs.Opt
	file, err := f.vfs.Open(name)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fs.Logf(f, "Error while closing file: %e", err)
		}
	}()
	err = file.Chown(uid, gid)
	if err == vfs.ENOSYS {
		err = nil
	}
	if err == nil && opt.PersistMetadataIncludes(vfscommon.MetadataFieldOwner) {
		u := uint32(uid)
		g := uint32(gid)
		m := vfsmeta.Meta{UID: &u, GID: &g}
		_ = f.vfs.SaveMetadata(context.TODO(), name, false, m)
	}
	return err
}

// Chtimes changes the access time and modified time
func (f *FS) Chtimes(name string, atime time.Time, mtime time.Time) (err error) {
	defer log.Trace(name, "atime=%v, mtime=%v", atime, mtime)("err=%v", &err)
	err = f.vfs.Chtimes(name, atime, mtime)
	if err == nil && f.vfs.Opt.PersistMetadataIncludes(vfscommon.MetadataFieldTimes) {
		a := atime.UTC()
		m := mtime.UTC()
		meta := vfsmeta.Meta{Atime: &a, Mtime: &m}
		_ = f.vfs.SaveMetadata(context.TODO(), name, false, meta)
	}
	return err
}

// Chroot is not supported in VFS
func (f *FS) Chroot(path string) (FS billy.Filesystem, err error) {
	defer log.Trace(path, "")("FS=%v, err=%v", &FS, &err)
	return nil, os.ErrInvalid
}

// Root  returns the root of a VFS
func (f *FS) Root() (root string) {
	defer log.Trace(nil, "")("root=%q", &root)
	return f.vfs.Fs().Root()
}

// Capabilities exports the filesystem capabilities
func (f *FS) Capabilities() (caps billy.Capability) {
	defer log.Trace(nil, "")("caps=%v", &caps)
	if f.vfs.Opt.CacheMode == vfscommon.CacheModeOff {
		return billy.ReadCapability | billy.SeekCapability
	}
	return billy.WriteCapability | billy.ReadCapability |
		billy.ReadAndWriteCapability | billy.SeekCapability | billy.TruncateCapability
}

// Interface check
var (
	_ billy.Filesystem = (*FS)(nil)
	_ billy.Change     = (*FS)(nil)
)
