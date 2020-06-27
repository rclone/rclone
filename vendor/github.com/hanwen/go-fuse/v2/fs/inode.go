// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fuse"
)

type parentData struct {
	name   string
	parent *Inode
}

// StableAttr holds immutable attributes of a object in the filesystem.
type StableAttr struct {
	// Each Inode has a type, which does not change over the
	// lifetime of the inode, for example fuse.S_IFDIR. The default (0)
	// is interpreted as S_IFREG (regular file).
	Mode uint32

	// The inode number must be unique among the currently live
	// objects in the file system. It is used to communicate to
	// the kernel about this file object. The values uint64(-1),
	// and 1 are reserved. When using Ino==0, a unique, sequential
	// number is assigned (starting at 2^63 by default) on Inode creation.
	Ino uint64

	// When reusing a previously used inode number for a new
	// object, the new object must have a different Gen
	// number. This is irrelevant if the FS is not exported over
	// NFS
	Gen uint64
}

// Reserved returns if the StableAttr is using reserved Inode numbers.
func (i *StableAttr) Reserved() bool {
	return i.Ino == 1 || i.Ino == ^uint64(0)
}

// Inode is a node in VFS tree.  Inodes are one-to-one mapped to
// Operations instances, which is the extension interface for file
// systems.  One can create fully-formed trees of Inodes ahead of time
// by creating "persistent" Inodes.
//
// The Inode struct contains a lock, so it should not be
// copied. Inodes should be obtained by calling Inode.NewInode() or
// Inode.NewPersistentInode().
type Inode struct {
	stableAttr StableAttr

	ops    InodeEmbedder
	bridge *rawBridge

	// Following data is mutable.

	// file handles.
	// protected by bridge.mu
	openFiles []uint32

	// mu protects the following mutable fields. When locking
	// multiple Inodes, locks must be acquired using
	// lockNodes/unlockNodes
	mu sync.Mutex

	// persistent indicates that this node should not be removed
	// from the tree, even if there are no live references. This
	// must be set on creation, and can only be changed to false
	// by calling removeRef.
	// When you change this, you MUST increment changeCounter.
	persistent bool

	// changeCounter increments every time the mutable state
	// (lookupCount, persistent, children, parents) protected by
	// mu is modified.
	//
	// This is used in places where we have to relock inode into inode
	// group lock, and after locking the group we have to check if inode
	// did not changed, and if it changed - retry the operation.
	changeCounter uint32

	// Number of kernel refs to this node.
	// When you change this, you MUST increment changeCounter.
	lookupCount uint64

	// Children of this Inode.
	// When you change this, you MUST increment changeCounter.
	children map[string]*Inode

	// Parents of this Inode. Can be more than one due to hard links.
	// When you change this, you MUST increment changeCounter.
	parents map[parentData]struct{}
}

func (n *Inode) IsDir() bool {
	return n.stableAttr.Mode&syscall.S_IFDIR != 0
}

func (n *Inode) embed() *Inode {
	return n
}

func (n *Inode) EmbeddedInode() *Inode {
	return n
}

func initInode(n *Inode, ops InodeEmbedder, attr StableAttr, bridge *rawBridge, persistent bool) {
	n.ops = ops
	n.stableAttr = attr
	n.bridge = bridge
	n.persistent = persistent
	n.parents = make(map[parentData]struct{})
	if attr.Mode == fuse.S_IFDIR {
		n.children = make(map[string]*Inode)
	}
}

// Set node ID and mode in EntryOut
func (n *Inode) setEntryOut(out *fuse.EntryOut) {
	out.NodeId = n.stableAttr.Ino
	out.Ino = n.stableAttr.Ino
	out.Mode = (out.Attr.Mode & 07777) | n.stableAttr.Mode
}

// StableAttr returns the (Ino, Gen) tuple for this node.
func (n *Inode) StableAttr() StableAttr {
	return n.stableAttr
}

// Mode returns the filetype
func (n *Inode) Mode() uint32 {
	return n.stableAttr.Mode
}

// Returns the root of the tree
func (n *Inode) Root() *Inode {
	return n.bridge.root
}

// Returns whether this is the root of the tree
func (n *Inode) IsRoot() bool {
	return n.bridge.root == n
}

