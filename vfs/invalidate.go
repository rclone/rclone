package vfs

import (
	"path"
	"strings"
	"sync"
)

// InvalidateKernelCacheHook is called by the VFS to tell a mount to drop
// the kernel's cached directory entry for name in parent, along with the
// attributes and page cache of the node it refers to. node is nil when
// the entry is no longer in the VFS cache; only the directory entry can
// be dropped then.
//
// Hooks run on a dedicated goroutine, never on a FUSE handler, so a
// synchronous kernel notify is safe.
type InvalidateKernelCacheHook func(parent Node, name string, node Node)

// pendingEntry identifies one kernel directory entry, so a storm of
// invalidations coalesces to one notify per entry.
type pendingEntry struct {
	parent Node
	name   string
}

// kernelCacheInvalidator coalesces and dispatches kernel cache
// invalidations to registered hooks. The zero value is ready for use.
type kernelCacheInvalidator struct {
	mu         sync.Mutex // guards the fields below
	hooks      map[int]InvalidateKernelCacheHook
	nextID     int
	pending    map[pendingEntry]Node // entry to invalidate → node (nil if not cached)
	wake       chan struct{}
	running    bool       // whether the dispatcher goroutine is running
	dispatchMu sync.Mutex // held while one entry's hooks run; a barrier for unsubscribe
}

// AddInvalidateKernelCacheHook registers fn and returns a function to
// unregister it. A VFS shared by several mounts (see New) calls every
// hook. The returned remove function does not return while fn is
// running, so the caller can tear down its mount safely afterwards.
func (vfs *VFS) AddInvalidateKernelCacheHook(fn InvalidateKernelCacheHook) (remove func()) {
	k := &vfs.kernelCacheInvalidator
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.hooks == nil {
		k.hooks = map[int]InvalidateKernelCacheHook{}
		k.pending = map[pendingEntry]Node{}
		k.wake = make(chan struct{}, 1)
	}
	id := k.nextID
	k.nextID++
	k.hooks[id] = fn
	if !k.running && vfs.ctx.Err() == nil {
		k.running = true
		go vfs.invalidateKernelCacheDispatcher()
	}
	return func() {
		k.mu.Lock()
		delete(k.hooks, id)
		k.mu.Unlock()
		// Barrier: wait for any in-flight dispatch to finish.
		k.dispatchMu.Lock()
		k.dispatchMu.Unlock() //nolint:staticcheck // barrier, not an empty critical section
	}
}

// HasInvalidateKernelCacheHooks reports whether any hook is registered.
func (vfs *VFS) HasInvalidateKernelCacheHooks() bool {
	vfs.kernelCacheInvalidator.mu.Lock()
	defer vfs.kernelCacheInvalidator.mu.Unlock()
	return len(vfs.kernelCacheInvalidator.hooks) > 0
}

// Queue an invalidation of the directory entry for name in parent, referring to
// node (which may be nil if the node is no longer cached).
func (vfs *VFS) invalidateKernelCacheForEntry(parent Node, name string, node Node) {
	if parent == nil || name == "" {
		return
	}
	k := &vfs.kernelCacheInvalidator
	k.mu.Lock()
	if len(k.hooks) == 0 || !k.running {
		k.mu.Unlock()
		return
	}
	key := pendingEntry{parent: parent, name: name}
	if existing := k.pending[key]; existing == nil {
		k.pending[key] = node
	}
	k.mu.Unlock()
	select {
	case k.wake <- struct{}{}:
	default:
	}
}

// Queue an invalidation of absPath's directory entry. The entry is queued even
// if the node itself is no longer in the VFS cache (the kernel may still hold
// it), as long as its parent directory is.
func (vfs *VFS) invalidateKernelCacheForPath(absPath string) {
	if !vfs.HasInvalidateKernelCacheHooks() {
		return
	}
	absPath = strings.Trim(absPath, "/")
	if absPath == "" {
		return // the root has no parent entry to invalidate
	}
	parentPath, name := path.Split(absPath)
	parent := vfs.root.cachedDir(parentPath)
	if parent == nil {
		return
	}
	vfs.invalidateKernelCacheForEntry(parent, name, parent.cachedNode(name))
}

func (vfs *VFS) invalidateKernelCacheDispatcher() {
	k := &vfs.kernelCacheInvalidator
	defer func() {
		k.mu.Lock()
		k.running = false
		k.pending = map[pendingEntry]Node{} // release Node references
		k.mu.Unlock()
	}()
	for {
		select {
		case <-vfs.ctx.Done():
			return
		case <-k.wake:
		}
		k.mu.Lock()
		pending := k.pending
		k.pending = make(map[pendingEntry]Node, len(pending))
		k.mu.Unlock()
		for entry, node := range pending {
			if vfs.ctx.Err() != nil {
				return
			}
			// dispatchMu is held per entry, not per batch, so an unsubscribing
			// mount waits for at most one entry's hooks.
			k.dispatchMu.Lock()
			k.mu.Lock()
			hooks := make([]InvalidateKernelCacheHook, 0, len(k.hooks))
			for _, fn := range k.hooks {
				hooks = append(hooks, fn)
			}
			k.mu.Unlock()
			for _, fn := range hooks {
				fn(entry.parent, entry.name, node)
			}
			k.dispatchMu.Unlock()
		}
	}
}
