// Package vfs provides a virtual filing system layer over rclone's
// native objects.
//
// It attempts to behave in a similar way to Go's filing system
// manipulation code in the os package.  The same named function
// should behave in an identical fashion.  The objects also obey Go's
// standard interfaces.
//
// It also includes directory caching
package vfs

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
)

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
// defaults will be used.
func New(f fs.Fs, opt *Options) *VFS {
	fsDir := fs.NewDir("", time.Now())
	vfs := &VFS{
		f:   f,
		Opt: *opt,
	}

	// Mask the permissions with the umask
	vfs.Opt.DirPerms &= ^os.FileMode(vfs.Opt.Umask)
	vfs.Opt.FilePerms &= ^os.FileMode(vfs.Opt.Umask)

	// Create root directory
	vfs.root = newDir(vfs, f, nil, fsDir)

	// Start polling if required
	if vfs.Opt.PollInterval > 0 {
		vfs.PollChanges(vfs.Opt.PollInterval)
	}
	return vfs
}

// PollChanges will poll the remote every pollInterval for changes if the remote
// supports it. If a non-polling option is used, the given time interval can be
// ignored
func (vfs *VFS) PollChanges(pollInterval time.Duration) *VFS {
	doDirChangeNotify := vfs.f.Features().DirChangeNotify
	if doDirChangeNotify != nil {
		doDirChangeNotify(vfs.root.ForgetPath, pollInterval)
	}
	return vfs
}

// Root returns the root node
func (vfs *VFS) Root() (*Dir, error) {
	// fs.Debugf(vfs.f, "Root()")
	return vfs.root, nil
}

var inodeCount uint64

// NewInode creates a new unique inode number
func NewInode() (inode uint64) {
	return atomic.AddUint64(&inodeCount, 1)
}

// Lookup finds the Node by path starting from the root
func (vfs *VFS) Lookup(path string) (node Node, err error) {
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
		node, err = dir.Lookup(name)
		if err != nil {
			return nil, err
		}
	}
	return
}

// Statfs is called to obtain file system metadata.
// It should write that data to resp.
func (vfs *VFS) Statfs() error {
	/* FIXME
	const blockSize = 4096
	const fsBlocks = (1 << 50) / blockSize
	resp.Blocks = fsBlocks  // Total data blocks in file system.
	resp.Bfree = fsBlocks   // Free blocks in file system.
	resp.Bavail = fsBlocks  // Free blocks in file system if you're not root.
	resp.Files = 1E9        // Total files in file system.
	resp.Ffree = 1E9        // Free files in file system.
	resp.Bsize = blockSize  // Block size
	resp.Namelen = 255      // Maximum file name length?
	resp.Frsize = blockSize // Fragment size, smallest addressable data size in the file system.
	*/
	return nil
}
