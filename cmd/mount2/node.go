//go:build linux || (darwin && amd64)

package mount2

import (
	"context"
	"os"
	"path"
	"syscall"

	fusefs "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs"
)

// Node represents a directory or file
type Node struct {
	fusefs.Inode
	node vfs.Node
	fsys *FS
}

// Node types must be InodeEmbedders
var _ fusefs.InodeEmbedder = (*Node)(nil)

// newNode creates a new fusefs.Node from a vfs Node
func newNode(fsys *FS, vfsNode vfs.Node) (node *Node) {
	// Check the vfsNode to see if it has a fuse Node cached
	// We must return the same fuse nodes for vfs Nodes
	node, ok := vfsNode.Sys().(*Node)
	if ok {
		return node
	}
	node = &Node{
		node: vfsNode,
		fsys: fsys,
	}
	// Cache the node for later
	vfsNode.SetSys(node)
	return node
}

// String used for pretty printing.
func (n *Node) String() string {
	return n.node.Path()
}

// lookup a Node in a directory
func (n *Node) lookupVfsNodeInDir(leaf string) (vfsNode vfs.Node, errno syscall.Errno) {
	dir, ok := n.node.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	vfsNode, err := dir.Stat(leaf)
	return vfsNode, translateError(err)
}

// // lookup a Dir given a path
// func (n *Node) lookupDir(path string) (dir *vfs.Dir, code fuse.Status) {
// 	node, code := fsys.lookupVfsNodeInDir(path)
// 	if !code.Ok() {
// 		return nil, code
// 	}
// 	dir, ok := n.(*vfs.Dir)
// 	if !ok {
// 		return nil, fuse.ENOTDIR
// 	}
// 	return dir, fuse.OK
// }

// // lookup a parent Dir given a path returning the dir and the leaf
// func (n *Node) lookupParentDir(filePath string) (leaf string, dir *vfs.Dir, code fuse.Status) {
// 	parentDir, leaf := path.Split(filePath)
// 	dir, code = fsys.lookupDir(parentDir)
// 	return leaf, dir, code
// }

// Statfs implements statistics for the filesystem that holds this
// Inode. If not defined, the `out` argument will zeroed with an OK
// result.  This is because OSX filesystems must Statfs, or the mount
// will not work.
func (n *Node) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	defer log.Trace(n, "")("out=%+v", &out)
	const blockSize = 4096
	total, _, free := n.fsys.VFS.Statfs()
	out.Blocks = uint64(total) / blockSize // Total data blocks in file system.
	out.Bfree = uint64(free) / blockSize   // Free blocks in file system.
	out.Bavail = out.Bfree                 // Free blocks in file system if you're not root.
	out.Files = 1e9                        // Total files in file system.
	out.Ffree = 1e9                        // Free files in file system.
	out.Bsize = blockSize                  // Block size
	out.NameLen = 255                      // Maximum file name length?
	out.Frsize = blockSize                 // Fragment size, smallest addressable data size in the file system.
	mountlib.ClipBlocks(&out.Blocks)
	mountlib.ClipBlocks(&out.Bfree)
	mountlib.ClipBlocks(&out.Bavail)
	return 0
}

var _ = (fusefs.NodeStatfser)((*Node)(nil))

// Getattr reads attributes for an Inode. The library will ensure that
// Mode and Ino are set correctly. For files that are not opened with
// FOPEN_DIRECTIO, Size should be set so it can be read correctly.  If
// returning zeroed permissions, the default behavior is to change the
// mode of 0755 (directory) or 0644 (files). This can be switched off
// with the Options.NullPermissions setting. If blksize is unset, 4096
// is assumed, and the 'blocks' field is set accordingly.
func (n *Node) Getattr(ctx context.Context, f fusefs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	n.fsys.setAttrOut(n.node, out)
	return 0
}

var _ = (fusefs.NodeGetattrer)((*Node)(nil))

// Setattr sets attributes for an Inode.
func (n *Node) Setattr(ctx context.Context, f fusefs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) (errno syscall.Errno) {
	defer log.Trace(n, "in=%v", in)("out=%#v, errno=%v", &out, &errno)
	var err error
	n.fsys.setAttrOut(n.node, out)
	size, ok := in.GetSize()
	if ok {
		err = n.node.Truncate(int64(size))
		if err != nil {
			return translateError(err)
		}
		out.Attr.Size = size
	}
	mtime, ok := in.GetMTime()
	if ok {
		err = n.node.SetModTime(mtime)
		if err != nil {
			return translateError(err)
		}
		out.Attr.Mtime = uint64(mtime.Unix())
		out.Attr.Mtimensec = uint32(mtime.Nanosecond())
	}
	return 0
}

