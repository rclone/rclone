package mountlib

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
)

// Node represents either a *Dir or a *File
type Node interface {
	IsFile() bool
	Inode() uint64
}

var (
	_ Node = (*File)(nil)
	_ Node = (*Dir)(nil)
)

// Noder represents something which can return a node
type Noder interface {
	Node() Node
}

var (
	_ Noder = (*File)(nil)
	_ Noder = (*Dir)(nil)
	_ Noder = (*ReadFileHandle)(nil)
	_ Noder = (*WriteFileHandle)(nil)
)

// FS represents the top level filing system
type FS struct {
	f          fs.Fs
	root       *Dir
	noSeek     bool // don't allow seeking if set
	noChecksum bool // don't check checksums if set
}

// NewFS creates a new filing system and root directory
func NewFS(f fs.Fs) *FS {
	fsDir := &fs.Dir{
		Name: "",
		When: time.Now(),
	}
	fsys := &FS{
		f: f,
	}
	fsys.root = newDir(fsys, f, fsDir)
	return fsys
}

// NoSeek disables seeking of files
func (fsys *FS) NoSeek() *FS {
	fsys.noSeek = true
	return fsys
}

// NoChecksum disables checksum checking
func (fsys *FS) NoChecksum() *FS {
	fsys.noChecksum = true
	return fsys
}

// Root returns the root node
func (fsys *FS) Root() (*Dir, error) {
	fs.Debugf(fsys.f, "Root()")
	return fsys.root, nil
}

var inodeCount uint64

// NewInode creates a new unique inode number
func NewInode() (inode uint64) {
	return atomic.AddUint64(&inodeCount, 1)
}

// Lookup finds the Node by path starting from the root
func (fsys *FS) Lookup(path string) (node Node, err error) {
	node = fsys.root
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
func (fsys *FS) Statfs() error {
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
