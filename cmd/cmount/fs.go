//go:build cmount && ((linux && cgo) || (darwin && cgo) || (freebsd && cgo) || windows)
// +build cmount
// +build linux,cgo darwin,cgo freebsd,cgo windows

package cmount

import (
	"io"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
	"github.com/winfsp/cgofuse/fuse"
)

const fhUnset = ^uint64(0)

// FS represents the top level filing system
type FS struct {
	VFS       *vfs.VFS
	f         fs.Fs
	ready     chan (struct{})
	mu        sync.Mutex // to protect the below
	handles   []vfs.Handle
	destroyed int32 // read/write with sync/atomic
}

// NewFS makes a new FS
func NewFS(VFS *vfs.VFS) *FS {
	fsys := &FS{
		VFS:   VFS,
		f:     VFS.Fs(),
		ready: make(chan (struct{})),
	}
	return fsys
}

// Open a handle returning an integer file handle
func (fsys *FS) openHandle(handle vfs.Handle) (fh uint64) {
	fsys.mu.Lock()
	defer fsys.mu.Unlock()
	var i int
	var oldHandle vfs.Handle
	for i, oldHandle = range fsys.handles {
		if oldHandle == nil {
			fsys.handles[i] = handle
			goto found
		}
	}
	fsys.handles = append(fsys.handles, handle)
	i = len(fsys.handles) - 1
found:
	return uint64(i)
}

// get the handle for fh, call with the lock held
func (fsys *FS) _getHandle(fh uint64) (i int, handle vfs.Handle, errc int) {
	if fh > uint64(len(fsys.handles)) {
		fs.Debugf(nil, "Bad file handle: too big: 0x%X", fh)
		return i, nil, -fuse.EBADF
	}
	i = int(fh)
	handle = fsys.handles[i]
	if handle == nil {
		fs.Debugf(nil, "Bad file handle: nil handle: 0x%X", fh)
		return i, nil, -fuse.EBADF
	}
	return i, handle, 0
}

// Get the handle for the file handle
func (fsys *FS) getHandle(fh uint64) (handle vfs.Handle, errc int) {
	fsys.mu.Lock()
	_, handle, errc = fsys._getHandle(fh)
	fsys.mu.Unlock()
	return
}

// Close the handle
func (fsys *FS) closeHandle(fh uint64) (errc int) {
	fsys.mu.Lock()
	i, _, errc := fsys._getHandle(fh)
	if errc == 0 {
		fsys.handles[i] = nil
	}
	fsys.mu.Unlock()
	return
}

// lookup a Node given a path
func (fsys *FS) lookupNode(path string) (node vfs.Node, errc int) {
	node, err := fsys.VFS.Stat(path)
	return node, translateError(err)
}

// lookup a Dir given a path
func (fsys *FS) lookupDir(path string) (dir *vfs.Dir, errc int) {
	node, errc := fsys.lookupNode(path)
	if errc != 0 {
		return nil, errc
	}
	dir, ok := node.(*vfs.Dir)
	if !ok {
		return nil, -fuse.ENOTDIR
	}
	return dir, 0
}

// lookup a parent Dir given a path returning the dir and the leaf
func (fsys *FS) lookupParentDir(filePath string) (leaf string, dir *vfs.Dir, errc int) {
	parentDir, leaf := path.Split(filePath)
	dir, errc = fsys.lookupDir(parentDir)
	return leaf, dir, errc
}

// lookup a File given a path
func (fsys *FS) lookupFile(path string) (file *vfs.File, errc int) {
	node, errc := fsys.lookupNode(path)
	if errc != 0 {
		return nil, errc
	}
	file, ok := node.(*vfs.File)
	if !ok {
		return nil, -fuse.EISDIR
	}
	return file, 0
}

// get a node and handle from the path or from the fh if not fhUnset
//
// handle may be nil
func (fsys *FS) getNode(path string, fh uint64) (node vfs.Node, handle vfs.Handle, errc int) {
	if fh == fhUnset {
		node, errc = fsys.lookupNode(path)
	} else {
		handle, errc = fsys.getHandle(fh)
		if errc == 0 {
			node = handle.Node()
		}
	}
	return
}

