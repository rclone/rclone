// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"context"
	"log"
	"math/rand"
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
	dirStream   DirStream
	hasOverflow bool
	overflow    fuse.DirEntry

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
	mu           sync.Mutex
	nodes        map[uint64]*Inode
	automaticIno uint64

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

	if id.Ino == 0 {
		for {
			id.Ino = b.automaticIno
			b.automaticIno++
			_, ok := b.nodes[id.Ino]
			if !ok {
				break
			}
		}
	}

	// Only the file type bits matter
	id.Mode = id.Mode & syscall.S_IFMT
	if id.Mode == 0 {
		id.Mode = fuse.S_IFREG
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
	var t time.Duration
	t0 := time.Now()
	for i := 1; true; i++ {
		old := b.nodes[id.Ino]
		if old == nil {
			break
		}
		if old.stableAttr == id {
			return old
		}
		b.mu.Unlock()

		t = expSleep(t)
		if i%5000 == 0 {
			b.logf("blocked for %.0f seconds waiting for FORGET on i%d", time.Since(t0).Seconds(), id.Ino)
		}
		b.mu.Lock()
	}

	b.nodes[id.Ino] = ops.embed()
	initInode(ops.embed(), ops, id, b, persistent)
	return ops.embed()
}

func (b *rawBridge) logf(format string, args ...interface{}) {
	if b.options.Logger != nil {
		b.options.Logger.Printf(format, args...)
	}
}

