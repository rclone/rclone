// Package dircache provides a simple cache for caching directory ID
// to path lookups and the inverse.
package dircache

// _methods are called without the lock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
)

// DirCache caches paths to directory IDs and vice versa
type DirCache struct {
	cacheMu  sync.RWMutex // protects cache and invCache
	cache    map[string]string
	invCache map[string]string

	mu           sync.Mutex // protects the below
	fs           DirCacher  // Interface to find and make directories
	trueRootID   string     // ID of the absolute root
	root         string     // the path the cache is rooted on
	rootID       string     // ID of the root directory
	rootParentID string     // ID of the root's parent directory
	foundRoot    bool       // Whether we have found the root or not
}

// DirCacher describes an interface for doing the low level directory work
//
// This should be implemented by the backend and will be called by the
// dircache package when appropriate.
type DirCacher interface {
	FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error)
	CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error)
}

// New makes a DirCache
//
// This is created with the true root ID and the root path.
//
// In order to use the cache FindRoot() must be called on it without
// error. This isn't done at initialization as it isn't known whether
// the root and intermediate directories need to be created or not.
//
// Most of the utility functions will call FindRoot() on the caller's
// behalf with the create flag passed in.
//
// The cache is safe for concurrent use
func New(root string, trueRootID string, fs DirCacher) *DirCache {
	d := &DirCache{
		trueRootID: trueRootID,
		root:       root,
		fs:         fs,
	}
	d.Flush()
	d.ResetRoot()
	return d
}

// String returns the directory cache in string form for debugging
func (dc *DirCache) String() string {
	dc.cacheMu.RLock()
	defer dc.cacheMu.RUnlock()
	var buf bytes.Buffer
	_, _ = buf.WriteString("DirCache{\n")
	_, _ = fmt.Fprintf(&buf, "\ttrueRootID: %q,\n", dc.trueRootID)
	_, _ = fmt.Fprintf(&buf, "\troot: %q,\n", dc.root)
	_, _ = fmt.Fprintf(&buf, "\trootID: %q,\n", dc.rootID)
	_, _ = fmt.Fprintf(&buf, "\trootParentID: %q,\n", dc.rootParentID)
	_, _ = fmt.Fprintf(&buf, "\tfoundRoot: %v,\n", dc.foundRoot)
	_, _ = buf.WriteString("\tcache: {\n")
	for k, v := range dc.cache {
		_, _ = fmt.Fprintf(&buf, "\t\t%q: %q,\n", k, v)
	}
	_, _ = buf.WriteString("\t},\n")
	_, _ = buf.WriteString("\tinvCache: {\n")
	for k, v := range dc.invCache {
		_, _ = fmt.Fprintf(&buf, "\t\t%q: %q,\n", k, v)
	}
	_, _ = buf.WriteString("\t},\n")
	_, _ = buf.WriteString("}\n")
	return buf.String()
}

// Get a directory ID given a path
//
// Returns the ID and a boolean as to whether it was found or not in
// the cache.
func (dc *DirCache) Get(path string) (id string, ok bool) {
	dc.cacheMu.RLock()
	id, ok = dc.cache[path]
	dc.cacheMu.RUnlock()
	return id, ok
}

// GetInv gets a path given a directory ID
//
// Returns the path and a boolean as to whether it was found or not in
// the cache.
func (dc *DirCache) GetInv(id string) (path string, ok bool) {
	dc.cacheMu.RLock()
	path, ok = dc.invCache[id]
	dc.cacheMu.RUnlock()
	return path, ok
}

// Put a (path, directory ID) pair into the cache
func (dc *DirCache) Put(path, id string) {
	dc.cacheMu.Lock()
	dc.cache[path] = id
	dc.invCache[id] = path
	dc.cacheMu.Unlock()
}

// Flush the cache of all data
func (dc *DirCache) Flush() {
	dc.cacheMu.Lock()
	dc.cache = make(map[string]string)
	dc.invCache = make(map[string]string)
	dc.cacheMu.Unlock()
}