// stat fills up the stat block for Node
func (fsys *FS) stat(node vfs.Node, stat *fuse.Stat_t) (errc int) {
	Size := uint64(node.Size())
	Blocks := (Size + 511) / 512
	modTime := node.ModTime()
	Mode := node.Mode().Perm()
	if node.IsDir() {
		Mode |= fuse.S_IFDIR
	} else {
		Mode |= fuse.S_IFREG
	}
	//stat.Dev = 1
	stat.Ino = node.Inode() // FIXME do we need to set the inode number?
	stat.Mode = uint32(Mode)
	stat.Nlink = 1
	stat.Uid = fsys.VFS.Opt.UID
	stat.Gid = fsys.VFS.Opt.GID
	//stat.Rdev
	stat.Size = int64(Size)
	t := fuse.NewTimespec(modTime)
	stat.Atim = t
	stat.Mtim = t
	stat.Ctim = t
	stat.Blksize = 512
	stat.Blocks = int64(Blocks)
	stat.Birthtim = t
	// fs.Debugf(nil, "stat = %+v", *stat)
	return 0
}

// Init is called after the filesystem is ready
func (fsys *FS) Init() {
	defer log.Trace(fsys.f, "")("")
	close(fsys.ready)
}

// Destroy is called when it is unmounted (note that depending on how
// the file system is terminated the file system may not receive the
// Destroy call).
func (fsys *FS) Destroy() {
	defer log.Trace(fsys.f, "")("")
	atomic.StoreInt32(&fsys.destroyed, 1)
}

// Getattr reads the attributes for path
func (fsys *FS) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	defer log.Trace(path, "fh=0x%X", fh)("errc=%v", &errc)
	node, _, errc := fsys.getNode(path, fh)
	if errc == 0 {
		errc = fsys.stat(node, stat)
	}
	return
}

// Opendir opens path as a directory
func (fsys *FS) Opendir(path string) (errc int, fh uint64) {
	defer log.Trace(path, "")("errc=%d, fh=0x%X", &errc, &fh)
	handle, err := fsys.VFS.OpenFile(path, os.O_RDONLY, 0777)
	if err != nil {
		return translateError(err), fhUnset
	}
	return 0, fsys.openHandle(handle)
}

// Readdir reads the directory at dirPath
func (fsys *FS) Readdir(dirPath string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) (errc int) {
	itemsRead := -1
	defer log.Trace(dirPath, "ofst=%d, fh=0x%X", ofst, fh)("items=%d, errc=%d", &itemsRead, &errc)

	dir, errc := fsys.lookupDir(dirPath)
	if errc != 0 {
		return errc
	}

	// We can't seek in directories and FUSE should know that so
	// return an error if ofst is ever set.
	if ofst > 0 {
		return -fuse.ESPIPE
	}

	nodes, err := dir.ReadDirAll()
	if err != nil {
		return translateError(err)
	}

	// Optionally, create a struct stat that describes the file as
	// for getattr (but FUSE only looks at st_ino and the
	// file-type bits of st_mode).
	//
	// We have called host.SetCapReaddirPlus() so WinFsp will
	// use the full stat information - a Useful optimization on
	// Windows.
	//
	// NB we are using the first mode for readdir: The readdir
	// implementation ignores the offset parameter, and passes
	// zero to the filler function's offset. The filler function
	// will not return '1' (unless an error happens), so the whole
	// directory is read in a single readdir operation.
	fill(".", nil, 0)
	fill("..", nil, 0)
	for _, node := range nodes {
		name := node.Name()
		if len(name) > mountlib.MaxLeafSize {
			fs.Errorf(dirPath, "Name too long (%d bytes) for FUSE, skipping: %s", len(name), name)
			continue
		}
		// We have called host.SetCapReaddirPlus() so supply the stat information
		// It is very cheap at this point so supply it regardless of OS capabilities
		var stat fuse.Stat_t
		_ = fsys.stat(node, &stat) // not capable of returning an error
		fill(name, &stat, 0)
	}
	itemsRead = len(nodes)
	return 0
}

// Releasedir finished reading the directory
func (fsys *FS) Releasedir(path string, fh uint64) (errc int) {
	defer log.Trace(path, "fh=0x%X", fh)("errc=%d", &errc)
	return fsys.closeHandle(fh)
}

