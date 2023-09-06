// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"context"
	"log"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/internal"
)

func errnoToStatus(errno syscall.Errno) fuse.Status {
	return fuse.Status(errno)
}

type fileEntry struct {
	file FileHandle

	// index into Inode.openFiles
	nodeIndex int

	// Protects directory fields. Must be acquired before bridge.mu
	mu sync.Mutex

	// Directory
	dirStream     DirStream
	hasOverflow   bool
	overflow      fuse.DirEntry
	overflowErrno syscall.Errno

	// dirOffset is the current location in the directory (see `telldir(3)`).
	// The value is equivalent to `d_off` (see `getdents(2)`) of the last
	// directory entry sent to the kernel so far.
	// If `dirOffset` and `fuse.DirEntryList.offset` disagree, then a
	// directory seek has taken place.
	dirOffset uint64

	wg sync.WaitGroup
}

// ServerCallbacks are calls into the kernel to manipulate the inode,
// entry and page cache.  They are stubbed so filesystems can be
// unittested without mounting them.
type ServerCallbacks interface {
	DeleteNotify(parent uint64, child uint64, name string) fuse.Status
	EntryNotify(parent uint64, name string) fuse.Status
	InodeNotify(node uint64, off int64, length int64) fuse.Status
	InodeRetrieveCache(node uint64, offset int64, dest []byte) (n int, st fuse.Status)
	InodeNotifyStoreCache(node uint64, offset int64, data []byte) fuse.Status
}

type rawBridge struct {
	options Options
	root    *Inode
	server  ServerCallbacks

	// mu protects the following data.  Locks for inodes must be
	// taken before rawBridge.mu
	mu sync.Mutex

	// stableAttrs is used to detect already-known nodes and hard links by
	// looking at:
	// 1) file type ......... StableAttr.Mode
	// 2) inode number ...... StableAttr.Ino
	// 3) generation number . StableAttr.Gen
	stableAttrs  map[StableAttr]*Inode
	automaticIno uint64

	// The *Node ID* is an arbitrary uint64 identifier chosen by the FUSE library.
	// It is used the identify *nodes* (files/directories/symlinks/...) in the
	// communication between the FUSE library and the Linux kernel.
	//
	// The kernelNodeIds map translates between the NodeID and the corresponding
	// go-fuse Inode object.
	//
	// A simple incrementing counter is used as the NodeID (see `nextNodeID`).
	kernelNodeIds map[uint64]*Inode
	// nextNodeID is the next free NodeID. Increment after copying the value.
	nextNodeId uint64
	// nodeCountHigh records the highest number of entries we had in the
	// kernelNodeIds map.
	// As the size of stableAttrs tracks kernelNodeIds (+- a few entries due to
	// concurrent FORGETs, LOOKUPs, and the fixed NodeID 1), this is also a good
	// estimate for stableAttrs.
	nodeCountHigh int

	files     []*fileEntry
	freeFiles []uint32
}

// newInode creates creates new inode pointing to ops.
func (b *rawBridge) newInodeUnlocked(ops InodeEmbedder, id StableAttr, persistent bool) *Inode {
	b.mu.Lock()
	defer b.mu.Unlock()

	if id.Reserved() {
		log.Panicf("using reserved ID %d for inode number", id.Ino)
	}

	// This ops already was populated. Just return it.
	if ops.embed().bridge != nil {
		return ops.embed()
	}

	// Only the file type bits matter
	id.Mode = id.Mode & syscall.S_IFMT
	if id.Mode == 0 {
		id.Mode = fuse.S_IFREG
	}

	if id.Ino == 0 {
		// Find free inode number.
		for {
			id.Ino = b.automaticIno
			b.automaticIno++
			_, ok := b.stableAttrs[id]
			if !ok {
				break
			}
		}
	}

	initInode(ops.embed(), ops, id, b, persistent, b.nextNodeId)
	b.nextNodeId++
	return ops.embed()
}

func (b *rawBridge) logf(format string, args ...interface{}) {
	if b.options.Logger != nil {
		b.options.Logger.Printf(format, args...)
	}
}

func (b *rawBridge) newInode(ctx context.Context, ops InodeEmbedder, id StableAttr, persistent bool) *Inode {
	ch := b.newInodeUnlocked(ops, id, persistent)
	if ch != ops.embed() {
		return ch
	}

	if oa, ok := ops.(NodeOnAdder); ok {
		oa.OnAdd(ctx)
	}
	return ch
}

