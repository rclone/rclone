//go:build unix

package nfs

import (
	"math"
	"os"
	"path"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
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
	}
	vfs := node.VFS()
	// Set the UID and GID for the node passed in from the VFS defaults.
	stat := file.FileInfo{
		Nlink:  1,
		UID:    vfs.Opt.UID,
		GID:    vfs.Opt.GID,
		Fileid: math.MaxUint64, // without this mounting doesn't work on Linux
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
	for _, fi := range dir {
		setSys(fi)
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
	return fi, nil
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
	return fi, nil
}

// Symlink is not supported over NFS
func (f *FS) Symlink(target, link string) (err error) {
	defer log.Trace(target, "link=%q", link)("err=%v", &err)
	return os.ErrInvalid
}

// Readlink is not supported
func (f *FS) Readlink(link string) (result string, err error) {
	defer log.Trace(link, "")("result=%q, err=%v", &result, &err)
	return "", os.ErrInvalid
}

// Chmod changes the file modes
func (f *FS) Chmod(name string, mode os.FileMode) (err error) {
	defer log.Trace(name, "mode=%v", mode)("err=%v", &err)
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
	file, err := f.vfs.Open(name)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fs.Logf(f, "Error while closing file: %e", err)
		}
	}()
	return file.Chown(uid, gid)
}

// Chtimes changes the acces time and modified time
func (f *FS) Chtimes(name string, atime time.Time, mtime time.Time) (err error) {
	defer log.Trace(name, "atime=%v, mtime=%v", atime, mtime)("err=%v", &err)
	return f.vfs.Chtimes(name, atime, mtime)
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