// Statfs reads overall stats on the filesystem
func (fsys *FS) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	defer log.Trace(path, "")("stat=%+v, errc=%d", stat, &errc)
	const blockSize = 4096
	total, _, free := fsys.VFS.Statfs()
	stat.Blocks = uint64(total) / blockSize // Total data blocks in file system.
	stat.Bfree = uint64(free) / blockSize   // Free blocks in file system.
	stat.Bavail = stat.Bfree                // Free blocks in file system if you're not root.
	stat.Files = 1e9                        // Total files in file system.
	stat.Ffree = 1e9                        // Free files in file system.
	stat.Bsize = blockSize                  // Block size
	stat.Namemax = 255                      // Maximum file name length?
	stat.Frsize = blockSize                 // Fragment size, smallest addressable data size in the file system.
	mountlib.ClipBlocks(&stat.Blocks)
	mountlib.ClipBlocks(&stat.Bfree)
	mountlib.ClipBlocks(&stat.Bavail)
	return 0
}

// OpenEx opens a file
func (fsys *FS) OpenEx(path string, fi *fuse.FileInfo_t) (errc int) {
	defer log.Trace(path, "flags=0x%X", fi.Flags)("errc=%d, fh=0x%X", &errc, &fi.Fh)
	fi.Fh = fhUnset

	// translate the fuse flags to os flags
	flags := translateOpenFlags(fi.Flags)
	handle, err := fsys.VFS.OpenFile(path, flags, 0777)
	if err != nil {
		return translateError(err)
	}

	// If size unknown then use direct io to read
	if entry := handle.Node().DirEntry(); entry != nil && entry.Size() < 0 {
		fi.DirectIo = true
	}

	fi.Fh = fsys.openHandle(handle)
	return 0
}

// Open opens a file
func (fsys *FS) Open(path string, flags int) (errc int, fh uint64) {
	var fi = fuse.FileInfo_t{
		Flags: flags,
	}
	errc = fsys.OpenEx(path, &fi)
	return errc, fi.Fh
}

// CreateEx creates and opens a file.
func (fsys *FS) CreateEx(filePath string, mode uint32, fi *fuse.FileInfo_t) (errc int) {
	defer log.Trace(filePath, "flags=0x%X, mode=0%o", fi.Flags, mode)("errc=%d, fh=0x%X", &errc, &fi.Fh)
	fi.Fh = fhUnset
	leaf, parentDir, errc := fsys.lookupParentDir(filePath)
	if errc != 0 {
		return errc
	}
	file, err := parentDir.Create(leaf, fi.Flags)
	if err != nil {
		return translateError(err)
	}
	// translate the fuse flags to os flags
	flags := translateOpenFlags(fi.Flags) | os.O_CREATE
	handle, err := file.Open(flags)
	if err != nil {
		return translateError(err)
	}
	fi.Fh = fsys.openHandle(handle)
	return 0
}

// Create creates and opens a file.
func (fsys *FS) Create(filePath string, flags int, mode uint32) (errc int, fh uint64) {
	var fi = fuse.FileInfo_t{
		Flags: flags,
	}
	errc = fsys.CreateEx(filePath, mode, &fi)
	return errc, fi.Fh
}

// Truncate truncates a file to size
func (fsys *FS) Truncate(path string, size int64, fh uint64) (errc int) {
	defer log.Trace(path, "size=%d, fh=0x%X", size, fh)("errc=%d", &errc)
	node, handle, errc := fsys.getNode(path, fh)
	if errc != 0 {
		return errc
	}
	var err error
	if handle != nil {
		err = handle.Truncate(size)
	} else {
		err = node.Truncate(size)
	}
	if err != nil {
		return translateError(err)
	}
	return 0
}

// Read data from file handle
func (fsys *FS) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer log.Trace(path, "ofst=%d, fh=0x%X", ofst, fh)("n=%d", &n)
	handle, errc := fsys.getHandle(fh)
	if errc != 0 {
		return errc
	}
	n, err := handle.ReadAt(buff, ofst)
	if err == io.EOF {
	} else if err != nil {
		return translateError(err)
	}
	return n
}

// Write data to file handle
func (fsys *FS) Write(path string, buff []byte, ofst int64, fh uint64) (n int) {
	defer log.Trace(path, "ofst=%d, fh=0x%X", ofst, fh)("n=%d", &n)
	handle, errc := fsys.getHandle(fh)
	if errc != 0 {
		return errc
	}
	n, err := handle.WriteAt(buff, ofst)
	if err != nil {
		return translateError(err)
	}
	return n
}

