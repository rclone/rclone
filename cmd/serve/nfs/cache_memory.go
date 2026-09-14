//go:build unix

package nfs

import (
	"path"
	"strings"
	"sync"

	billy "github.com/go-git/go-billy/v5"
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
)

// memoryHandler is the in-memory NFS file handle cache.
//
// It differs from go-nfs's CachingHandler in one way: a file or directory keeps
// its handle when it is renamed. NFS clients hold on to the handle of a
// directory they have open, and go-nfs invalidates that handle straight after a
// rename, so the next operation inside the renamed directory fails with a stale
// handle. On macOS that is Finder's "New Folder" inside a folder that was just
// created and renamed from "untitled folder" (error -8060).
//
// FS.Rename calls renamed, which re-points the renamed entry and every entry
// below it to the new path. The InvalidateHandle go-nfs sends for the renamed
// entry is then ignored; InvalidateHandle for a removed file works as before.
type memoryHandler struct {
	mu      sync.Mutex
	handles *lru.Cache[uuid.UUID, memoryEntry]
	byPath  map[string]uuid.UUID
	kept    map[uuid.UUID]struct{} // re-pointed by a rename, awaiting go-nfs's InvalidateHandle
	limit   int
}

type memoryEntry struct {
	f   billy.Filesystem
	key string // f.Join(path...)
	p   []string
}

func newMemoryHandler(limit int) *memoryHandler {
	c := &memoryHandler{
		byPath: make(map[string]uuid.UUID),
		kept:   make(map[uuid.UUID]struct{}),
		limit:  limit,
	}
	// the eviction callback runs inside Add/Remove, which are only called with mu held
	c.handles, _ = lru.NewWithEvict(limit, func(id uuid.UUID, e memoryEntry) {
		if c.byPath[e.key] == id {
			delete(c.byPath, e.key)
		}
		delete(c.kept, id)
	})
	return c
}

// ToHandle takes a file and represents it with an opaque handle to reference it.
func (c *memoryHandler) ToHandle(f billy.Filesystem, splitPath []string) []byte {
	key := f.Join(splitPath...)
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.byPath[key]; ok {
		if _, ok := c.handles.Get(id); ok {
			return id[:]
		}
	}
	id := uuid.New()
	c.handles.Add(id, memoryEntry{f: f, key: key, p: append([]string(nil), splitPath...)})
	c.byPath[key] = id
	return id[:]
}

// FromHandle converts from an opaque handle to the file it represents
func (c *memoryHandler) FromHandle(fh []byte) (billy.Filesystem, []string, error) {
	id, err := uuid.FromBytes(fh)
	if err != nil {
		return nil, nil, errStaleHandle
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.handles.Get(id)
	if !ok {
		return nil, nil, errStaleHandle
	}
	return e.f, append([]string(nil), e.p...), nil
}

// InvalidateHandle invalidates the handle passed - used on rename and delete
func (c *memoryHandler) InvalidateHandle(f billy.Filesystem, fh []byte) error {
	id, err := uuid.FromBytes(fh)
	if err != nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.kept[id]; ok {
		delete(c.kept, id)
		return nil
	}
	c.handles.Remove(id)
	return nil
}

// HandleLimit exports how many file handles can be safely stored by this cache.
func (c *memoryHandler) HandleLimit() int {
	return c.limit
}

// renamed re-points the handles of oldpath, and of everything below it, to
// newpath. Handles of anything newpath replaced go stale. Paths are absolute
// within the VFS, as FS.Rename passes them.
func (c *memoryHandler) renamed(oldpath, newpath string) {
	oldKey, newKey := cacheKey(oldpath), cacheKey(newpath)
	if oldKey == newKey {
		return
	}
	under := func(key, dir string) bool { return key == dir || strings.HasPrefix(key, dir+"/") }
	c.mu.Lock()
	defer c.mu.Unlock()
	var moving, replaced []uuid.UUID
	for key, id := range c.byPath {
		switch {
		case under(key, oldKey):
			moving = append(moving, id)
		case under(key, newKey):
			replaced = append(replaced, id)
		}
	}
	for _, id := range replaced {
		c.handles.Remove(id)
	}
	for _, id := range moving {
		e, ok := c.handles.Peek(id)
		if !ok {
			continue
		}
		if c.byPath[e.key] == id {
			delete(c.byPath, e.key)
		}
		if e.key == oldKey {
			c.kept[id] = struct{}{}
		}
		e.key = newKey + e.key[len(oldKey):]
		e.p = splitKey(e.key)
		c.handles.Add(id, e)
		c.byPath[e.key] = id
	}
}

// cacheKey turns an FS path into the form f.Join gives for the same file
func cacheKey(p string) string {
	p = strings.Trim(path.Clean("/"+p), "/")
	return p
}

// splitKey is the inverse of f.Join for a cache key
func splitKey(key string) []string {
	if key == "" {
		return []string{}
	}
	return strings.Split(key, "/")
}
