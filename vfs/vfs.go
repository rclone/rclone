// Package vfs provides a virtual filing system layer over rclone's
// native objects.
//
// It attempts to behave in a similar way to Go's filing system
// manipulation code in the os package.  The same named function
// should behave in an identical fashion.  The objects also obey Go's
// standard interfaces.
//
// Note that paths don't start or end with /, so the root directory
// may be referred to as "".  However Stat strips slashes so you can
// use paths with slashes in.
//
// It also includes directory caching
package vfs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
)

// DefaultOpt is the default values uses for Opt
var DefaultOpt = Options{
	NoModTime:    false,
	NoChecksum:   false,
	NoSeek:       false,
	DirCacheTime: 5 * 60 * time.Second,
	PollInterval: time.Minute,
	ReadOnly:     false,
	Umask:        0,
	UID:          ^uint32(0), // these values instruct WinFSP-FUSE to use the current user
	GID:          ^uint32(0), // overriden for non windows in mount_unix.go
	DirPerms:     os.FileMode(0777) | os.ModeDir,
	FilePerms:    os.FileMode(0666),
}

// Node represents either a directory (*Dir) or a file (*File)
type Node interface {
	os.FileInfo
	IsFile() bool
	Inode() uint64
	SetModTime(modTime time.Time) error
	Fsync() error
	Remove() error
	RemoveAll() error
	DirEntry() fs.DirEntry
	VFS() *VFS
	Open(flags int) (Handle, error)
}

// Check interfaces
var (
	_ Node = (*File)(nil)
	_ Node = (*Dir)(nil)
)

// Nodes is a slice of Node
type Nodes []Node

// Sort functions
func (ns Nodes) Len() int           { return len(ns) }
func (ns Nodes) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns Nodes) Less(i, j int) bool { return ns[i].DirEntry().Remote() < ns[j].DirEntry().Remote() }

// Noder represents something which can return a node
type Noder interface {
	fmt.Stringer
	Node() Node
}

// Check interfaces
var (
	_ Noder = (*File)(nil)
	_ Noder = (*Dir)(nil)
	_ Noder = (*ReadFileHandle)(nil)
	_ Noder = (*WriteFileHandle)(nil)
	_ Noder = (*DirHandle)(nil)
)

// Handle is the interface statisified by open files or directories.
// It is the methods on *os.File.  Not all of them are supported.
type Handle interface {
	Chdir() error
	Chmod(mode os.FileMode) error
	Chown(uid, gid int) error
	Close() error
	Fd() uintptr
	Name() string
	Read(b []byte) (n int, err error)
	ReadAt(b []byte, off int64) (n int, err error)
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) (names []string, err error)
	Seek(offset int64, whence int) (ret int64, err error)
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(size int64) error
	Write(b []byte) (n int, err error)
	WriteAt(b []byte, off int64) (n int, err error)
	WriteString(s string) (n int, err error)
}

// baseHandle implements all the missing methods
type baseHandle struct{}

func (h baseHandle) Chdir() error                                         { return ENOSYS }
func (h baseHandle) Chmod(mode os.FileMode) error                         { return ENOSYS }
func (h baseHandle) Chown(uid, gid int) error                             { return ENOSYS }
func (h baseHandle) Close() error                                         { return ENOSYS }
func (h baseHandle) Fd() uintptr                                          { return 0 }
func (h baseHandle) Name() string                                         { return "" }
func (h baseHandle) Read(b []byte) (n int, err error)                     { return 0, ENOSYS }
func (h baseHandle) ReadAt(b []byte, off int64) (n int, err error)        { return 0, ENOSYS }
func (h baseHandle) Readdir(n int) ([]os.FileInfo, error)                 { return nil, ENOSYS }
func (h baseHandle) Readdirnames(n int) (names []string, err error)       { return nil, ENOSYS }
func (h baseHandle) Seek(offset int64, whence int) (ret int64, err error) { return 0, ENOSYS }
func (h baseHandle) Stat() (os.FileInfo, error)                           { return nil, ENOSYS }
func (h baseHandle) Sync() error                                          { return nil }
func (h baseHandle) Truncate(size int64) error                            { return ENOSYS }
func (h baseHandle) Write(b []byte) (n int, err error)                    { return 0, ENOSYS }
func (h baseHandle) WriteAt(b []byte, off int64) (n int, err error)       { return 0, ENOSYS }
func (h baseHandle) WriteString(s string) (n int, err error)              { return 0, ENOSYS }