func modeStr(m uint32) string {
	return map[uint32]string{
		syscall.S_IFREG:  "reg",
		syscall.S_IFLNK:  "lnk",
		syscall.S_IFDIR:  "dir",
		syscall.S_IFSOCK: "soc",
		syscall.S_IFIFO:  "pip",
		syscall.S_IFCHR:  "chr",
		syscall.S_IFBLK:  "blk",
	}[m]
}

// debugString is used for debugging. Racy.
func (n *Inode) String() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var ss []string
	for nm, ch := range n.children {
		ss = append(ss, fmt.Sprintf("%q=i%d[%s]", nm, ch.stableAttr.Ino, modeStr(ch.stableAttr.Mode)))
	}

	return fmt.Sprintf("i%d (%s): %s", n.stableAttr.Ino, modeStr(n.stableAttr.Mode), strings.Join(ss, ","))
}

// sortNodes rearranges inode group in consistent order.
//
// The nodes are ordered by their in-RAM address, which gives consistency
// property: for any A and B inodes, sortNodes will either always order A < B,
// or always order A > B.
//
// See lockNodes where this property is used to avoid deadlock when taking
// locks on inode group.
func sortNodes(ns []*Inode) {
	sort.Slice(ns, func(i, j int) bool {
		return nodeLess(ns[i], ns[j])
	})
}

func nodeLess(a, b *Inode) bool {
	return uintptr(unsafe.Pointer(a)) < uintptr(unsafe.Pointer(b))
}

// lockNodes locks group of inodes.
//
// It always lock the inodes in the same order - to avoid deadlocks.
// It also avoids locking an inode more than once, if it was specified multiple times.
// An example when an inode might be given multiple times is if dir/a and dir/b
// are hardlinked to the same inode and the caller needs to take locks on dir children.
func lockNodes(ns ...*Inode) {
	sortNodes(ns)

	// The default value nil prevents trying to lock nil nodes.
	var nprev *Inode
	for _, n := range ns {
		if n != nprev {
			n.mu.Lock()
			nprev = n
		}
	}
}

// lockNode2 locks a and b in order consistent with lockNodes.
func lockNode2(a, b *Inode) {
	if a == b {
		a.mu.Lock()
	} else if nodeLess(a, b) {
		a.mu.Lock()
		b.mu.Lock()
	} else {
		b.mu.Lock()
		a.mu.Lock()
	}
}

// unlockNode2 unlocks a and b
func unlockNode2(a, b *Inode) {
	if a == b {
		a.mu.Unlock()
	} else {
		a.mu.Unlock()
		b.mu.Unlock()
	}
}

// unlockNodes releases locks taken by lockNodes.
func unlockNodes(ns ...*Inode) {
	// we don't need to unlock in the same order that was used in lockNodes.
	// however it still helps to have nodes sorted to avoid duplicates.
	sortNodes(ns)

	var nprev *Inode
	for _, n := range ns {
		if n != nprev {
			n.mu.Unlock()
			nprev = n
		}
	}
}

// Forgotten returns true if the kernel holds no references to this
// inode.  This can be used for background cleanup tasks, since the
// kernel has no way of reviving forgotten nodes by its own
// initiative.
func (n *Inode) Forgotten() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lookupCount == 0 && len(n.parents) == 0 && !n.persistent
}

// Operations returns the object implementing the file system
// operations.
func (n *Inode) Operations() InodeEmbedder {
	return n.ops
}

// Path returns a path string to the inode relative to `root`.
// Pass nil to walk the hierarchy as far up as possible.
//
// If you set `root`, Path() warns if it finds an orphaned Inode, i.e.
// if it does not end up at `root` after walking the hierarchy.
func (n *Inode) Path(root *Inode) string {
	var segments []string
	p := n
	for p != nil && p != root {
		var pd parentData

		// We don't try to take all locks at the same time, because
		// the caller won't use the "path" string under lock anyway.
		found := false
		p.mu.Lock()
		// Select an arbitrary parent
		for pd = range p.parents {
			found = true
			break
		}
		p.mu.Unlock()
		if found == false {
			p = nil
			break
		}
		if pd.parent == nil {
			break
		}

		segments = append(segments, pd.name)
		p = pd.parent
	}

	if root != nil && root != p {
		deletedPlaceholder := fmt.Sprintf(".go-fuse.%d/deleted", rand.Uint64())
		n.bridge.logf("warning: Inode.Path: inode i%d is orphaned, replacing segment with %q",
			n.stableAttr.Ino, deletedPlaceholder)
		// NOSUBMIT - should replace rather than append?
		segments = append(segments, deletedPlaceholder)
	}

	i := 0
	j := len(segments) - 1

	for i < j {
		segments[i], segments[j] = segments[j], segments[i]
		i++
		j--
	}

	path := strings.Join(segments, "/")
	return path
}

