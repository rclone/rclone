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
//
// The vfs package returns Error values to signal precisely which
// error conditions have ocurred.  It may also return general errors
// it receives.  It tries to use os Error values (eg os.ErrExist)
// where possible.
package vfs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/log"
	"golang.org/x/net/context" // switch to "context" when we stop supporting go1.6
)

// DefaultOpt is the default values uses for Opt
var DefaultOpt = Options{
	NoModTime:         false,
	NoChecksum:        false,
	NoSeek:            false,
	DirCacheTime:      5 * 60 * time.Second,
	PollInterval:      time.Minute,
	ReadOnly:          false,
	Umask:             0,
	UID:               ^uint32(0), // these values instruct WinFSP-FUSE to use the current user
	GID:               ^uint32(0), // overriden for non windows in mount_unix.go
	DirPerms:          os.FileMode(0777) | os.ModeDir,
	FilePerms:         os.FileMode(0666),
	CacheMode:         CacheModeOff,
	CacheMaxAge:       3600 * time.Second,
	CachePollInterval: 60 * time.Second,
}

// Node represents either a directory (*Dir) or a file (*File)
type Node interface {
	os.FileInfo
	IsFile() bool
	Inode() uint64
	SetModTime(modTime time.Time) error
	Sync() error
	Remove() error
	RemoveAll() error
	DirEntry() fs.DirEntry
	VFS() *VFS
	Open(flags int) (Handle, error)
	Truncate(size int64) error
	Path() string
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
func (ns Nodes) Less(i, j int) bool { return ns[i].Path() < ns[j].Path() }

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
	_ Noder = (*RWFileHandle)(nil)
	_ Noder = (*DirHandle)(nil)
)

