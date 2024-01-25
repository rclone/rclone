//go:build unix
// +build unix

package nfs

import (
	"os"
	"path"
	"strings"
	"time"

	billy "github.com/go-git/go-billy/v5"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// FS is our wrapper around the VFS to properly support billy.Filesystem interface
type FS struct {
	vfs *vfs.VFS
}

// ReadDir implements read dir
func (f *FS) ReadDir(path string) (dir []os.FileInfo, err error) {
	fs.Debugf("nfs", "ReadDir %v", path)
	return f.vfs.ReadDir(path)
}

// Create implements creating new files
func (f *FS) Create(filename string) (billy.File, error) {
	fs.Debugf("nfs", "Create %v", filename)
	return f.vfs.Create(filename)
}

// Open opens a file
func (f *FS) Open(filename string) (billy.File, error) {
	file, err := f.vfs.Open(filename)
	fs.Debugf("nfs", "Open %v file: %v err: %v", filename, file, err)
	return file, err
}

// OpenFile opens a file
func (f *FS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	file, err := f.vfs.OpenFile(filename, flag, perm)
	fs.Debugf("nfs", "OpenFile %v flag: %v perm: %v file: %v err: %v", filename, flag, perm, file, err)
	return file, err
}

// Stat gets the file stat
func (f *FS) Stat(filename string) (os.FileInfo, error) {
	node, err := f.vfs.Stat(filename)
	fs.Debugf("nfs", "Stat %v node: %v err: %v", filename, node, err)
	return node, err
}

// Rename renames a file
func (f *FS) Rename(oldpath, newpath string) error {
	return f.vfs.Rename(oldpath, newpath)
}

// Remove deletes a file
func (f *FS) Remove(filename string) error {
	return f.vfs.Remove(filename)
}

// Join joins path elements
func (f *FS) Join(elem ...string) string {
	return path.Join(elem...)
}

// TempFile is not implemented
func (f *FS) TempFile(dir, prefix string) (billy.File, error) {
	return nil, os.ErrInvalid
}

// MkdirAll creates a directory and all the ones above it
// it does not redirect to VFS.MkDirAll because that one doesn't
// honor the permissions
func (f *FS) MkdirAll(filename string, perm os.FileMode) error {
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
func (f *FS) Lstat(filename string) (os.FileInfo, error) {
	node, err := f.vfs.Stat(filename)
	fs.Debugf("nfs", "Lstat %v node: %v err: %v", filename, node, err)
	return node, err
}

// Symlink is not supported over NFS
func (f *FS) Symlink(target, link string) error {
	return os.ErrInvalid
}

// Readlink is not supported
func (f *FS) Readlink(link string) (string, error) {
	return "", os.ErrInvalid
}

// Chmod changes the file modes
func (f *FS) Chmod(name string, mode os.FileMode) error {
	file, err := f.vfs.Open(name)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fs.Logf(f, "Error while closing file: %e", err)
		}
	}()
	return file.Chmod(mode)
}

// Lchown changes the owner of symlink
func (f *FS) Lchown(name string, uid, gid int) error {
	return f.Chown(name, uid, gid)
}

// Chown changes owner of the file
func (f *FS) Chown(name string, uid, gid int) error {
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
func (f *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return f.vfs.Chtimes(name, atime, mtime)
}

// Chroot is not supported in VFS
func (f *FS) Chroot(path string) (billy.Filesystem, error) {
	return nil, os.ErrInvalid
}

// Root  returns the root of a VFS
func (f *FS) Root() string {
	return f.vfs.Fs().Root()
}

// Capabilities exports the filesystem capabilities
func (f *FS) Capabilities() billy.Capability {
	if f.vfs.Opt.CacheMode == vfscommon.CacheModeOff {
		return billy.ReadCapability | billy.SeekCapability
	}
	return billy.WriteCapability | billy.ReadCapability |
		billy.ReadAndWriteCapability | billy.SeekCapability | billy.TruncateCapability
}

// Interface check
var _ billy.Filesystem = (*FS)(nil)