// addNewChild inserts the child into the tree. Returns file handle if file != nil.
// Unless fileFlags has the syscall.O_EXCL bit set, child.stableAttr will be used
// to find an already-known node. If one is found, `child` is ignored and the
// already-known one is used. The node that was actually used is returned.
func (b *rawBridge) addNewChild(parent *Inode, name string, child *Inode, file FileHandle, fileFlags uint32, out *fuse.EntryOut) (selected *Inode, fh uint32) {
	if name == "." || name == ".." {
		log.Panicf("BUG: tried to add virtual entry %q to the actual tree", name)
	}

	// the same node can be looked up through 2 paths in parallel, eg.
	//
	//	    root
	//	    /  \
	//	  dir1 dir2
	//	    \  /
	//	    file
	//
	// dir1.Lookup("file") and dir2.Lookup("file") are executed
	// simultaneously.  The matching StableAttrs ensure that we return the
	// same node.
	orig := child
	id := child.stableAttr
	if id.Mode & ^(uint32(syscall.S_IFMT)) != 0 {
		log.Panicf("%#v", id)
	}
	for {
		lockNodes(parent, child)
		b.mu.Lock()
		if fileFlags&syscall.O_EXCL != 0 {
			// must create a new node - don't look for existing nodes
			break
		}
		old := b.stableAttrs[id]
		if old == nil {
			if child == orig {
				// no pre-existing node under this inode number
				break
			} else {
				// old inode disappeared while we were looping here. Go back to
				// original child.
				b.mu.Unlock()
				unlockNodes(parent, child)
				child = orig
				continue
			}
		}
		if old == child {
			// we now have the right inode locked
			break
		}
		// found a different existing node
		b.mu.Unlock()
		unlockNodes(parent, child)
		child = old
	}

	child.lookupCount++
	child.changeCounter++

	b.kernelNodeIds[child.nodeId] = child
	if len(b.kernelNodeIds) > b.nodeCountHigh {
		b.nodeCountHigh = len(b.kernelNodeIds)
	}
	// Any node that might be there is overwritten - it is obsolete now
	b.stableAttrs[id] = child
	if file != nil {
		fh = b.registerFile(child, file, fileFlags)
	}

	parent.setEntry(name, child)

	out.NodeId = child.nodeId
	out.Generation = child.stableAttr.Gen
	out.Attr.Ino = child.stableAttr.Ino

	b.mu.Unlock()
	unlockNodes(parent, child)

	return child, fh
}

func (b *rawBridge) setEntryOutTimeout(out *fuse.EntryOut) {
	b.setAttr(&out.Attr)
	if b.options.AttrTimeout != nil && out.AttrTimeout() == 0 {
		out.SetAttrTimeout(*b.options.AttrTimeout)
	}
	if b.options.EntryTimeout != nil && out.EntryTimeout() == 0 {
		out.SetEntryTimeout(*b.options.EntryTimeout)
	}
}

func (b *rawBridge) setAttr(out *fuse.Attr) {
	if !b.options.NullPermissions && out.Mode&07777 == 0 {
		out.Mode |= 0644
		if out.Mode&syscall.S_IFDIR != 0 {
			out.Mode |= 0111
		}
	}
	if b.options.UID != 0 && out.Uid == 0 {
		out.Uid = b.options.UID
	}
	if b.options.GID != 0 && out.Gid == 0 {
		out.Gid = b.options.GID
	}
	setBlocks(out)
}

func (b *rawBridge) setAttrTimeout(out *fuse.AttrOut) {
	if b.options.AttrTimeout != nil && out.Timeout() == 0 {
		out.SetTimeout(*b.options.AttrTimeout)
	}
}

// NewNodeFS creates a node based filesystem based on the
// InodeEmbedder instance for the root of the tree.
func NewNodeFS(root InodeEmbedder, opts *Options) fuse.RawFileSystem {
	bridge := &rawBridge{
		automaticIno: opts.FirstAutomaticIno,
		server:       opts.ServerCallbacks,
		nextNodeId:   2, // the root node has nodeid 1
		stableAttrs:  make(map[StableAttr]*Inode),
	}

	if bridge.automaticIno == 0 {
		bridge.automaticIno = 1 << 63
	}

	if opts != nil {
		bridge.options = *opts
	} else {
		oneSec := time.Second
		bridge.options.EntryTimeout = &oneSec
		bridge.options.AttrTimeout = &oneSec
	}

	stableAttr := StableAttr{
		Ino:  root.embed().StableAttr().Ino,
		Mode: fuse.S_IFDIR,
	}
	if opts.RootStableAttr != nil {
		stableAttr.Ino = opts.RootStableAttr.Ino
		stableAttr.Gen = opts.RootStableAttr.Gen
	}

	initInode(root.embed(), root,
		stableAttr,
		bridge,
		false,
		1,
	)
	bridge.root = root.embed()
	bridge.root.lookupCount = 1
	bridge.kernelNodeIds = map[uint64]*Inode{
		1: bridge.root,
	}

	// Fh 0 means no file handle.
	bridge.files = []*fileEntry{{}}

	if opts.OnAdd != nil {
		opts.OnAdd(context.Background())
	} else if oa, ok := root.(NodeOnAdder); ok {
		oa.OnAdd(context.Background())
	}

	return bridge
}

func (b *rawBridge) String() string {
	return "rawBridge"
}

func (b *rawBridge) inode(id uint64, fh uint64) (*Inode, *fileEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, f := b.kernelNodeIds[id], b.files[fh]
	if n == nil {
		log.Panicf("unknown node %d", id)
	}
	return n, f
}