// OsFiler is the methods on *os.File
type OsFiler interface {
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

// Handle is the interface statisified by open files or directories.
// It is the methods on *os.File, plus a few more useful for FUSE
// filingsystems.  Not all of them are supported.
type Handle interface {
	OsFiler
	// Additional methods useful for FUSE filesystems
	Flush() error
	Release() error
	Node() Node
	//	Size() int64
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
func (h baseHandle) Flush() (err error)                                   { return ENOSYS }
func (h baseHandle) Release() (err error)                                 { return ENOSYS }
func (h baseHandle) Node() Node                                           { return nil }

//func (h baseHandle) Size() int64                                          { return 0 }

// Check interfaces
var (
	_ OsFiler = (*os.File)(nil)
	_ Handle  = (*baseHandle)(nil)
	_ Handle  = (*ReadFileHandle)(nil)
	_ Handle  = (*WriteFileHandle)(nil)
	_ Handle  = (*DirHandle)(nil)
)

// VFS represents the top level filing system
type VFS struct {
	f      fs.Fs
	root   *Dir
	Opt    Options
	cache  *cache
	cancel context.CancelFunc
}

// Options is options for creating the vfs
type Options struct {
	NoSeek            bool          // don't allow seeking if set
	NoChecksum        bool          // don't check checksums if set
	ReadOnly          bool          // if set VFS is read only
	NoModTime         bool          // don't read mod times for files
	DirCacheTime      time.Duration // how long to consider directory listing cache valid
	PollInterval      time.Duration
	Umask             int
	UID               uint32
	GID               uint32
	DirPerms          os.FileMode
	FilePerms         os.FileMode
	CacheMode         CacheMode
	CacheMaxAge       time.Duration
	CachePollInterval time.Duration
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
		if do := vfs.f.Features().ChangeNotify; do != nil {
			do(vfs.root.ForgetPath, vfs.Opt.PollInterval)
		} else {
			fs.Infof(f, "poll-interval is not supported by this remote")
		}
	}

	// Create the cache
	ctx, cancel := context.WithCancel(context.Background())
	vfs.cancel = cancel
	cache, err := newCache(ctx, f, &vfs.Opt) // FIXME pass on context or get from Opt?
	if err != nil {
		// FIXME
		panic(fmt.Sprintf("failed to create local cache: %v", err))
	}
	vfs.cache = cache

	// add the remote control
	vfs.addRC()
	return vfs
}

// Shutdown stops any background go-routines
func (vfs *VFS) Shutdown() {
	if vfs.cancel != nil {
		vfs.cancel()
		vfs.cancel = nil
	}
}

// CleanUp deletes the contents of the on disk cache
func (vfs *VFS) CleanUp() error {
	return vfs.cache.cleanUp()
}

// FlushDirCache empties the directory cache
func (vfs *VFS) FlushDirCache() {
	vfs.root.ForgetAll()
}

// WaitForWriters sleeps until all writers have finished or
// time.Duration has elapsed
func (vfs *VFS) WaitForWriters(timeout time.Duration) {
	defer log.Trace(nil, "timeout=%v", timeout)("")
	const tickTime = 1 * time.Second
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTimer(tickTime)
	defer tick.Stop()
	tick.Stop()
	for {
		writers := 0
		vfs.root.walk("", func(d *Dir) {
			fs.Debugf(d.path, "Looking for writers")
			// NB d.mu is held by walk() here
			for leaf, item := range d.items {
				fs.Debugf(leaf, "reading active writers")
				if file, ok := item.(*File); ok {
					n := file.activeWriters()
					if n != 0 {
						fs.Debugf(file, "active writers %d", n)
					}
					writers += n
				}
			}
		})
		if writers == 0 {
			return
		}
		fs.Debugf(nil, "Still %d writers active, waiting %v", writers, tickTime)
		tick.Reset(tickTime)
		select {
		case <-tick.C:
			break
		case <-deadline.C:
			fs.Errorf(nil, "Exiting even though %d writers are active after %v", writers, timeout)
			return
		}
	}
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

// decodeOpenFlags returns a string representing the open flags
func decodeOpenFlags(flags int) string {
	var out []string
	rdwrMode := flags & accessModeMask
	switch rdwrMode {
	case os.O_RDONLY:
		out = append(out, "O_RDONLY")
	case os.O_WRONLY:
		out = append(out, "O_WRONLY")
	case os.O_RDWR:
		out = append(out, "O_RDWR")
	default:
		out = append(out, fmt.Sprintf("0x%X", rdwrMode))
	}
	if flags&os.O_APPEND != 0 {
		out = append(out, "O_APPEND")
	}
	if flags&os.O_CREATE != 0 {
		out = append(out, "O_CREATE")
	}
	if flags&os.O_EXCL != 0 {
		out = append(out, "O_EXCL")
	}
	if flags&os.O_SYNC != 0 {
		out = append(out, "O_SYNC")
	}
	if flags&os.O_TRUNC != 0 {
		out = append(out, "O_TRUNC")
	}
	flags &^= accessModeMask | os.O_APPEND | os.O_CREATE | os.O_EXCL | os.O_SYNC | os.O_TRUNC
	if flags != 0 {
		out = append(out, fmt.Sprintf("0x%X", flags))
	}
	return strings.Join(out, "|")
}

// OpenFile a file according to the flags and perm provided
func (vfs *VFS) OpenFile(name string, flags int, perm os.FileMode) (fd Handle, err error) {
	defer log.Trace(name, "flags=%s, perm=%v", decodeOpenFlags(flags), perm)("fd=%v, err=%v", &fd, &err)

	// http://pubs.opengroup.org/onlinepubs/7908799/xsh/open.html
	// The result of using O_TRUNC with O_RDONLY is undefined.
	// Linux seems to truncate the file, but we prefer to return EINVAL
	if flags&accessModeMask == os.O_RDONLY && flags&os.O_TRUNC != 0 {
		return nil, EINVAL
	}

	node, err := vfs.Stat(name)
	if err != nil {
		if err != ENOENT || flags&os.O_CREATE == 0 {
			return nil, err
		}
		// If not found and O_CREATE then create the file
		dir, leaf, err := vfs.StatParent(name)
		if err != nil {
			return nil, err
		}
		node, err = dir.Create(leaf, flags)
		if err != nil {
			return nil, err
		}
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