// setEntry does `iparent[name] = ichild` linking.
//
// setEntry must not be called simultaneously for any of iparent or ichild.
// This, for example could be satisfied if both iparent and ichild are locked,
// but it could be also valid if only iparent is locked and ichild was just
// created and only one goroutine keeps referencing it.
func (iparent *Inode) setEntry(name string, ichild *Inode) {
	ichild.parents[parentData{name, iparent}] = struct{}{}
	iparent.children[name] = ichild
	ichild.changeCounter++
	iparent.changeCounter++
}

// NewPersistentInode returns an Inode whose lifetime is not in
// control of the kernel.
//
// When the kernel is short on memory, it will forget cached file
// system information (directory entries and inode metadata). This is
// announced with FORGET messages.  There are no guarantees if or when
// this happens. When it happens, these are handled transparently by
// go-fuse: all Inodes created with NewInode are released
// automatically. NewPersistentInode creates inodes that go-fuse keeps
// in memory, even if the kernel is not interested in them. This is
// convenient for building static trees up-front.
func (n *Inode) NewPersistentInode(ctx context.Context, node InodeEmbedder, id StableAttr) *Inode {
	return n.newInode(ctx, node, id, true)
}

// ForgetPersistent manually marks the node as no longer important. If
// it has no children, and if the kernel as no references, the nodes
// gets removed from the tree.
func (n *Inode) ForgetPersistent() {
	n.removeRef(0, true)
}

// NewInode returns an inode for the given InodeEmbedder. The mode
// should be standard mode argument (eg. S_IFDIR). The inode number in
// id.Ino argument is used to implement hard-links.  If it is given,
// and another node with the same ID is known, that will node will be
// returned, and the passed-in `node` is ignored.
func (n *Inode) NewInode(ctx context.Context, node InodeEmbedder, id StableAttr) *Inode {
	return n.newInode(ctx, node, id, false)
}

func (n *Inode) newInode(ctx context.Context, ops InodeEmbedder, id StableAttr, persistent bool) *Inode {
	return n.bridge.newInode(ctx, ops, id, persistent)
}

// removeRef decreases references. Returns if this operation caused
// the node to be forgotten (for kernel references), and whether it is
// live (ie. was not dropped from the tree)
func (n *Inode) removeRef(nlookup uint64, dropPersistence bool) (forgotten bool, live bool) {
	var lockme []*Inode
	var parents []parentData

	n.mu.Lock()
	if nlookup > 0 && dropPersistence {
		log.Panic("only one allowed")
	} else if nlookup > 0 {

		n.lookupCount -= nlookup
		n.changeCounter++
	} else if dropPersistence && n.persistent {
		n.persistent = false
		n.changeCounter++
	}

retry:
	for {
		lockme = append(lockme[:0], n)
		parents = parents[:0]
		nChange := n.changeCounter
		live = n.lookupCount > 0 || len(n.children) > 0 || n.persistent
		forgotten = n.lookupCount == 0
		for p := range n.parents {
			parents = append(parents, p)
			lockme = append(lockme, p.parent)
		}
		n.mu.Unlock()

		if live {
			return forgotten, live
		}

		lockNodes(lockme...)
		if n.changeCounter != nChange {
			unlockNodes(lockme...)
			// could avoid unlocking and relocking n here.
			n.mu.Lock()
			continue retry
		}

		for _, p := range parents {
			delete(p.parent.children, p.name)
			p.parent.changeCounter++
		}
		n.parents = map[parentData]struct{}{}
		n.changeCounter++

		if n.lookupCount != 0 {
			panic("lookupCount changed")
		}

		n.bridge.mu.Lock()
		delete(n.bridge.nodes, n.stableAttr.Ino)
		n.bridge.mu.Unlock()

		unlockNodes(lockme...)
		break
	}

	for _, p := range lockme {
		if p != n {
			p.removeRef(0, false)
		}
	}
	return forgotten, false
}