func (b *rawBridge) Lookup(cancel <-chan struct{}, header *fuse.InHeader, name string, out *fuse.EntryOut) fuse.Status {
	parent, _ := b.inode(header.NodeId, 0)
	ctx := &fuse.Context{Caller: header.Caller, Cancel: cancel}
	child, errno := b.lookup(ctx, parent, name, out)

	if errno != 0 {
		if b.options.NegativeTimeout != nil && out.EntryTimeout() == 0 {
			out.SetEntryTimeout(*b.options.NegativeTimeout)
		}
		return errnoToStatus(errno)
	}

	child, _ = b.addNewChild(parent, name, child, nil, 0, out)
	child.setEntryOut(out)
	b.setEntryOutTimeout(out)
	return fuse.OK
}

func (b *rawBridge) lookup(ctx *fuse.Context, parent *Inode, name string, out *fuse.EntryOut) (*Inode, syscall.Errno) {
	if lu, ok := parent.ops.(NodeLookuper); ok {
		return lu.Lookup(ctx, name, out)
	}

	child := parent.GetChild(name)
	if child == nil {
		return nil, syscall.ENOENT
	}

	if ga, ok := child.ops.(NodeGetattrer); ok {
		var a fuse.AttrOut
		errno := ga.Getattr(ctx, nil, &a)
		if errno == 0 {
			out.Attr = a.Attr
		}
	}

	return child, OK
}

func (b *rawBridge) Rmdir(cancel <-chan struct{}, header *fuse.InHeader, name string) fuse.Status {
	parent, _ := b.inode(header.NodeId, 0)
	var errno syscall.Errno
	if mops, ok := parent.ops.(NodeRmdirer); ok {
		errno = mops.Rmdir(&fuse.Context{Caller: header.Caller, Cancel: cancel}, name)
	}

	if errno == 0 {
		parent.RmChild(name)
	}
	return errnoToStatus(errno)
}

func (b *rawBridge) Unlink(cancel <-chan struct{}, header *fuse.InHeader, name string) fuse.Status {
	parent, _ := b.inode(header.NodeId, 0)
	var errno syscall.Errno
	if mops, ok := parent.ops.(NodeUnlinker); ok {
		errno = mops.Unlink(&fuse.Context{Caller: header.Caller, Cancel: cancel}, name)
	}

	if errno == 0 {
		parent.RmChild(name)
	}
	return errnoToStatus(errno)
}

func (b *rawBridge) Mkdir(cancel <-chan struct{}, input *fuse.MkdirIn, name string, out *fuse.EntryOut) fuse.Status {
	parent, _ := b.inode(input.NodeId, 0)

	var child *Inode
	var errno syscall.Errno
	if mops, ok := parent.ops.(NodeMkdirer); ok {
		child, errno = mops.Mkdir(&fuse.Context{Caller: input.Caller, Cancel: cancel}, name, input.Mode, out)
	} else {
		return fuse.ENOTSUP
	}

	if errno != 0 {
		return errnoToStatus(errno)
	}

	if out.Attr.Mode&^07777 == 0 {
		out.Attr.Mode |= fuse.S_IFDIR
	}

	if out.Attr.Mode&^07777 != fuse.S_IFDIR {
		log.Panicf("Mkdir: mode must be S_IFDIR (%o), got %o", fuse.S_IFDIR, out.Attr.Mode)
	}

	child, _ = b.addNewChild(parent, name, child, nil, syscall.O_EXCL, out)
	child.setEntryOut(out)
	b.setEntryOutTimeout(out)
	return fuse.OK
}

func (b *rawBridge) Mknod(cancel <-chan struct{}, input *fuse.MknodIn, name string, out *fuse.EntryOut) fuse.Status {
	parent, _ := b.inode(input.NodeId, 0)

	var child *Inode
	var errno syscall.Errno
	if mops, ok := parent.ops.(NodeMknoder); ok {
		child, errno = mops.Mknod(&fuse.Context{Caller: input.Caller, Cancel: cancel}, name, input.Mode, input.Rdev, out)
	} else {
		return fuse.ENOTSUP
	}

	if errno != 0 {
		return errnoToStatus(errno)
	}

	child, _ = b.addNewChild(parent, name, child, nil, syscall.O_EXCL, out)
	child.setEntryOut(out)
	b.setEntryOutTimeout(out)
	return fuse.OK
}

func (b *rawBridge) Create(cancel <-chan struct{}, input *fuse.CreateIn, name string, out *fuse.CreateOut) fuse.Status {
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	parent, _ := b.inode(input.NodeId, 0)

	var child *Inode
	var errno syscall.Errno
	var f FileHandle
	var flags uint32
	if mops, ok := parent.ops.(NodeCreater); ok {
		child, f, flags, errno = mops.Create(ctx, name, input.Flags, input.Mode, &out.EntryOut)
	} else {
		return fuse.EROFS
	}

	if errno != 0 {
		if b.options.NegativeTimeout != nil {
			out.SetEntryTimeout(*b.options.NegativeTimeout)
		}
		return errnoToStatus(errno)
	}

	child, fh := b.addNewChild(parent, name, child, f, input.Flags|syscall.O_CREAT|syscall.O_EXCL, &out.EntryOut)

	out.Fh = uint64(fh)
	out.OpenFlags = flags

	child.setEntryOut(&out.EntryOut)
	b.setEntryOutTimeout(&out.EntryOut)
	return fuse.OK
}