// Check interfaces
var (
	_ Handle = (*baseHandle)(nil)
	_ Handle = (*ReadFileHandle)(nil)
	_ Handle = (*WriteFileHandle)(nil)
	_ Handle = (*DirHandle)(nil)
	_ Handle = (*os.File)(nil)
)

// VFS represents the top level filing system
type VFS struct {
	f    fs.Fs
	root *Dir
	Opt  Options
}

// Options is options for creating the vfs
type Options struct {
	NoSeek       bool          // don't allow seeking if set
	NoChecksum   bool          // don't check checksums if set
	ReadOnly     bool          // if set VFS is read only
	NoModTime    bool          // don't read mod times for files
	DirCacheTime time.Duration // how long to consider directory listing cache valid
	PollInterval time.Duration
	Umask        int
	UID          uint32
	GID          uint32
	DirPerms     os.FileMode
	FilePerms    os.FileMode
}

// New creates a new VFS and root directory.  If opt is nil, then
// DefaultOpt will be used
func New(f fs.Fs, opt *Options) *VFS {
	fsDir := fs.NewDir("", time.Now())
	vfs := &VFS{
		f: f,
	}

	// Make a copy of the options
	if opt != nil {
		vfs.Opt = *opt
	} else {
		vfs.Opt = DefaultOpt
	}

	// Mask the permissions with the umask
	vfs.Opt.DirPerms &= ^os.FileMode(vfs.Opt.Umask)
	vfs.Opt.FilePerms &= ^os.FileMode(vfs.Opt.Umask)

	// Make sure directories are returned as directories
	vfs.Opt.DirPerms |= os.ModeDir

	// Create root directory
	vfs.root = newDir(vfs, f, nil, fsDir)

	// Start polling if required
	if vfs.Opt.PollInterval > 0 {
		if do := vfs.f.Features().DirChangeNotify; do != nil {
			do(vfs.root.ForgetPath, vfs.Opt.PollInterval)
		}
	}
	return vfs
}

// Root returns the root node
func (vfs *VFS) Root() (*Dir, error) {
	// fs.Debugf(vfs.f, "Root()")
	return vfs.root, nil
}

var inodeCount uint64

// newInode creates a new unique inode number
func newInode() (inode uint64) {
	return atomic.AddUint64(&inodeCount, 1)
}

// Stat finds the Node by path starting from the root
//
// It is the equivalent of os.Stat - Node contains the os.FileInfo
// interface.
func (vfs *VFS) Stat(path string) (node Node, err error) {
	path = strings.Trim(path, "/")
	node = vfs.root
	for path != "" {
		i := strings.IndexRune(path, '/')
		var name string
		if i < 0 {
			name, path = path, ""
		} else {
			name, path = path[:i], path[i+1:]
		}
		if name == "" {
			continue
		}
		dir, ok := node.(*Dir)
		if !ok {
			// We need to look in a directory, but found a file
			return nil, ENOENT
		}
		node, err = dir.Stat(name)
		if err != nil {
			return nil, err
		}
	}
	return
}

// StatParent finds the parent directory and the leaf name of a path
func (vfs *VFS) StatParent(name string) (dir *Dir, leaf string, err error) {
	name = strings.Trim(name, "/")
	parent, leaf := path.Split(name)
	node, err := vfs.Stat(parent)
	if err != nil {
		return nil, "", err
	}
	if node.IsFile() {
		return nil, "", os.ErrExist
	}
	dir = node.(*Dir)
	return dir, leaf, nil
}

// OpenFile a file according to the flags and perm provided
func (vfs *VFS) OpenFile(name string, flags int, perm os.FileMode) (fd Handle, err error) {
	node, err := vfs.Stat(name)
	if err != nil {
		if err == ENOENT && flags&os.O_CREATE != 0 {
			dir, leaf, err := vfs.StatParent(name)
			if err != nil {
				return nil, err
			}
			_, fd, err = dir.Create(leaf)
			return fd, err
		}
		return nil, err
	}
	return node.Open(flags)
}

// Rename oldName to newName
func (vfs *VFS) Rename(oldName, newName string) error {
	// find the parent directories
	oldDir, oldLeaf, err := vfs.StatParent(oldName)
	if err != nil {
		return err
	}
	newDir, newLeaf, err := vfs.StatParent(newName)
	if err != nil {
		return err
	}
	err = oldDir.Rename(oldLeaf, newLeaf, newDir)
	if err != nil {
		return err
	}
	return nil
}