var _ = (fusefs.NodeSetattrer)((*Node)(nil))

// Open opens an Inode (of regular file type) for reading. It
// is optional but recommended to return a FileHandle.
func (n *Node) Open(ctx context.Context, flags uint32) (fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer log.Trace(n, "flags=%#o", flags)("errno=%v", &errno)
	// fuse flags are based off syscall flags as are os flags, so
	// should be compatible
	handle, err := n.node.Open(int(flags))
	if err != nil {
		return nil, 0, translateError(err)
	}
	// If size unknown then use direct io to read
	if entry := n.node.DirEntry(); entry != nil && entry.Size() < 0 {
		fuseFlags |= fuse.FOPEN_DIRECT_IO
	}
	if n.fsys.opt.DirectIO {
		fuseFlags |= fuse.FOPEN_DIRECT_IO
	}
	return newFileHandle(handle, n.fsys), fuseFlags, 0
}

var _ = (fusefs.NodeOpener)((*Node)(nil))

// Lookup should find a direct child of a directory by the child's name.  If
// the entry does not exist, it should return ENOENT and optionally
// set a NegativeTimeout in `out`. If it does exist, it should return
// attribute data in `out` and return the Inode for the child. A new
// inode can be created using `Inode.NewInode`. The new Inode will be
// added to the FS tree automatically if the return status is OK.
//
// If a directory does not implement NodeLookuper, the library looks
// for an existing child with the given name.
//
// The input to a Lookup is {parent directory, name string}.
//
// Lookup, if successful, must return an *Inode. Once the Inode is
// returned to the kernel, the kernel can issue further operations,
// such as Open or Getxattr on that node.
//
// A successful Lookup also returns an EntryOut. Among others, this
// contains file attributes (mode, size, mtime, etc.).
//
// FUSE supports other operations that modify the namespace. For
// example, the Symlink, Create, Mknod, Link methods all create new
// children in directories. Hence, they also return *Inode and must
// populate their fuse.EntryOut arguments.
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (inode *fusefs.Inode, errno syscall.Errno) {
	defer log.Trace(n, "name=%q", name)("inode=%v, attr=%v, errno=%v", &inode, &out, &errno)
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return nil, errno
	}
	newNode := newNode(n.fsys, vfsNode)

	// FIXME
	// out.SetEntryTimeout(dt time.Duration)
	// out.SetAttrTimeout(dt time.Duration)
	n.fsys.setEntryOut(vfsNode, out)

	return n.NewInode(ctx, newNode, fusefs.StableAttr{Mode: out.Attr.Mode}), 0
}

var _ = (fusefs.NodeLookuper)((*Node)(nil))

// Opendir opens a directory Inode for reading its
// contents. The actual reading is driven from Readdir, so
// this method is just for performing sanity/permission
// checks. The default is to return success.
func (n *Node) Opendir(ctx context.Context) syscall.Errno {
	if !n.node.IsDir() {
		return syscall.ENOTDIR
	}
	return 0
}

var _ = (fusefs.NodeOpendirer)((*Node)(nil))

type dirStream struct {
	nodes []os.FileInfo
	i     int
}

// HasNext indicates if there are further entries. HasNext
// might be called on already closed streams.
func (ds *dirStream) HasNext() bool {
	return ds.i < len(ds.nodes)+2
}

// Next retrieves the next entry. It is only called if HasNext
// has previously returned true.  The Errno return may be used to
// indicate I/O errors
func (ds *dirStream) Next() (de fuse.DirEntry, errno syscall.Errno) {
	// defer log.Trace(nil, "")("de=%+v, errno=%v", &de, &errno)
	if ds.i == 0 {
		ds.i++
		return fuse.DirEntry{
			Mode: fuse.S_IFDIR,
			Name: ".",
			Ino:  0, // FIXME
		}, 0
	} else if ds.i == 1 {
		ds.i++
		return fuse.DirEntry{
			Mode: fuse.S_IFDIR,
			Name: "..",
			Ino:  0, // FIXME
		}, 0
	}
	fi := ds.nodes[ds.i-2]
	de = fuse.DirEntry{
		// Mode is the file's mode. Only the high bits (e.g. S_IFDIR)
		// are considered.
		Mode: getMode(fi),

		// Name is the basename of the file in the directory.
		Name: path.Base(fi.Name()),

		// Ino is the inode number.
		Ino: 0, // FIXME
	}
	ds.i++
	return de, 0
}