// Flush flushes an open file descriptor or path
func (fsys *FS) Flush(path string, fh uint64) (errc int) {
	defer log.Trace(path, "fh=0x%X", fh)("errc=%d", &errc)
	handle, errc := fsys.getHandle(fh)
	if errc != 0 {
		return errc
	}
	return translateError(handle.Flush())
}

// Release closes the file if still open
func (fsys *FS) Release(path string, fh uint64) (errc int) {
	defer log.Trace(path, "fh=0x%X", fh)("errc=%d", &errc)
	handle, errc := fsys.getHandle(fh)
	if errc != 0 {
		return errc
	}
	_ = fsys.closeHandle(fh)
	return translateError(handle.Release())
}

// Unlink removes a file.
func (fsys *FS) Unlink(filePath string) (errc int) {
	defer log.Trace(filePath, "")("errc=%d", &errc)
	leaf, parentDir, errc := fsys.lookupParentDir(filePath)
	if errc != 0 {
		return errc
	}
	return translateError(parentDir.RemoveName(leaf))
}

// Mkdir creates a directory.
func (fsys *FS) Mkdir(dirPath string, mode uint32) (errc int) {
	defer log.Trace(dirPath, "mode=0%o", mode)("errc=%d", &errc)
	leaf, parentDir, errc := fsys.lookupParentDir(dirPath)
	if errc != 0 {
		return errc
	}
	_, err := parentDir.Mkdir(leaf)
	return translateError(err)
}

// Rmdir removes a directory
func (fsys *FS) Rmdir(dirPath string) (errc int) {
	defer log.Trace(dirPath, "")("errc=%d", &errc)
	leaf, parentDir, errc := fsys.lookupParentDir(dirPath)
	if errc != 0 {
		return errc
	}
	return translateError(parentDir.RemoveName(leaf))
}

// Rename renames a file.
func (fsys *FS) Rename(oldPath string, newPath string) (errc int) {
	defer log.Trace(oldPath, "newPath=%q", newPath)("errc=%d", &errc)
	return translateError(fsys.VFS.Rename(oldPath, newPath))
}

// Windows sometimes seems to send times that are the epoch which is
// 1601-01-01 +/- timezone so filter out times that are earlier than
// this.
var invalidDateCutoff = time.Date(1601, 1, 2, 0, 0, 0, 0, time.UTC)

// Utimens changes the access and modification times of a file.
func (fsys *FS) Utimens(path string, tmsp []fuse.Timespec) (errc int) {
	defer log.Trace(path, "tmsp=%+v", tmsp)("errc=%d", &errc)
	node, errc := fsys.lookupNode(path)
	if errc != 0 {
		return errc
	}
	if tmsp == nil || len(tmsp) < 2 {
		fs.Debugf(path, "Utimens: Not setting time as timespec isn't complete: %v", tmsp)
		return 0
	}
	t := tmsp[1].Time()
	if t.Before(invalidDateCutoff) {
		fs.Debugf(path, "Utimens: Not setting out of range time: %v", t)
		return 0
	}
	fs.Debugf(path, "Utimens: SetModTime: %v", t)
	return translateError(node.SetModTime(t))
}

// Mknod creates a file node.
func (fsys *FS) Mknod(path string, mode uint32, dev uint64) (errc int) {
	defer log.Trace(path, "mode=0x%X, dev=0x%X", mode, dev)("errc=%d", &errc)
	return -fuse.ENOSYS
}

// Fsync synchronizes file contents.
func (fsys *FS) Fsync(path string, datasync bool, fh uint64) (errc int) {
	defer log.Trace(path, "datasync=%v, fh=0x%X", datasync, fh)("errc=%d", &errc)
	// This is a no-op for rclone
	return 0
}

// Link creates a hard link to a file.
func (fsys *FS) Link(oldpath string, newpath string) (errc int) {
	defer log.Trace(oldpath, "newpath=%q", newpath)("errc=%d", &errc)
	return -fuse.ENOSYS
}

// Symlink creates a symbolic link.
func (fsys *FS) Symlink(target string, newpath string) (errc int) {
	defer log.Trace(target, "newpath=%q", newpath)("errc=%d", &errc)
	return -fuse.ENOSYS
}

// Readlink reads the target of a symbolic link.
func (fsys *FS) Readlink(path string) (errc int, linkPath string) {
	defer log.Trace(path, "")("linkPath=%q, errc=%d", &linkPath, &errc)
	return -fuse.ENOSYS, ""
}