func (b *rawBridge) Forget(nodeid, nlookup uint64) {
	n, _ := b.inode(nodeid, 0)
	forgotten, _ := n.removeRef(nlookup, false)

	if forgotten {
		b.compactMemory()
	}
}

// compactMemory tries to free memory that was previously used by forgotten
// nodes.
//
// Maps do not free all memory when elements get deleted
// ( https://github.com/golang/go/issues/20135 ).
// As a workaround, we recreate our two big maps (stableAttrs & kernelNodeIds)
// every time they have shrunk dramatically (100 x smaller).
// In this case, `nodeCountHigh` is reset to the new (smaller) size.
func (b *rawBridge) compactMemory() {
	b.mu.Lock()

	if b.nodeCountHigh <= len(b.kernelNodeIds)*100 {
		b.mu.Unlock()
		return
	}

	tmpStableAttrs := make(map[StableAttr]*Inode, len(b.stableAttrs))
	for i, v := range b.stableAttrs {
		tmpStableAttrs[i] = v
	}
	b.stableAttrs = tmpStableAttrs

	tmpKernelNodeIds := make(map[uint64]*Inode, len(b.kernelNodeIds))
	for i, v := range b.kernelNodeIds {
		tmpKernelNodeIds[i] = v
	}
	b.kernelNodeIds = tmpKernelNodeIds

	b.nodeCountHigh = len(b.kernelNodeIds)

	b.mu.Unlock()

	// Run outside b.mu
	debug.FreeOSMemory()
}

func (b *rawBridge) SetDebug(debug bool) {}

func (b *rawBridge) GetAttr(cancel <-chan struct{}, input *fuse.GetAttrIn, out *fuse.AttrOut) fuse.Status {
	n, fEntry := b.inode(input.NodeId, input.Fh())
	f := fEntry.file
	if f == nil {
		// The linux kernel doesnt pass along the file
		// descriptor, so we have to fake it here.
		// See https://github.com/libfuse/libfuse/issues/62
		b.mu.Lock()
		for _, fh := range n.openFiles {
			f = b.files[fh].file
			b.files[fh].wg.Add(1)
			defer b.files[fh].wg.Done()
			break
		}
		b.mu.Unlock()
	}
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	return errnoToStatus(b.getattr(ctx, n, f, out))
}

func (b *rawBridge) getattr(ctx context.Context, n *Inode, f FileHandle, out *fuse.AttrOut) syscall.Errno {
	var errno syscall.Errno

	var fg FileGetattrer
	if f != nil {
		fg, _ = f.(FileGetattrer)
	}

	if fops, ok := n.ops.(NodeGetattrer); ok {
		errno = fops.Getattr(ctx, f, out)
	} else if fg != nil {
		errno = fg.Getattr(ctx, out)
	} else {
		// We set Mode below, which is the minimum for success
	}

	if errno == 0 {
		if out.Ino != 0 && n.stableAttr.Ino > 1 && out.Ino != n.stableAttr.Ino {
			b.logf("warning: rawBridge.getattr: overriding ino %d with %d", out.Ino, n.stableAttr.Ino)
		}
		out.Ino = n.stableAttr.Ino
		out.Mode = (out.Attr.Mode & 07777) | n.stableAttr.Mode
		b.setAttr(&out.Attr)
		b.setAttrTimeout(out)
	}
	return errno
}

func (b *rawBridge) SetAttr(cancel <-chan struct{}, in *fuse.SetAttrIn, out *fuse.AttrOut) fuse.Status {
	ctx := &fuse.Context{Caller: in.Caller, Cancel: cancel}

	fh, _ := in.GetFh()

	n, fEntry := b.inode(in.NodeId, fh)
	f := fEntry.file

	var errno = syscall.ENOTSUP
	if fops, ok := n.ops.(NodeSetattrer); ok {
		errno = fops.Setattr(ctx, f, in, out)
	} else if fops, ok := f.(FileSetattrer); ok {
		errno = fops.Setattr(ctx, in, out)
	}

	out.Mode = n.stableAttr.Mode | (out.Mode & 07777)
	return errnoToStatus(errno)
}