// Close releases resources related to this directory
// stream.
func (ds *dirStream) Close() {
}

var _ fusefs.DirStream = (*dirStream)(nil)

// Readdir opens a stream of directory entries.
//
// Readdir essentially returns a list of strings, and it is allowed
// for Readdir to return different results from Lookup. For example,
// you can return nothing for Readdir ("ls my-fuse-mount" is empty),
// while still implementing Lookup ("ls my-fuse-mount/a-specific-file"
// shows a single file).
//
// If a directory does not implement NodeReaddirer, a list of
// currently known children from the tree is returned. This means that
// static in-memory file systems need not implement NodeReaddirer.
func (n *Node) Readdir(ctx context.Context) (ds fusefs.DirStream, errno syscall.Errno) {
	defer log.Trace(n, "")("ds=%v, errno=%v", &ds, &errno)
	if !n.node.IsDir() {
		return nil, syscall.ENOTDIR
	}
	fh, err := n.node.Open(os.O_RDONLY)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() {
		closeErr := fh.Close()
		if errno == 0 && closeErr != nil {
			errno = translateError(closeErr)
		}
	}()
	items, err := fh.Readdir(-1)
	if err != nil {
		return nil, translateError(err)
	}
	return &dirStream{
		nodes: items,
	}, 0
}

var _ = (fusefs.NodeReaddirer)((*Node)(nil))

// Mkdir is similar to Lookup, but must create a directory entry and Inode.
// Default is to return EROFS.
func (n *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (inode *fusefs.Inode, errno syscall.Errno) {
	defer log.Trace(name, "mode=0%o", mode)("inode=%v, errno=%v", &inode, &errno)
	dir, ok := n.node.(*vfs.Dir)
	if !ok {
		return nil, syscall.ENOTDIR
	}
	newDir, err := dir.Mkdir(name)
	if err != nil {
		return nil, translateError(err)
	}
	newNode := newNode(n.fsys, newDir)
	n.fsys.setEntryOut(newNode.node, out)
	newInode := n.NewInode(ctx, newNode, fusefs.StableAttr{Mode: out.Attr.Mode})
	return newInode, 0
}

var _ = (fusefs.NodeMkdirer)((*Node)(nil))

// Create is similar to Lookup, but should create a new
// child. It typically also returns a FileHandle as a
// reference for future reads/writes.
// Default is to return EROFS.
func (n *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (node *fusefs.Inode, fh fusefs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	defer log.Trace(n, "name=%q, flags=%#o, mode=%#o", name, flags, mode)("node=%v, fh=%v, flags=%#o, errno=%v", &node, &fh, &fuseFlags, &errno)
	dir, ok := n.node.(*vfs.Dir)
	if !ok {
		return nil, nil, 0, syscall.ENOTDIR
	}
	// translate the fuse flags to os flags
	osFlags := int(flags) | os.O_CREATE
	file, err := dir.Create(name, osFlags)
	if err != nil {
		return nil, nil, 0, translateError(err)
	}
	handle, err := file.Open(osFlags)
	if err != nil {
		return nil, nil, 0, translateError(err)
	}
	fh = newFileHandle(handle, n.fsys)
	// FIXME
	// fh = &fusefs.WithFlags{
	// 	File: fh,
	// 	//FuseFlags: fuse.FOPEN_NONSEEKABLE,
	// 	OpenFlags: flags,
	// }

	// Find the created node
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return nil, nil, 0, errno
	}
	n.fsys.setEntryOut(vfsNode, out)
	newNode := newNode(n.fsys, vfsNode)
	fs.Debugf(nil, "attr=%#v", out.Attr)
	newInode := n.NewInode(ctx, newNode, fusefs.StableAttr{Mode: out.Attr.Mode})
	return newInode, fh, 0, 0
}

var _ = (fusefs.NodeCreater)((*Node)(nil))

// Unlink should remove a child from this directory.  If the
// return status is OK, the Inode is removed as child in the
// FS tree automatically. Default is to return EROFS.
func (n *Node) Unlink(ctx context.Context, name string) (errno syscall.Errno) {
	defer log.Trace(n, "name=%q", name)("errno=%v", &errno)
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return errno
	}
	return translateError(vfsNode.Remove())
}