// GetChild returns a child node with the given name, or nil if the
// directory has no child by that name.
func (n *Inode) GetChild(name string) *Inode {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.children[name]
}

// AddChild adds a child to this node. If overwrite is false, fail if
// the destination already exists.
func (n *Inode) AddChild(name string, ch *Inode, overwrite bool) (success bool) {
	if len(name) == 0 {
		log.Panic("empty name for inode")
	}

retry:
	for {
		lockNode2(n, ch)
		prev, ok := n.children[name]
		parentCounter := n.changeCounter
		if !ok {
			n.children[name] = ch
			ch.parents[parentData{name, n}] = struct{}{}
			n.changeCounter++
			ch.changeCounter++
			unlockNode2(n, ch)
			return true
		}
		unlockNode2(n, ch)
		if !overwrite {
			return false
		}
		lockme := [3]*Inode{n, ch, prev}

		lockNodes(lockme[:]...)
		if parentCounter != n.changeCounter {
			unlockNodes(lockme[:]...)
			continue retry
		}

		delete(prev.parents, parentData{name, n})
		n.children[name] = ch
		ch.parents[parentData{name, n}] = struct{}{}
		n.changeCounter++
		ch.changeCounter++
		prev.changeCounter++
		unlockNodes(lockme[:]...)

		return true
	}
}

// Children returns the list of children of this directory Inode.
func (n *Inode) Children() map[string]*Inode {
	n.mu.Lock()
	defer n.mu.Unlock()
	r := make(map[string]*Inode, len(n.children))
	for k, v := range n.children {
		r[k] = v
	}
	return r
}

// Parents returns a parent of this Inode, or nil if this Inode is
// deleted or is the root
func (n *Inode) Parent() (string, *Inode) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for k := range n.parents {
		return k.name, k.parent
	}
	return "", nil
}

// RmAllChildren recursively drops a tree, forgetting all persistent
// nodes.
func (n *Inode) RmAllChildren() {
	for {
		chs := n.Children()
		if len(chs) == 0 {
			break
		}
		for nm, ch := range chs {
			ch.RmAllChildren()
			n.RmChild(nm)
		}
	}
	n.removeRef(0, true)
}

// RmChild removes multiple children.  Returns whether the removal
// succeeded and whether the node is still live afterward. The removal
// is transactional: it only succeeds if all names are children, and
// if they all were removed successfully.  If the removal was
// successful, and there are no children left, the node may be removed
// from the FS tree. In that case, RmChild returns live==false.
func (n *Inode) RmChild(names ...string) (success, live bool) {
	var lockme []*Inode

retry:
	for {
		n.mu.Lock()
		lockme = append(lockme[:0], n)
		nChange := n.changeCounter
		for _, nm := range names {
			ch := n.children[nm]
			if ch == nil {
				n.mu.Unlock()
				return false, true
			}
			lockme = append(lockme, ch)
		}
		n.mu.Unlock()

		lockNodes(lockme...)
		if n.changeCounter != nChange {
			unlockNodes(lockme...)
			// could avoid unlocking and relocking n here.
			n.mu.Lock()
			continue retry
		}

		for _, nm := range names {
			ch := n.children[nm]
			delete(n.children, nm)
			delete(ch.parents, parentData{nm, n})

			ch.changeCounter++
		}
		n.changeCounter++

		live = n.lookupCount > 0 || len(n.children) > 0 || n.persistent
		unlockNodes(lockme...)

		// removal successful
		break
	}

	if !live {
		_, live := n.removeRef(0, false)
		return true, live
	}

	return true, true
}