// SetRootIDAlias sets the rootID to that passed in. This assumes that
// the new ID is just an alias for the old ID so does not flush
// anything.
//
// This should be called from FindLeaf (and only from FindLeaf) if it
// is discovered that the root ID is incorrect. For example some
// backends use "0" as a root ID, but it has a real ID which is needed
// for some operations.
func (dc *DirCache) SetRootIDAlias(rootID string) {
	// No locking as this is called from FindLeaf
	dc.rootID = rootID
	dc.Put("", dc.rootID)
}

// FlushDir flushes the map of all data starting with the path
// dir.
//
// If dir is empty string then this is equivalent to calling ResetRoot
func (dc *DirCache) FlushDir(dir string) {
	if dir == "" {
		dc.ResetRoot()
		return
	}
	dc.cacheMu.Lock()

	// Delete the root dir
	ID, ok := dc.cache[dir]
	if ok {
		delete(dc.cache, dir)
		delete(dc.invCache, ID)
	}

	// And any sub directories
	dir += "/"
	for key, ID := range dc.cache {
		if strings.HasPrefix(key, dir) {
			delete(dc.cache, key)
			delete(dc.invCache, ID)
		}
	}

	dc.cacheMu.Unlock()
}

// SplitPath splits a path into directory, leaf
//
// Path shouldn't start or end with a /
//
// If there are no slashes then directory will be "" and leaf = path
func SplitPath(path string) (directory, leaf string) {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash >= 0 {
		directory = path[:lastSlash]
		leaf = path[lastSlash+1:]
	} else {
		directory = ""
		leaf = path
	}
	return
}

// FindDir finds the directory passed in returning the directory ID
// starting from pathID
//
// Path shouldn't start or end with a /
//
// If create is set it will make the directory if not found.
//
// It will call FindRoot if it hasn't been called already
func (dc *DirCache) FindDir(ctx context.Context, path string, create bool) (pathID string, err error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	err = dc._findRoot(ctx, create)
	if err != nil {
		return "", err
	}
	return dc._findDir(ctx, path, create)
}

// Unlocked findDir
//
// Call with a lock on mu
func (dc *DirCache) _findDir(ctx context.Context, path string, create bool) (pathID string, err error) {
	// If it is the root, then return it
	if path == "" {
		return dc.rootID, nil
	}

	// If it is in the cache then return it
	pathID, ok := dc.Get(path)
	if ok {
		return pathID, nil
	}

	// Split the path into directory, leaf
	directory, leaf := SplitPath(path)

	// Recurse and find pathID for parent directory
	parentPathID, err := dc._findDir(ctx, directory, create)
	if err != nil {
		return "", err

	}

	// Find the leaf in parentPathID
	pathID, found, err := dc.fs.FindLeaf(ctx, parentPathID, leaf)
	if err != nil {
		return "", err
	}

	// If not found create the directory if required or return an error
	if !found {
		if create {
			pathID, err = dc.fs.CreateDir(ctx, parentPathID, leaf)
			if err != nil {
				return "", fmt.Errorf("failed to make directory: %w", err)
			}
		} else {
			return "", fs.ErrorDirNotFound
		}
	}

	// Store the leaf directory in the cache
	dc.Put(path, pathID)

	// fmt.Println("Dir", path, "is", pathID)
	return pathID, nil
}

// FindPath finds the leaf and directoryID from a path
//
// If called with path == "" then it will return the ID of the parent
// directory of the root and the leaf name of the root in that
// directory. Note that it won't create the root directory in this
// case even if create is true.
//
// If create is set parent directories will be created if they don't exist
//
// It will call FindRoot if it hasn't been called already
func (dc *DirCache) FindPath(ctx context.Context, path string, create bool) (leaf, directoryID string, err error) {
	if path == "" {
		_, leaf = SplitPath(dc.root)
		directoryID, err = dc.RootParentID(ctx, create)
	} else {
		var directory string
		directory, leaf = SplitPath(path)
		directoryID, err = dc.FindDir(ctx, directory, create)
	}
	return leaf, directoryID, err
}

// FindRoot finds the root directory if not already found
//
// If successful this changes the root of the cache from the true root
// to the root specified by the path passed into New.
//
// Resets the root directory.
//
// If create is set it will make the directory if not found
func (dc *DirCache) FindRoot(ctx context.Context, create bool) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc._findRoot(ctx, create)
}