// Chmod changes the permission bits of a file.
func (fsys *FS) Chmod(path string, mode uint32) (errc int) {
	defer log.Trace(path, "mode=0%o", mode)("errc=%d", &errc)
	// This is a no-op for rclone
	return 0
}

// Chown changes the owner and group of a file.
func (fsys *FS) Chown(path string, uid uint32, gid uint32) (errc int) {
	defer log.Trace(path, "uid=%d, gid=%d", uid, gid)("errc=%d", &errc)
	// This is a no-op for rclone
	return 0
}

// Access checks file access permissions.
func (fsys *FS) Access(path string, mask uint32) (errc int) {
	defer log.Trace(path, "mask=0%o", mask)("errc=%d", &errc)
	// This is a no-op for rclone
	return 0
}

// Fsyncdir synchronizes directory contents.
func (fsys *FS) Fsyncdir(path string, datasync bool, fh uint64) (errc int) {
	defer log.Trace(path, "datasync=%v, fh=0x%X", datasync, fh)("errc=%d", &errc)
	// This is a no-op for rclone
	return 0
}

// Setxattr sets extended attributes.
func (fsys *FS) Setxattr(path string, name string, value []byte, flags int) (errc int) {
	return -fuse.ENOSYS
}

// Getxattr gets extended attributes.
func (fsys *FS) Getxattr(path string, name string) (errc int, value []byte) {
	return -fuse.ENOSYS, nil
}

// Removexattr removes extended attributes.
func (fsys *FS) Removexattr(path string, name string) (errc int) {
	return -fuse.ENOSYS
}

// Listxattr lists extended attributes.
func (fsys *FS) Listxattr(path string, fill func(name string) bool) (errc int) {
	return -fuse.ENOSYS
}

// Translate errors from mountlib
func translateError(err error) (errc int) {
	if err == nil {
		return 0
	}
	_, uErr := fserrors.Cause(err)
	switch uErr {
	case vfs.OK:
		return 0
	case vfs.ENOENT, fs.ErrorDirNotFound, fs.ErrorObjectNotFound:
		return -fuse.ENOENT
	case vfs.EEXIST, fs.ErrorDirExists:
		return -fuse.EEXIST
	case vfs.EPERM, fs.ErrorPermissionDenied:
		return -fuse.EPERM
	case vfs.ECLOSED:
		return -fuse.EBADF
	case vfs.ENOTEMPTY:
		return -fuse.ENOTEMPTY
	case vfs.ESPIPE:
		return -fuse.ESPIPE
	case vfs.EBADF:
		return -fuse.EBADF
	case vfs.EROFS:
		return -fuse.EROFS
	case vfs.ENOSYS, fs.ErrorNotImplemented:
		return -fuse.ENOSYS
	case vfs.EINVAL:
		return -fuse.EINVAL
	}
	fs.Errorf(nil, "IO error: %v", err)
	return -fuse.EIO
}

// Translate Open Flags from FUSE to os (as used in the vfs layer)
func translateOpenFlags(inFlags int) (outFlags int) {
	switch inFlags & fuse.O_ACCMODE {
	case fuse.O_RDONLY:
		outFlags = os.O_RDONLY
	case fuse.O_WRONLY:
		outFlags = os.O_WRONLY
	case fuse.O_RDWR:
		outFlags = os.O_RDWR
	}
	if inFlags&fuse.O_APPEND != 0 {
		outFlags |= os.O_APPEND
	}
	if inFlags&fuse.O_CREAT != 0 {
		outFlags |= os.O_CREATE
	}
	if inFlags&fuse.O_EXCL != 0 {
		outFlags |= os.O_EXCL
	}
	if inFlags&fuse.O_TRUNC != 0 {
		outFlags |= os.O_TRUNC
	}
	// NB O_SYNC isn't defined by fuse
	return outFlags
}

// Make sure interfaces are satisfied
var (
	_ fuse.FileSystemInterface = (*FS)(nil)
	_ fuse.FileSystemOpenEx    = (*FS)(nil)
	//_ fuse.FileSystemChflags    = (*FS)(nil)
	//_ fuse.FileSystemSetcrtime  = (*FS)(nil)
	//_ fuse.FileSystemSetchgtime = (*FS)(nil)
)