func (b *rawBridge) Rename(cancel <-chan struct{}, input *fuse.RenameIn, oldName string, newName string) fuse.Status {
	p1, _ := b.inode(input.NodeId, 0)
	p2, _ := b.inode(input.Newdir, 0)

	if mops, ok := p1.ops.(NodeRenamer); ok {
		errno := mops.Rename(&fuse.Context{Caller: input.Caller, Cancel: cancel}, oldName, p2.ops, newName, input.Flags)
		if errno == 0 {
			if input.Flags&RENAME_EXCHANGE != 0 {
				p1.ExchangeChild(oldName, p2, newName)
			} else {
				// MvChild cannot fail with overwrite=true.
				_ = p1.MvChild(oldName, p2, newName, true)
			}
		}
		return errnoToStatus(errno)
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Link(cancel <-chan struct{}, input *fuse.LinkIn, name string, out *fuse.EntryOut) fuse.Status {
	parent, _ := b.inode(input.NodeId, 0)
	target, _ := b.inode(input.Oldnodeid, 0)

	if mops, ok := parent.ops.(NodeLinker); ok {
		child, errno := mops.Link(&fuse.Context{Caller: input.Caller, Cancel: cancel}, target.ops, name, out)
		if errno != 0 {
			return errnoToStatus(errno)
		}

		child, _ = b.addNewChild(parent, name, child, nil, 0, out)
		child.setEntryOut(out)
		b.setEntryOutTimeout(out)
		return fuse.OK
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Symlink(cancel <-chan struct{}, header *fuse.InHeader, target string, name string, out *fuse.EntryOut) fuse.Status {
	parent, _ := b.inode(header.NodeId, 0)

	if mops, ok := parent.ops.(NodeSymlinker); ok {
		child, status := mops.Symlink(&fuse.Context{Caller: header.Caller, Cancel: cancel}, target, name, out)
		if status != 0 {
			return errnoToStatus(status)
		}

		child, _ = b.addNewChild(parent, name, child, nil, syscall.O_EXCL, out)
		child.setEntryOut(out)
		b.setEntryOutTimeout(out)
		return fuse.OK
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Readlink(cancel <-chan struct{}, header *fuse.InHeader) (out []byte, status fuse.Status) {
	n, _ := b.inode(header.NodeId, 0)

	if linker, ok := n.ops.(NodeReadlinker); ok {
		result, errno := linker.Readlink(&fuse.Context{Caller: header.Caller, Cancel: cancel})
		if errno != 0 {
			return nil, errnoToStatus(errno)
		}

		return result, fuse.OK

	}

	return nil, fuse.ENOTSUP
}

func (b *rawBridge) Access(cancel <-chan struct{}, input *fuse.AccessIn) fuse.Status {
	n, _ := b.inode(input.NodeId, 0)

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if a, ok := n.ops.(NodeAccesser); ok {
		return errnoToStatus(a.Access(ctx, input.Mask))
	}

	// default: check attributes.
	caller := input.Caller

	var out fuse.AttrOut
	if s := b.getattr(ctx, n, nil, &out); s != 0 {
		return errnoToStatus(s)
	}

	if !internal.HasAccess(caller.Uid, caller.Gid, out.Uid, out.Gid, out.Mode, input.Mask) {
		return fuse.EACCES
	}
	return fuse.OK
}

// Extended attributes.

func (b *rawBridge) GetXAttr(cancel <-chan struct{}, header *fuse.InHeader, attr string, data []byte) (uint32, fuse.Status) {
	n, _ := b.inode(header.NodeId, 0)

	if xops, ok := n.ops.(NodeGetxattrer); ok {
		nb, errno := xops.Getxattr(&fuse.Context{Caller: header.Caller, Cancel: cancel}, attr, data)
		return nb, errnoToStatus(errno)
	}

	return 0, fuse.ENOATTR
}

func (b *rawBridge) ListXAttr(cancel <-chan struct{}, header *fuse.InHeader, dest []byte) (sz uint32, status fuse.Status) {
	n, _ := b.inode(header.NodeId, 0)
	if xops, ok := n.ops.(NodeListxattrer); ok {
		sz, errno := xops.Listxattr(&fuse.Context{Caller: header.Caller, Cancel: cancel}, dest)
		return sz, errnoToStatus(errno)
	}
	return 0, fuse.OK
}

func (b *rawBridge) SetXAttr(cancel <-chan struct{}, input *fuse.SetXAttrIn, attr string, data []byte) fuse.Status {
	n, _ := b.inode(input.NodeId, 0)
	if xops, ok := n.ops.(NodeSetxattrer); ok {
		return errnoToStatus(xops.Setxattr(&fuse.Context{Caller: input.Caller, Cancel: cancel}, attr, data, input.Flags))
	}
	return fuse.ENOATTR
}

func (b *rawBridge) RemoveXAttr(cancel <-chan struct{}, header *fuse.InHeader, attr string) fuse.Status {
	n, _ := b.inode(header.NodeId, 0)
	if xops, ok := n.ops.(NodeRemovexattrer); ok {
		return errnoToStatus(xops.Removexattr(&fuse.Context{Caller: header.Caller, Cancel: cancel}, attr))
	}
	return fuse.ENOATTR
}

func (b *rawBridge) Open(cancel <-chan struct{}, input *fuse.OpenIn, out *fuse.OpenOut) fuse.Status {
	n, _ := b.inode(input.NodeId, 0)

	if op, ok := n.ops.(NodeOpener); ok {
		f, flags, errno := op.Open(&fuse.Context{Caller: input.Caller, Cancel: cancel}, input.Flags)
		if errno != 0 {
			return errnoToStatus(errno)
		}

		if f != nil {
			b.mu.Lock()
			defer b.mu.Unlock()
			out.Fh = uint64(b.registerFile(n, f, input.Flags))
		}
		out.OpenFlags = flags
		return fuse.OK
	}

	return fuse.ENOTSUP
}

// registerFile hands out a file handle. Must have bridge.mu
func (b *rawBridge) registerFile(n *Inode, f FileHandle, flags uint32) uint32 {
	var fh uint32
	if len(b.freeFiles) > 0 {
		last := len(b.freeFiles) - 1
		fh = b.freeFiles[last]
		b.freeFiles = b.freeFiles[:last]
	} else {
		fh = uint32(len(b.files))
		b.files = append(b.files, &fileEntry{})
	}

	fileEntry := b.files[fh]
	fileEntry.nodeIndex = len(n.openFiles)
	fileEntry.file = f

	n.openFiles = append(n.openFiles, fh)
	return fh
}

func (b *rawBridge) Read(cancel <-chan struct{}, input *fuse.ReadIn, buf []byte) (fuse.ReadResult, fuse.Status) {
	n, f := b.inode(input.NodeId, input.Fh)

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if fops, ok := n.ops.(NodeReader); ok {
		res, errno := fops.Read(ctx, f.file, buf, int64(input.Offset))
		return res, errnoToStatus(errno)
	}
	if fr, ok := f.file.(FileReader); ok {
		res, errno := fr.Read(ctx, buf, int64(input.Offset))
		return res, errnoToStatus(errno)
	}

	return nil, fuse.ENOTSUP
}

func (b *rawBridge) GetLk(cancel <-chan struct{}, input *fuse.LkIn, out *fuse.LkOut) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if lops, ok := n.ops.(NodeGetlker); ok {
		return errnoToStatus(lops.Getlk(ctx, f.file, input.Owner, &input.Lk, input.LkFlags, &out.Lk))
	}
	if gl, ok := f.file.(FileGetlker); ok {
		return errnoToStatus(gl.Getlk(ctx, input.Owner, &input.Lk, input.LkFlags, &out.Lk))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) SetLk(cancel <-chan struct{}, input *fuse.LkIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if lops, ok := n.ops.(NodeSetlker); ok {
		return errnoToStatus(lops.Setlk(ctx, f.file, input.Owner, &input.Lk, input.LkFlags))
	}
	if sl, ok := f.file.(FileSetlker); ok {
		return errnoToStatus(sl.Setlk(ctx, input.Owner, &input.Lk, input.LkFlags))
	}
	return fuse.ENOTSUP
}
func (b *rawBridge) SetLkw(cancel <-chan struct{}, input *fuse.LkIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if lops, ok := n.ops.(NodeSetlkwer); ok {
		return errnoToStatus(lops.Setlkw(ctx, f.file, input.Owner, &input.Lk, input.LkFlags))
	}
	if sl, ok := f.file.(FileSetlkwer); ok {
		return errnoToStatus(sl.Setlkw(ctx, input.Owner, &input.Lk, input.LkFlags))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Release(cancel <-chan struct{}, input *fuse.ReleaseIn) {
	n, f := b.releaseFileEntry(input.NodeId, input.Fh)
	if f == nil {
		return
	}

	f.wg.Wait()

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if r, ok := n.ops.(NodeReleaser); ok {
		r.Release(ctx, f.file)
	} else if r, ok := f.file.(FileReleaser); ok {
		r.Release(ctx)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.freeFiles = append(b.freeFiles, uint32(input.Fh))
}

func (b *rawBridge) ReleaseDir(input *fuse.ReleaseIn) {
	_, f := b.releaseFileEntry(input.NodeId, input.Fh)
	f.wg.Wait()

	f.mu.Lock()
	if f.dirStream != nil {
		f.dirStream.Close()
		f.dirStream = nil
	}
	f.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()
	b.freeFiles = append(b.freeFiles, uint32(input.Fh))
}

func (b *rawBridge) releaseFileEntry(nid uint64, fh uint64) (*Inode, *fileEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := b.kernelNodeIds[nid]
	var entry *fileEntry
	if fh > 0 {
		last := len(n.openFiles) - 1
		entry = b.files[fh]
		if last != entry.nodeIndex {
			n.openFiles[entry.nodeIndex] = n.openFiles[last]

			b.files[n.openFiles[entry.nodeIndex]].nodeIndex = entry.nodeIndex
		}
		n.openFiles = n.openFiles[:last]
	}
	return n, entry
}

func (b *rawBridge) Write(cancel <-chan struct{}, input *fuse.WriteIn, data []byte) (written uint32, status fuse.Status) {
	n, f := b.inode(input.NodeId, input.Fh)

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if wr, ok := n.ops.(NodeWriter); ok {
		w, errno := wr.Write(ctx, f.file, data, int64(input.Offset))
		return w, errnoToStatus(errno)
	}
	if fr, ok := f.file.(FileWriter); ok {
		w, errno := fr.Write(ctx, data, int64(input.Offset))
		return w, errnoToStatus(errno)
	}

	return 0, fuse.ENOTSUP
}

func (b *rawBridge) Flush(cancel <-chan struct{}, input *fuse.FlushIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if fl, ok := n.ops.(NodeFlusher); ok {
		return errnoToStatus(fl.Flush(ctx, f.file))
	}
	if fl, ok := f.file.(FileFlusher); ok {
		return errnoToStatus(fl.Flush(ctx))
	}
	return 0
}

func (b *rawBridge) Fsync(cancel <-chan struct{}, input *fuse.FsyncIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if fs, ok := n.ops.(NodeFsyncer); ok {
		return errnoToStatus(fs.Fsync(ctx, f.file, input.FsyncFlags))
	}
	if fs, ok := f.file.(FileFsyncer); ok {
		return errnoToStatus(fs.Fsync(ctx, input.FsyncFlags))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Fallocate(cancel <-chan struct{}, input *fuse.FallocateIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	if a, ok := n.ops.(NodeAllocater); ok {
		return errnoToStatus(a.Allocate(ctx, f.file, input.Offset, input.Length, input.Mode))
	}
	if a, ok := f.file.(FileAllocater); ok {
		return errnoToStatus(a.Allocate(ctx, input.Offset, input.Length, input.Mode))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) OpenDir(cancel <-chan struct{}, input *fuse.OpenIn, out *fuse.OpenOut) fuse.Status {
	n, _ := b.inode(input.NodeId, 0)

	if od, ok := n.ops.(NodeOpendirer); ok {
		errno := od.Opendir(&fuse.Context{Caller: input.Caller, Cancel: cancel})
		if errno != 0 {
			return errnoToStatus(errno)
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	out.Fh = uint64(b.registerFile(n, nil, 0))
	return fuse.OK
}

// setStream makes sure `f.dirStream` and associated state variables are set and
// seeks to offset requested in `input`. Caller must hold `f.mu`.
// The `eof` return value shows if `f.dirStream` ended before the requested
// offset was reached.
func (b *rawBridge) setStream(cancel <-chan struct{}, input *fuse.ReadIn, inode *Inode, f *fileEntry) (errno syscall.Errno, eof bool) {
	// Get a new directory stream in the following cases:
	// 1) f.dirStream == nil ............ First READDIR[PLUS] on this file handle.
	// 2) input.Offset == 0 ............. Start reading the directory again from
	//                                    the beginning (user called rewinddir(3) or lseek(2)).
	// 3) input.Offset < f.nextOffset ... Seek back (user called seekdir(3) or lseek(2)).
	if f.dirStream == nil || input.Offset == 0 || input.Offset < f.dirOffset {
		if f.dirStream != nil {
			f.dirStream.Close()
			f.dirStream = nil
		}
		str, errno := b.getStream(&fuse.Context{Caller: input.Caller, Cancel: cancel}, inode)
		if errno != 0 {
			return errno, false
		}

		f.dirOffset = 0
		f.hasOverflow = false
		f.dirStream = str
	}

	// Seek forward?
	for f.dirOffset < input.Offset {
		f.hasOverflow = false
		if !f.dirStream.HasNext() {
			// Seek past end of directory. This is not an error, but the
			// user will get an empty directory listing.
			return 0, true
		}
		_, errno := f.dirStream.Next()
		if errno != 0 {
			return errno, true
		}
		f.dirOffset++
	}

	return 0, false
}

func (b *rawBridge) getStream(ctx context.Context, inode *Inode) (DirStream, syscall.Errno) {
	if rd, ok := inode.ops.(NodeReaddirer); ok {
		return rd.Readdir(ctx)
	}

	lst := inode.childrenList()
	r := make([]fuse.DirEntry, 0, len(lst))
	for _, e := range lst {
		r = append(r, fuse.DirEntry{Mode: e.Inode.Mode(),
			Name: e.Name,
			Ino:  e.Inode.StableAttr().Ino})
	}
	return NewListDirStream(r), 0
}

func (b *rawBridge) ReadDir(cancel <-chan struct{}, input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)

	f.mu.Lock()
	defer f.mu.Unlock()

	errno, eof := b.setStream(cancel, input, n, f)
	if errno != 0 {
		return errnoToStatus(errno)
	} else if eof {
		return fuse.OK
	}

	if f.hasOverflow {
		if f.overflowErrno != 0 {
			return errnoToStatus(f.overflowErrno)
		}

		f.hasOverflow = false
		// always succeeds.
		out.AddDirEntry(f.overflow)
		f.dirOffset++
	}

	first := true
	for f.dirStream.HasNext() {
		e, errno := f.dirStream.Next()

		if errno != 0 {
			if first {
				return errnoToStatus(errno)
			} else {
				f.hasOverflow = true
				f.overflowErrno = errno
				return fuse.OK
			}
		}

		first = false
		if !out.AddDirEntry(e) {
			f.overflow = e
			f.hasOverflow = true
			return errnoToStatus(errno)
		}
		f.dirOffset++
	}

	return fuse.OK
}

func (b *rawBridge) ReadDirPlus(cancel <-chan struct{}, input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)

	f.mu.Lock()
	defer f.mu.Unlock()

	errno, eof := b.setStream(cancel, input, n, f)
	if errno != 0 {
		return errnoToStatus(errno)
	} else if eof {
		return fuse.OK
	}

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}

	first := true
	for f.dirStream.HasNext() || f.hasOverflow {
		var e fuse.DirEntry
		var errno syscall.Errno

		if f.hasOverflow {
			e = f.overflow
			errno = f.overflowErrno
			f.hasOverflow = false
		} else {
			e, errno = f.dirStream.Next()
		}

		if errno != 0 {
			if first {
				return errnoToStatus(errno)
			} else {
				f.overflowErrno = errno
				f.hasOverflow = true
				return fuse.OK
			}
		}
		first = false

		entryOut := out.AddDirLookupEntry(e)
		if entryOut == nil {
			f.overflow = e
			f.hasOverflow = true
			return fuse.OK
		}
		f.dirOffset++

		// Virtual entries "." and ".." should be part of the
		// directory listing, but not part of the filesystem tree.
		// The values in EntryOut are ignored by Linux
		// (see fuse_direntplus_link() in linux/fs/fuse/readdir.c), so leave
		// them at zero-value.
		if e.Name == "." || e.Name == ".." {
			continue
		}

		child, errno := b.lookup(ctx, n, e.Name, entryOut)
		if errno != 0 {
			if b.options.NegativeTimeout != nil {
				entryOut.SetEntryTimeout(*b.options.NegativeTimeout)
			}
		} else {
			child, _ = b.addNewChild(n, e.Name, child, nil, 0, entryOut)
			child.setEntryOut(entryOut)
			b.setEntryOutTimeout(entryOut)
			if e.Mode&syscall.S_IFMT != child.stableAttr.Mode&syscall.S_IFMT {
				// The file type has changed behind our back. Use the new value.
				out.FixMode(child.stableAttr.Mode)
			}
			entryOut.Mode = child.stableAttr.Mode | (entryOut.Mode & 07777)
		}
	}

	return fuse.OK
}

func (b *rawBridge) FsyncDir(cancel <-chan struct{}, input *fuse.FsyncIn) fuse.Status {
	n, _ := b.inode(input.NodeId, input.Fh)
	if fs, ok := n.ops.(NodeFsyncer); ok {
		return errnoToStatus(fs.Fsync(&fuse.Context{Caller: input.Caller, Cancel: cancel}, nil, input.FsyncFlags))
	}

	return fuse.ENOTSUP
}

func (b *rawBridge) StatFs(cancel <-chan struct{}, input *fuse.InHeader, out *fuse.StatfsOut) fuse.Status {
	n, _ := b.inode(input.NodeId, 0)
	if sf, ok := n.ops.(NodeStatfser); ok {
		return errnoToStatus(sf.Statfs(&fuse.Context{Caller: input.Caller, Cancel: cancel}, out))
	}

	// leave zeroed out
	return fuse.OK
}

func (b *rawBridge) Init(s *fuse.Server) {
	b.server = s
}

func (b *rawBridge) CopyFileRange(cancel <-chan struct{}, in *fuse.CopyFileRangeIn) (size uint32, status fuse.Status) {
	n1, f1 := b.inode(in.NodeId, in.FhIn)
	cfr, ok := n1.ops.(NodeCopyFileRanger)
	if !ok {
		return 0, fuse.ENOTSUP
	}

	n2, f2 := b.inode(in.NodeIdOut, in.FhOut)

	sz, errno := cfr.CopyFileRange(&fuse.Context{Caller: in.Caller, Cancel: cancel},
		f1.file, in.OffIn, n2, f2.file, in.OffOut, in.Len, in.Flags)
	return sz, errnoToStatus(errno)
}

func (b *rawBridge) Lseek(cancel <-chan struct{}, in *fuse.LseekIn, out *fuse.LseekOut) fuse.Status {
	n, f := b.inode(in.NodeId, in.Fh)

	ctx := &fuse.Context{Caller: in.Caller, Cancel: cancel}

	ls, ok := n.ops.(NodeLseeker)
	if ok {
		off, errno := ls.Lseek(ctx,
			f.file, in.Offset, in.Whence)
		out.Offset = off
		return errnoToStatus(errno)
	}
	if fs, ok := f.file.(FileLseeker); ok {
		off, errno := fs.Lseek(ctx, in.Offset, in.Whence)
		out.Offset = off
		return errnoToStatus(errno)
	}

	if in.Whence == _SEEK_DATA {
		out.Offset = in.Offset
		return fuse.OK
	}

	if in.Whence == _SEEK_HOLE {
		var attr fuse.AttrOut
		if s := b.getattr(ctx, n, nil, &attr); s != 0 {
			return errnoToStatus(s)
		}

		out.Offset = attr.Size
		return fuse.OK
	}

	return fuse.ENOTSUP
}