// _findRoot finds the root directory if not already found
//
// Resets the root directory.
//
// If create is set it will make the directory if not found.
//
// Call with mu held
func (dc *DirCache) _findRoot(ctx context.Context, create bool) error {
	if dc.foundRoot {
		return nil
	}
	rootID, err := dc._findDir(ctx, dc.root, create)
	if err != nil {
		return err
	}
	dc.foundRoot = true
	dc.rootID = rootID

	// Find the parent of the root while we still have the root
	// directory tree cached
	rootParentPath, _ := SplitPath(dc.root)
	dc.rootParentID, _ = dc.Get(rootParentPath)

	// Reset the tree based on dc.root
	dc.Flush()
	// Put the root directory in
	dc.Put("", dc.rootID)
	return nil
}

// FoundRoot returns whether the root directory has been found yet
func (dc *DirCache) FoundRoot() bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.foundRoot
}

// RootID returns the ID of the root directory
//
// If create is set it will make the root directory if not found
func (dc *DirCache) RootID(ctx context.Context, create bool) (ID string, err error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	err = dc._findRoot(ctx, create)
	if err != nil {
		return "", err
	}
	return dc.rootID, nil
}

// RootParentID returns the ID of the parent of the root directory
//
// If create is set it will make the root parent directory if not found (but not the root)
func (dc *DirCache) RootParentID(ctx context.Context, create bool) (ID string, err error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if !dc.foundRoot {
		if dc.root == "" {
			return "", errors.New("is root directory")
		}
		// Find the rootParentID without creating the root
		rootParent, _ := SplitPath(dc.root)
		rootParentID, err := dc._findDir(ctx, rootParent, create)
		if err != nil {
			return "", err
		}
		dc.rootParentID = rootParentID
	} else if dc.rootID == dc.trueRootID {
		return "", errors.New("is root directory")
	}
	if dc.rootParentID == "" {
		return "", errors.New("internal error: didn't find rootParentID")
	}
	return dc.rootParentID, nil
}

// ResetRoot resets the root directory to the absolute root and clears
// the DirCache
func (dc *DirCache) ResetRoot() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.foundRoot = false
	dc.Flush()

	// Put the true root in
	dc.rootID = dc.trueRootID

	// Put the root directory in
	dc.Put("", dc.rootID)
}

// DirMove prepares to move the directory (srcDC, srcRoot, srcRemote)
// into the directory (dc, dstRoot, dstRemote)
//
// It does all the checking, creates intermediate directories and
// returns leafs and IDs ready for the move.
//
// This returns:
//
// - srcID - ID of the source directory
// - srcDirectoryID - ID of the parent of the source directory
// - srcLeaf - leaf name of the source directory
// - dstDirectoryID - ID of the parent of the destination directory
// - dstLeaf - leaf name of the destination directory
//
// These should be used to do the actual move then
// srcDC.FlushDir(srcRemote) should be called.
func (dc *DirCache) DirMove(
	ctx context.Context, srcDC *DirCache, srcRoot, srcRemote, dstRoot, dstRemote string) (srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf string, err error) {
	var (
		dstDC   = dc
		srcPath = path.Join(srcRoot, srcRemote)
		dstPath = path.Join(dstRoot, dstRemote)
	)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		// fs.Debugf(src, "DirMove error: Can't move root")
		err = errors.New("can't move root directory")
		return
	}

	// Find ID of dst parent, creating subdirs if necessary
	dstLeaf, dstDirectoryID, err = dstDC.FindPath(ctx, dstRemote, true)
	if err != nil {
		return
	}

	// Check destination does not exist
	_, err = dstDC.FindDir(ctx, dstRemote, false)
	if err == fs.ErrorDirNotFound {
		// OK
	} else if err != nil {
		return
	} else {
		err = fs.ErrorDirExists
		return
	}

	// Find ID of src parent
	srcLeaf, srcDirectoryID, err = srcDC.FindPath(ctx, srcRemote, false)
	if err != nil {
		return
	}

	// Find ID of src
	srcID, err = srcDC.FindDir(ctx, srcRemote, false)
	if err != nil {
		return
	}

	return
}
