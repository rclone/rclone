// FUSE main Fs

//go:build linux || (darwin && amd64)
// +build linux darwin,amd64

package mount2

import (
	"os"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
)

// FS represents the top level filing system
type FS struct {
	VFS *vfs.VFS
	f   fs.Fs
	opt *mountlib.Options
}

// NewFS creates a pathfs.FileSystem from the fs.Fs passed in
func NewFS(VFS *vfs.VFS, opt *mountlib.Options) *FS {
	fsys := &FS{
		VFS: VFS,
		f:   VFS.Fs(),
		opt: opt,
	}
	return fsys
}

// Root returns the root node
func (f *FS) Root() (node *Node, err error) {
	defer log.Trace("", "")("node=%+v, err=%v", &node, &err)
	root, err := f.VFS.Root()
	if err != nil {
		return nil, err
	}
	return newNode(f, root), nil
}

// SetDebug if called, provide debug output through the log package.
func (f *FS) SetDebug(debug bool) {
	fs.Debugf(f.f, "SetDebug %v", debug)
}

// get the Mode from a vfs Node
func getMode(node os.FileInfo) uint32 {
	Mode := node.Mode().Perm()
	if node.IsDir() {
		Mode |= fuse.S_IFDIR
	} else {
		Mode |= fuse.S_IFREG
	}
	return uint32(Mode)
}

// fill in attr from node
func setAttr(node vfs.Node, attr *fuse.Attr) {
	Size := uint64(node.Size())
	const BlockSize = 512
	Blocks := (Size + BlockSize - 1) / BlockSize
	modTime := node.ModTime()
	// set attributes
	vfs := node.VFS()
	attr.Owner.Gid = vfs.Opt.GID
	attr.Owner.Uid = vfs.Opt.UID
	attr.Mode = getMode(node)
	attr.Size = Size
	attr.Nlink = 1
	attr.Blocks = Blocks
	// attr.Blksize = BlockSize // not supported in freebsd/darwin, defaults to 4k if not set
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

// fill in AttrOut from node
func (f *FS) setAttrOut(node vfs.Node, out *fuse.AttrOut) {
	setAttr(node, &out.Attr)
	out.SetTimeout(f.opt.AttrTimeout)
}

// fill in EntryOut from node
func (f *FS) setEntryOut(node vfs.Node, out *fuse.EntryOut) {
	setAttr(node, &out.Attr)
	out.SetEntryTimeout(f.opt.AttrTimeout)
	out.SetAttrTimeout(f.opt.AttrTimeout)
}