// MvChild executes a rename. If overwrite is set, a child at the
// destination will be overwritten, should it exist. It returns false
// if 'overwrite' is false, and the destination exists.
func (n *Inode) MvChild(old string, newParent *Inode, newName string, overwrite bool) bool {
	if len(newName) == 0 {
		log.Panicf("empty newName for MvChild")
	}

retry:
	for {
		lockNode2(n, newParent)
		counter1 := n.changeCounter
		counter2 := newParent.changeCounter

		oldChild := n.children[old]
		destChild := newParent.children[newName]
		unlockNode2(n, newParent)

		if destChild != nil && !overwrite {
			return false
		}

		lockNodes(n, newParent, oldChild, destChild)
		if counter2 != newParent.changeCounter || counter1 != n.changeCounter {
			unlockNodes(n, newParent, oldChild, destChild)
			continue retry
		}

		if oldChild != nil {
			delete(n.children, old)
			delete(oldChild.parents, parentData{old, n})
			n.changeCounter++
			oldChild.changeCounter++
		}

		if destChild != nil {
			// This can cause the child to be slated for
			// removal; see below
			delete(newParent.children, newName)
			delete(destChild.parents, parentData{newName, newParent})
			destChild.changeCounter++
			newParent.changeCounter++
		}

		if oldChild != nil {
			newParent.children[newName] = oldChild
			newParent.changeCounter++

			oldChild.parents[parentData{newName, newParent}] = struct{}{}
			oldChild.changeCounter++
		}

		unlockNodes(n, newParent, oldChild, destChild)

		if destChild != nil {
			destChild.removeRef(0, false)
		}
		return true
	}
}

// ExchangeChild swaps the entries at (n, oldName) and (newParent,
// newName).
func (n *Inode) ExchangeChild(oldName string, newParent *Inode, newName string) {
	oldParent := n
retry:
	for {
		lockNode2(oldParent, newParent)
		counter1 := oldParent.changeCounter
		counter2 := newParent.changeCounter

		oldChild := oldParent.children[oldName]
		destChild := newParent.children[newName]
		unlockNode2(oldParent, newParent)

		if destChild == oldChild {
			return
		}

		lockNodes(oldParent, newParent, oldChild, destChild)
		if counter2 != newParent.changeCounter || counter1 != oldParent.changeCounter {
			unlockNodes(oldParent, newParent, oldChild, destChild)
			continue retry
		}

		// Detach
		if oldChild != nil {
			delete(oldParent.children, oldName)
			delete(oldChild.parents, parentData{oldName, oldParent})
			oldParent.changeCounter++
			oldChild.changeCounter++
		}

		if destChild != nil {
			delete(newParent.children, newName)
			delete(destChild.parents, parentData{newName, newParent})
			destChild.changeCounter++
			newParent.changeCounter++
		}

		// Attach
		if oldChild != nil {
			newParent.children[newName] = oldChild
			newParent.changeCounter++

			oldChild.parents[parentData{newName, newParent}] = struct{}{}
			oldChild.changeCounter++
		}

		if destChild != nil {
			oldParent.children[oldName] = oldChild
			oldParent.changeCounter++

			destChild.parents[parentData{oldName, oldParent}] = struct{}{}
			destChild.changeCounter++
		}
		unlockNodes(oldParent, newParent, oldChild, destChild)
		return
	}
}

// NotifyEntry notifies the kernel that data for a (directory, name)
// tuple should be invalidated. On next access, a LOOKUP operation
// will be started.
func (n *Inode) NotifyEntry(name string) syscall.Errno {
	status := n.bridge.server.EntryNotify(n.stableAttr.Ino, name)
	return syscall.Errno(status)
}

// NotifyDelete notifies the kernel that the given inode was removed
// from this directory as entry under the given name. It is equivalent
// to NotifyEntry, but also sends an event to inotify watchers.
func (n *Inode) NotifyDelete(name string, child *Inode) syscall.Errno {
	// XXX arg ordering?
	return syscall.Errno(n.bridge.server.DeleteNotify(n.stableAttr.Ino, child.stableAttr.Ino, name))

}

// NotifyContent notifies the kernel that content under the given
// inode should be flushed from buffers.
func (n *Inode) NotifyContent(off, sz int64) syscall.Errno {
	// XXX how does this work for directories?
	return syscall.Errno(n.bridge.server.InodeNotify(n.stableAttr.Ino, off, sz))
}

// WriteCache stores data in the kernel cache.
func (n *Inode) WriteCache(offset int64, data []byte) syscall.Errno {
	return syscall.Errno(n.bridge.server.InodeNotifyStoreCache(n.stableAttr.Ino, offset, data))
}

// ReadCache reads data from the kernel cache.
func (n *Inode) ReadCache(offset int64, dest []byte) (count int, errno syscall.Errno) {
	c, s := n.bridge.server.InodeRetrieveCache(n.stableAttr.Ino, offset, dest)
	return c, syscall.Errno(s)
}
