// FUSE main Fs

// +build linux darwin freebsd

package mount2

import (
	"os"
	"path"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/log"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	"github.com/pkg/errors"
)

// FS represents the top level filing system
type FS struct {
	pathfs.FileSystem
	VFS     *vfs.VFS
	f       fs.Fs
	mounted chan struct{} // closed when mount is ready
}

// NewFS creates a pathfs.FileSystem from the fs.Fs passed in
func NewFS(f fs.Fs) *FS {
	fsys := &FS{
		FileSystem: pathfs.NewDefaultFileSystem(),
		VFS:        vfs.New(f, &vfsflags.Opt),
		f:          f,
		mounted:    make(chan struct{}),
	}
	return fsys
}

// Check interface satistfied
var _ pathfs.FileSystem = (*FS)(nil)

// String used for pretty printing.
func (fsys *FS) String() string {
	return fsys.f.Name() + ":" + fsys.f.Root()
}

// If called, provide debug output through the log package.
func (fsys *FS) SetDebug(debug bool) {
	fs.Debugf(fsys.f, "SetDebug %v", debug)
}

// lookup a Node given a path
func (fsys *FS) lookupNode(path string) (node vfs.Node, code fuse.Status) {
	node, err := fsys.VFS.Stat(path)
	return node, translateError(err)
}

// lookup a Dir given a path
func (fsys *FS) lookupDir(path string) (dir *vfs.Dir, code fuse.Status) {
	node, code := fsys.lookupNode(path)
	if !code.Ok() {
		return nil, code
	}
	dir, ok := node.(*vfs.Dir)
	if !ok {
		return nil, fuse.ENOTDIR
	}
	return dir, fuse.OK
}

// lookup a parent Dir given a path returning the dir and the leaf
func (fsys *FS) lookupParentDir(filePath string) (leaf string, dir *vfs.Dir, code fuse.Status) {
	parentDir, leaf := path.Split(filePath)
	dir, code = fsys.lookupDir(parentDir)
	return leaf, dir, code
}

// fill in attr from node
func setAttr(node vfs.Node, attr *fuse.Attr) {
	Size := uint64(node.Size())
	const BlockSize = 512
	Blocks := (Size + BlockSize - 1) / BlockSize
	modTime := node.ModTime()
	Mode := node.Mode().Perm()
	if node.IsDir() {
		Mode |= fuse.S_IFDIR
	} else {
		Mode |= fuse.S_IFREG
	}
	// set attributes
	vfs := node.VFS()
	attr.Owner.Gid = vfs.Opt.UID
	attr.Owner.Uid = vfs.Opt.GID
	attr.Mode = uint32(Mode)
	attr.Size = Size
	attr.Nlink = 1
	attr.Blocks = Blocks
	attr.Blksize = BlockSize
	s := uint64(modTime.Unix())
	ns := uint32(modTime.Nanosecond())
	attr.Atime = s
	attr.Atimensec = ns
	attr.Mtime = s
	attr.Mtimensec = ns
	attr.Ctime = s
	attr.Ctimensec = ns
	//attr.Rdev
}

// Attributes.  This function is the main entry point, through
// which FUSE discovers which files and directories exist.
//
// If the filesystem wants to implement hard-links, it should
// return consistent non-zero FileInfo.Ino data.  Using
// hardlinks incurs a performance hit.
func (fsys *FS) GetAttr(name string, context *fuse.Context) (attr *fuse.Attr, code fuse.Status) {
	if noisyDebug {
		defer log.Trace(name, "")("attr=%v, status=%v", &attr, &code)
	}

	node, code := fsys.lookupNode(name)
	if !code.Ok() {
		return nil, code
	}

	attr = new(fuse.Attr)
	setAttr(node, attr)
	return attr, fuse.OK
}

func (fsys *FS) Utimens(name string, Atime *time.Time, Mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	defer log.Trace(name, "Atime=%v, Mtime=%v", Atime, Mtime)("status=%v", &code)
	node, code := fsys.lookupNode(name)
	if !code.Ok() {
		return code
	}
	if Mtime != nil {
		code = translateError(node.SetModTime(*Mtime))
	}
	return code
}

func (fsys *FS) Truncate(name string, size uint64, context *fuse.Context) (code fuse.Status) {
	defer log.Trace(name, "size=%d", size)("code=%d", &code)
	// We just kind of ignore Truncate, since the next operation
	// will be a write to the file which will truncate it.
	return fuse.OK
}

// func (fsys *FS) Access(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
// 	fs.Debugf(name, "Access")
// 	return fuse.ENOSYS
// }

// Tree structure
// func (fsys *FS) Link(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
// 	fs.Debugf(oldName, "Link %q", newName)
// 	return fuse.ENOSYS
// }

func (fsys *FS) Mkdir(name string, mode uint32, context *fuse.Context) (code fuse.Status) {
	defer log.Trace(name, "mode=0%o", mode)("status=%v", &code)
	leaf, parentDir, code := fsys.lookupParentDir(name)
	if !code.Ok() {
		return code
	}
	_, err := parentDir.Mkdir(leaf)
	return translateError(err)
}

func (fsys *FS) Rename(oldName string, newName string, context *fuse.Context) (code fuse.Status) {
	defer log.Trace(oldName, "newName=%q", newName)("status=%v", &code)
	oldDir, code := fsys.lookupDir(oldName)
	if !code.Ok() {
		return code
	}
	newDir, code := fsys.lookupDir(newName)
	if !code.Ok() {
		return code
	}
	return translateError(oldDir.Rename(oldName, newName, newDir))
}

