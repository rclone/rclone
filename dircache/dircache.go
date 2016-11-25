// Package dircache provides a simple cache for caching directory to path lookups
package dircache

// _methods are called without the lock

import (
	"log"
	"strings"
	"sync"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// DirCache caches paths to directory IDs and vice versa
type DirCache struct {
	cacheMu      sync.RWMutex
	cache        map[string]string
	invCache     map[string]string
	mu           sync.Mutex
	fs           DirCacher // Interface to find and make stuff
	trueRootID   string    // ID of the absolute root
	root         string    // the path we are working on
	rootID       string    // ID of the root directory
	rootParentID string    // ID of the root's parent directory
	foundRoot    bool      // Whether we have found the root or not
}

// DirCacher describes an interface for doing the low level directory work
type DirCacher interface {
	FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error)
	CreateDir(pathID, leaf string) (newID string, err error)
}

// New makes a DirCache
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

// Get an ID given a path
func (dc *DirCache) Get(path string) (id string, ok bool) {
	dc.cacheMu.RLock()
	id, ok = dc.cache[path]
	dc.cacheMu.RUnlock()
	return
}

// GetInv gets a path given an ID
func (dc *DirCache) GetInv(id string) (path string, ok bool) {
	dc.cacheMu.RLock()
	path, ok = dc.invCache[id]
	dc.cacheMu.RUnlock()
	return
}

// Put a path, id into the map
func (dc *DirCache) Put(path, id string) {
	dc.cacheMu.Lock()
	dc.cache[path] = id
	dc.invCache[id] = path
	dc.cacheMu.Unlock()
}

// Flush the map of all data
func (dc *DirCache) Flush() {
	dc.cacheMu.Lock()
	dc.cache = make(map[string]string)
	dc.invCache = make(map[string]string)
	dc.cacheMu.Unlock()
}

// FlushDir flushes the map of all data starting with dir
//
// If dir is empty then this is equivalent to calling ResetRoot
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
// If create is set it will make the directory if not found
//
// Algorithm:
//  Look in the cache for the path, if found return the pathID
//  If not found strip the last path off the path and recurse
//  Now have a parent directory id, so look in the parent for self and return it
func (dc *DirCache) FindDir(path string, create bool) (pathID string, err error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc._findDir(path, create)
}

// Look for the root and in the cache - safe to call without the mu
func (dc *DirCache) _findDirInCache(path string) string {
	// fmt.Println("Finding",path,"create",create,"cache",cache)
	// If it is the root, then return it
	if path == "" {
		// fmt.Println("Root")
		return dc.rootID
	}

	// If it is in the cache then return it
	pathID, ok := dc.Get(path)
	if ok {
		// fmt.Println("Cache hit on", path)
		return pathID
	}

	return ""
}

// Unlocked findDir - must have mu
func (dc *DirCache) _findDir(path string, create bool) (pathID string, err error) {
	pathID = dc._findDirInCache(path)
	if pathID != "" {
		return pathID, nil
	}

	// Split the path into directory, leaf
	directory, leaf := SplitPath(path)

	// Recurse and find pathID for parent directory
	parentPathID, err := dc._findDir(directory, create)
	if err != nil {
		return "", err

	}

	// Find the leaf in parentPathID
	pathID, found, err := dc.fs.FindLeaf(parentPathID, leaf)
	if err != nil {
		return "", err
	}

	// If not found create the directory if required or return an error
	if !found {
		if create {
			pathID, err = dc.fs.CreateDir(parentPathID, leaf)
			if err != nil {
				return "", errors.Wrap(err, "failed to make directory")
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
// If create is set parent directories will be created if they don't exist
func (dc *DirCache) FindPath(path string, create bool) (leaf, directoryID string, err error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	directory, leaf := SplitPath(path)
	directoryID, err = dc._findDir(directory, create)
	return
}

// FindRoot finds the root directory if not already found
//
// Resets the root directory
//
// If create is set it will make the directory if not found
func (dc *DirCache) FindRoot(create bool) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if dc.foundRoot {
		return nil
	}
	rootID, err := dc._findDir(dc.root, create)
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
//
// Call this from FindLeaf or CreateDir only
func (dc *DirCache) FoundRoot() bool {
	return dc.foundRoot
}

// RootID returns the ID of the root directory
//
// This should be called after FindRoot
func (dc *DirCache) RootID() string {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if !dc.foundRoot {
		log.Fatalf("Internal Error: RootID() called before FindRoot")
	}
	return dc.rootID
}

// RootParentID returns the ID of the parent of the root directory
//
// This should be called after FindRoot
func (dc *DirCache) RootParentID() (string, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	if !dc.foundRoot {
		return "", errors.New("internal error: RootID() called before FindRoot")
	}
	if dc.rootParentID == "" {
		return "", errors.New("internal error: didn't find rootParentID")
	}
	if dc.rootID == dc.trueRootID {
		return "", errors.New("is root directory")
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