// expSleep sleeps for time `t` and returns an exponentially increasing value
// for the next sleep time, capped at 1 ms.
func expSleep(t time.Duration) time.Duration {
	if t == 0 {
		return time.Microsecond
	}
	time.Sleep(t)
	// Next sleep is between t and 2*t
	t += time.Duration(rand.Int63n(int64(t)))
	if t >= time.Millisecond {
		return time.Millisecond
	}
	return t
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
func (b *rawBridge) addNewChild(parent *Inode, name string, child *Inode, file FileHandle, fileFlags uint32, out *fuse.EntryOut) uint32 {
	if name == "." || name == ".." {
		log.Panicf("BUG: tried to add virtual entry %q to the actual tree", name)
	}
	lockNodes(parent, child)
	parent.setEntry(name, child)
	b.mu.Lock()

	// Due to concurrent FORGETs, lookupCount may have dropped to zero.
	// This means it MAY have been deleted from nodes[] already. Add it back.
	if child.lookupCount == 0 {
		b.nodes[child.stableAttr.Ino] = child
	}
	child.lookupCount++
	child.changeCounter++

	var fh uint32
	if file != nil {
		fh = b.registerFile(child, file, fileFlags)
	}

	out.NodeId = child.stableAttr.Ino
	out.Generation = child.stableAttr.Gen
	out.Attr.Ino = child.stableAttr.Ino

	b.mu.Unlock()
	unlockNodes(parent, child)
	return fh
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
	}
	if bridge.automaticIno == 1 {
		bridge.automaticIno++
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

	initInode(root.embed(), root,
		StableAttr{
			Ino:  1,
			Mode: fuse.S_IFDIR,
		},
		bridge,
		false,
	)
	bridge.root = root.embed()
	bridge.root.lookupCount = 1
	bridge.nodes = map[uint64]*Inode{
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
	n, f := b.nodes[id], b.files[fh]
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

	child.setEntryOut(out)
	b.addNewChild(parent, name, child, nil, 0, out)
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

	child.setEntryOut(out)
	b.addNewChild(parent, name, child, nil, 0, out)
	b.setEntryOutTimeout(out)
	return fuse.OK
}

func (b *rawBridge) Mknod(cancel <-chan struct{}, input *fuse.MknodIn, name string, out *fuse.EntryOut) fuse.Status {
	parent, _ := b.inode(input.NodeId, 0)

	var child *Inode
	var errno syscall.Errno
	if mops, ok := parent.ops.(NodeMknoder); ok {
		child, errno = mops.Mknod(&fuse.Context{Caller: input.Caller, Cancel: cancel}, name, input.Mode, input.Rdev, out)
	}

	if errno != 0 {
		return errnoToStatus(errno)
	}

	child.setEntryOut(out)
	b.addNewChild(parent, name, child, nil, 0, out)
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

	out.Fh = uint64(b.addNewChild(parent, name, child, f, input.Flags|syscall.O_CREAT, &out.EntryOut))

	out.OpenFlags = flags

	child.setEntryOut(&out.EntryOut)
	b.setEntryOutTimeout(&out.EntryOut)
	return fuse.OK
}

func (b *rawBridge) Forget(nodeid, nlookup uint64) {
	n, _ := b.inode(nodeid, 0)
	n.removeRef(nlookup, false)
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

		child.setEntryOut(out)
		b.addNewChild(parent, name, child, nil, 0, out)
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

		b.addNewChild(parent, name, child, nil, 0, out)
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

	if fops, ok := n.ops.(NodeReader); ok {
		res, errno := fops.Read(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, buf, int64(input.Offset))
		return res, errnoToStatus(errno)
	}
	if fr, ok := f.file.(FileReader); ok {
		res, errno := fr.Read(&fuse.Context{Caller: input.Caller, Cancel: cancel}, buf, int64(input.Offset))
		return res, errnoToStatus(errno)
	}

	return nil, fuse.ENOTSUP
}

func (b *rawBridge) GetLk(cancel <-chan struct{}, input *fuse.LkIn, out *fuse.LkOut) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)

	if lops, ok := n.ops.(NodeGetlker); ok {
		return errnoToStatus(lops.Getlk(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, input.Owner, &input.Lk, input.LkFlags, &out.Lk))
	}
	if gl, ok := f.file.(FileGetlker); ok {
		return errnoToStatus(gl.Getlk(&fuse.Context{Caller: input.Caller, Cancel: cancel}, input.Owner, &input.Lk, input.LkFlags, &out.Lk))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) SetLk(cancel <-chan struct{}, input *fuse.LkIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	if lops, ok := n.ops.(NodeSetlker); ok {
		return errnoToStatus(lops.Setlk(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, input.Owner, &input.Lk, input.LkFlags))
	}
	if sl, ok := n.ops.(FileSetlker); ok {
		return errnoToStatus(sl.Setlk(&fuse.Context{Caller: input.Caller, Cancel: cancel}, input.Owner, &input.Lk, input.LkFlags))
	}
	return fuse.ENOTSUP
}
func (b *rawBridge) SetLkw(cancel <-chan struct{}, input *fuse.LkIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	if lops, ok := n.ops.(NodeSetlkwer); ok {
		return errnoToStatus(lops.Setlkw(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, input.Owner, &input.Lk, input.LkFlags))
	}
	if sl, ok := n.ops.(FileSetlkwer); ok {
		return errnoToStatus(sl.Setlkw(&fuse.Context{Caller: input.Caller, Cancel: cancel}, input.Owner, &input.Lk, input.LkFlags))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Release(cancel <-chan struct{}, input *fuse.ReleaseIn) {
	n, f := b.releaseFileEntry(input.NodeId, input.Fh)
	if f == nil {
		return
	}

	f.wg.Wait()

	if r, ok := n.ops.(NodeReleaser); ok {
		r.Release(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file)
	} else if r, ok := f.file.(FileReleaser); ok {
		r.Release(&fuse.Context{Caller: input.Caller, Cancel: cancel})
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

	n := b.nodes[nid]
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

	if wr, ok := n.ops.(NodeWriter); ok {
		w, errno := wr.Write(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, data, int64(input.Offset))
		return w, errnoToStatus(errno)
	}
	if fr, ok := f.file.(FileWriter); ok {
		w, errno := fr.Write(&fuse.Context{Caller: input.Caller, Cancel: cancel}, data, int64(input.Offset))
		return w, errnoToStatus(errno)
	}

	return 0, fuse.ENOTSUP
}

func (b *rawBridge) Flush(cancel <-chan struct{}, input *fuse.FlushIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	if fl, ok := n.ops.(NodeFlusher); ok {
		return errnoToStatus(fl.Flush(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file))
	}
	if fl, ok := f.file.(FileFlusher); ok {
		return errnoToStatus(fl.Flush(&fuse.Context{Caller: input.Caller, Cancel: cancel}))
	}
	return 0
}

func (b *rawBridge) Fsync(cancel <-chan struct{}, input *fuse.FsyncIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	if fs, ok := n.ops.(NodeFsyncer); ok {
		return errnoToStatus(fs.Fsync(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, input.FsyncFlags))
	}
	if fs, ok := f.file.(FileFsyncer); ok {
		return errnoToStatus(fs.Fsync(&fuse.Context{Caller: input.Caller, Cancel: cancel}, input.FsyncFlags))
	}
	return fuse.ENOTSUP
}

func (b *rawBridge) Fallocate(cancel <-chan struct{}, input *fuse.FallocateIn) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)
	if a, ok := n.ops.(NodeAllocater); ok {
		return errnoToStatus(a.Allocate(&fuse.Context{Caller: input.Caller, Cancel: cancel}, f.file, input.Offset, input.Length, input.Mode))
	}
	if a, ok := f.file.(FileAllocater); ok {
		return errnoToStatus(a.Allocate(&fuse.Context{Caller: input.Caller, Cancel: cancel}, input.Offset, input.Length, input.Mode))
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

// setStream sets the directory part of f. Must hold f.mu
func (b *rawBridge) setStream(cancel <-chan struct{}, input *fuse.ReadIn, inode *Inode, f *fileEntry) syscall.Errno {
	if f.dirStream == nil || input.Offset == 0 {
		if f.dirStream != nil {
			f.dirStream.Close()
			f.dirStream = nil
		}
		str, errno := b.getStream(&fuse.Context{Caller: input.Caller, Cancel: cancel}, inode)
		if errno != 0 {
			return errno
		}

		f.hasOverflow = false
		f.dirStream = str
	}

	return 0
}

func (b *rawBridge) getStream(ctx context.Context, inode *Inode) (DirStream, syscall.Errno) {
	if rd, ok := inode.ops.(NodeReaddirer); ok {
		return rd.Readdir(ctx)
	}

	r := []fuse.DirEntry{}
	for k, ch := range inode.Children() {
		r = append(r, fuse.DirEntry{Mode: ch.Mode(),
			Name: k,
			Ino:  ch.StableAttr().Ino})
	}
	return NewListDirStream(r), 0
}

func (b *rawBridge) ReadDir(cancel <-chan struct{}, input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)

	f.mu.Lock()
	defer f.mu.Unlock()
	if errno := b.setStream(cancel, input, n, f); errno != 0 {
		return errnoToStatus(errno)
	}

	if f.hasOverflow {
		// always succeeds.
		out.AddDirEntry(f.overflow)
		f.hasOverflow = false
	}

	for f.dirStream.HasNext() {
		e, errno := f.dirStream.Next()

		if errno != 0 {
			return errnoToStatus(errno)
		}
		if !out.AddDirEntry(e) {
			f.overflow = e
			f.hasOverflow = true
			return errnoToStatus(errno)
		}
	}

	return fuse.OK
}

func (b *rawBridge) ReadDirPlus(cancel <-chan struct{}, input *fuse.ReadIn, out *fuse.DirEntryList) fuse.Status {
	n, f := b.inode(input.NodeId, input.Fh)

	f.mu.Lock()
	defer f.mu.Unlock()
	if errno := b.setStream(cancel, input, n, f); errno != 0 {
		return errnoToStatus(errno)
	}

	ctx := &fuse.Context{Caller: input.Caller, Cancel: cancel}
	for f.dirStream.HasNext() || f.hasOverflow {
		var e fuse.DirEntry
		var errno syscall.Errno

		if f.hasOverflow {
			e = f.overflow
			f.hasOverflow = false
		} else {
			e, errno = f.dirStream.Next()
		}

		if errno != 0 {
			return errnoToStatus(errno)
		}

		entryOut := out.AddDirLookupEntry(e)
		if entryOut == nil {
			f.overflow = e
			f.hasOverflow = true
			return fuse.OK
		}

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
			b.addNewChild(n, e.Name, child, nil, 0, entryOut)
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

	ls, ok := n.ops.(NodeLseeker)
	if ok {
		off, errno := ls.Lseek(&fuse.Context{Caller: in.Caller, Cancel: cancel},
			f.file, in.Offset, in.Whence)
		out.Offset = off
		return errnoToStatus(errno)
	}
	if fs, ok := f.file.(FileLseeker); ok {
		off, errno := fs.Lseek(&fuse.Context{Caller: in.Caller, Cancel: cancel}, in.Offset, in.Whence)
		out.Offset = off
		return errnoToStatus(errno)
	}

	if in.Whence == _SEEK_DATA || in.Whence == _SEEK_HOLE {
		out.Offset = in.Offset
		return fuse.OK
	}

	return fuse.ENOTSUP
}