func (fsys *FS) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	defer log.Trace(name, "")("status=%v", &code)
	leaf, parentDir, code := fsys.lookupParentDir(name)
	if !code.Ok() {
		return code
	}
	return translateError(parentDir.RemoveName(leaf))
}

func (fsys *FS) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	defer log.Trace(name, "")("status=%v", &code)
	fs.Debugf(name, "Unlink")
	leaf, parentDir, code := fsys.lookupParentDir(name)
	if !code.Ok() {
		return code
	}
	return translateError(parentDir.RemoveName(leaf))
}

// Called after mount.
func (fsys *FS) OnMount(nodeFs *pathfs.PathNodeFs) {
	fs.Debugf(fsys.f, "Mounted")
	close(fsys.mounted)
}

func (fsys *FS) OnUnmount() {
	fs.Debugf(fsys.f, "Unmounted")
}

// File handling.  If opening for writing, the file's mtime
// should be updated too.
func (fsys *FS) Open(name string, flags uint32, context *fuse.Context) (fh nodefs.File, code fuse.Status) {
	defer log.Trace(name, "flags=%#o", flags)("status=%v", &code)
	// translate the fuse flags to os flags
	handle, err := fsys.VFS.OpenFile(name, int(flags), 0777)
	if err != nil {
		return nil, translateError(err)
	}
	return newFile(handle), fuse.OK
}

func (fsys *FS) Create(name string, flags uint32, mode uint32, context *fuse.Context) (fh nodefs.File, code fuse.Status) {
	defer log.Trace(name, "flags=%#o, mode=%#o", flags, mode)("handle=%v, status=%v", &fh, &code)
	leaf, parentDir, code := fsys.lookupParentDir(name)
	if !code.Ok() {
		return nil, code
	}
	// translate the fuse flags to os flags
	osFlags := int(flags) | os.O_CREATE
	file, err := parentDir.Create(leaf, osFlags)
	if err != nil {
		return nil, translateError(err)
	}
	handle, err := file.Open(osFlags)
	if err != nil {
		return nil, translateError(err)
	}
	fh = newFile(handle)
	fh = &nodefs.WithFlags{
		File:      fh,
		FuseFlags: fuse.FOPEN_NONSEEKABLE,
		OpenFlags: flags,
	}
	return fh, fuse.OK
}

// Directory handling
func (fsys *FS) OpenDir(name string, context *fuse.Context) (stream []fuse.DirEntry, code fuse.Status) {
	itemsRead := -1
	defer log.Trace(name, "")("items=%d, status=%v", &itemsRead, &code)
	dir, code := fsys.lookupDir(name)
	if !code.Ok() {
		return nil, code
	}
	items, err := dir.ReadDirAll()
	if err != nil {
		return nil, translateError(err)
	}
	for _, node := range items {
		dirent := fuse.DirEntry{
			Mode: uint32(node.Mode()),
			Name: node.Name(),
		}
		stream = append(stream, dirent)
	}
	itemsRead = len(stream)
	return stream, fuse.OK
}

// Symlinks.
// func (fsys *FS) Symlink(value string, linkName string, context *fuse.Context) (code fuse.Status) {
// 	fs.Debugf(linkName, "Symlink %q", value)
// 	return fuse.ENOSYS
// }

// func (fsys *FS) Readlink(name string, context *fuse.Context) (string, fuse.Status) {
// 	fs.Debugf(name, "Readlink")
// 	return "", fuse.ENOSYS
// }

// Statfs is called to obtain file system metadata.
// It should write that data to resp.
func (fsys *FS) StatFs(name string) (resp *fuse.StatfsOut) {
	defer log.Trace(name, "")("resp=%+v", &resp)
	resp = new(fuse.StatfsOut)
	const blockSize = 4096
	const fsBlocks = (1 << 50) / blockSize
	resp.Blocks = fsBlocks  // Total data blocks in file system.
	resp.Bfree = fsBlocks   // Free blocks in file system.
	resp.Bavail = fsBlocks  // Free blocks in file system if you're not root.
	resp.Files = 1E9        // Total files in file system.
	resp.Ffree = 1E9        // Free files in file system.
	resp.Bsize = blockSize  // Block size
	resp.NameLen = 255      // Maximum file name length?
	resp.Frsize = blockSize // Fragment size, smallest addressable data size in the file system.
	return resp
}

// Translate errors from mountlib
func translateError(err error) fuse.Status {
	if err == nil {
		return fuse.OK
	}
	switch errors.Cause(err) {
	case vfs.OK:
		return fuse.OK
	case vfs.ENOENT:
		return fuse.ENOENT
	case vfs.EEXIST:
		return fuse.Status(syscall.EEXIST)
	case vfs.EPERM:
		return fuse.EPERM
	case vfs.ECLOSED:
		return fuse.EBADF
	case vfs.ENOTEMPTY:
		return fuse.Status(syscall.ENOTEMPTY)
	case vfs.ESPIPE:
		return fuse.Status(syscall.ESPIPE)
	case vfs.EBADF:
		return fuse.EBADF
	case vfs.EROFS:
		return fuse.EROFS
	case vfs.ENOSYS:
		return fuse.ENOSYS
	case vfs.EINVAL:
		return fuse.EINVAL
	}
	fs.Errorf(nil, "IO error: %v", err)
	return fuse.EIO
}