var _ = (fusefs.NodeUnlinker)((*Node)(nil))

// Rmdir is like Unlink but for directories.
// Default is to return EROFS.
func (n *Node) Rmdir(ctx context.Context, name string) (errno syscall.Errno) {
	defer log.Trace(n, "name=%q", name)("errno=%v", &errno)
	vfsNode, errno := n.lookupVfsNodeInDir(name)
	if errno != 0 {
		return errno
	}
	return translateError(vfsNode.Remove())
}

var _ = (fusefs.NodeRmdirer)((*Node)(nil))

// Rename should move a child from one directory to a different
// one. The change is effected in the FS tree if the return status is
// OK. Default is to return EROFS.
func (n *Node) Rename(ctx context.Context, oldName string, newParent fusefs.InodeEmbedder, newName string, flags uint32) (errno syscall.Errno) {
	defer log.Trace(n, "oldName=%q, newParent=%v, newName=%q", oldName, newParent, newName)("errno=%v", &errno)
	oldDir, ok := n.node.(*vfs.Dir)
	if !ok {
		return syscall.ENOTDIR
	}
	newParentNode, ok := newParent.(*Node)
	if !ok {
		fs.Errorf(n, "newParent was not a *Node")
		return syscall.EIO
	}
	newDir, ok := newParentNode.node.(*vfs.Dir)
	if !ok {
		return syscall.ENOTDIR
	}
	return translateError(oldDir.Rename(oldName, newName, newDir))
}

var _ = (fusefs.NodeRenamer)((*Node)(nil))

// Getxattr should read data for the given attribute into
// `dest` and return the number of bytes. If `dest` is too
// small, it should return ERANGE and the size of the attribute.
// If not defined, Getxattr will return ENOATTR.
func (n *Node) Getxattr(ctx context.Context, attr string, dest []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeGetxattrer = (*Node)(nil)

// Setxattr should store data for the given attribute.  See
// setxattr(2) for information about flags.
// If not defined, Setxattr will return ENOATTR.
func (n *Node) Setxattr(ctx context.Context, attr string, data []byte, flags uint32) syscall.Errno {
	return syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeSetxattrer = (*Node)(nil)

// Removexattr should delete the given attribute.
// If not defined, Removexattr will return ENOATTR.
func (n *Node) Removexattr(ctx context.Context, attr string) syscall.Errno {
	return syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeRemovexattrer = (*Node)(nil)

// Listxattr should read all attributes (null terminated) into
// `dest`. If the `dest` buffer is too small, it should return ERANGE
// and the correct size.  If not defined, return an empty list and
// success.
func (n *Node) Listxattr(ctx context.Context, dest []byte) (uint32, syscall.Errno) {
	return 0, syscall.ENOSYS // we never implement this
}

var _ fusefs.NodeListxattrer = (*Node)(nil)

var _ fusefs.NodeReadlinker = (*Node)(nil)

// Readlink read symbolic link target.
func (n *Node) Readlink(ctx context.Context) (ret []byte, err syscall.Errno) {
	defer log.Trace(n, "")("ret=%v, err=%v", &ret, &err)
	path := n.node.Path()
	s, serr := n.node.VFS().Readlink(path)
	return []byte(s), translateError(serr)
}

var _ fusefs.NodeSymlinker = (*Node)(nil)

// Symlink create symbolic link.
func (n *Node) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (node *fusefs.Inode, err syscall.Errno) {
	defer log.Trace(n, "name=%v, target=%v", name, target)("node=%v, err=%v", &node, &err)
	fullPath := path.Join(n.node.Path(), name)
	vfsNode, serr := n.node.VFS().CreateSymlink(target, fullPath)
	if serr != nil {
		return nil, translateError(serr)
	}

	n.fsys.setEntryOut(vfsNode, out)
	newNode := newNode(n.fsys, vfsNode)
	newInode := n.NewInode(ctx, newNode, fusefs.StableAttr{Mode: out.Attr.Mode})

	return newInode, 0
}
