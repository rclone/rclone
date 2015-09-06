// dircache provides a simple cache for caching directory to path lookups
package dircache

// _methods are called without the lock

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// DirCache caches paths to directory IDs and vice versa
type DirCache struct {
	mu           sync.RWMutex
	cache        map[string]string
	invCache     map[string]string
	fs           DirCacher // Interface to find and make stuff
	trueRootID   string    // ID of the absolute root
	root         string    // the path we are working on
	rootID       string    // ID of the root directory
	rootParentID string    // ID of the root's parent directory
	foundRoot    bool      // Whether we have found the root or not
}

// DirCache describes an interface for doing the low level directory work
type DirCacher interface {
	FindLeaf(pathID, leaf string) (pathIDOut string, found bool, err error)
	CreateDir(pathID, leaf string) (newID string, err error)
}

// Make a new DirCache
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

// _get an ID given a path - without lock
func (dc *DirCache) _get(path string) (id string, ok bool) {
	id, ok = dc.cache[path]
	return
}

// Gets an ID given a path
func (dc *DirCache) Get(path string) (id string, ok bool) {
	dc.mu.RLock()
	id, ok = dc._get(path)
	dc.mu.RUnlock()
	return
}

// GetInv gets a path given an ID
func (dc *DirCache) GetInv(path string) (id string, ok bool) {
	dc.mu.RLock()
	id, ok = dc.invCache[path]
	dc.mu.RUnlock()
	return
}

// _put a path, id into the map without lock
func (dc *DirCache) _put(path, id string) {
	dc.cache[path] = id
	dc.invCache[id] = path
}

// Put a path, id into the map
func (dc *DirCache) Put(path, id string) {
	dc.mu.Lock()
	dc._put(path, id)
	dc.mu.Unlock()
}

// _flush the map of all data without lock
func (dc *DirCache) _flush() {
	dc.cache = make(map[string]string)
	dc.invCache = make(map[string]string)
}

// Flush the map of all data
func (dc *DirCache) Flush() {
	dc.mu.Lock()
	dc._flush()
	dc.mu.Unlock()
}

// Splits a path into directory, leaf
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

// Finds the directory passed in returning the directory ID starting from pathID
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
	dc.mu.RLock()
	defer dc.mu.RUnlock()
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
	pathID, ok := dc._get(path)
	if ok {
		// fmt.Println("Cache hit on", path)
		return pathID
	}

	return ""
}

// Unlocked findDir - must have mu
func (dc *DirCache) _findDir(path string, create bool) (pathID string, err error) {
	// if !dc.foundRoot {
	// 	return "", fmt.Errorf("FindDir called before FindRoot")
	// }

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
				return "", fmt.Errorf("Failed to make directory: %v", err)
			}
		} else {
			return "", fmt.Errorf("Couldn't find directory: %q", path)
		}
	}

	// Store the leaf directory in the cache
	dc._put(path, pathID)

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
	if err != nil {
		if create {
			err = fmt.Errorf("Couldn't find or make directory %q: %s", directory, err)
		} else {
			err = fmt.Errorf("Couldn't find directory %q: %s", directory, err)
		}
	}
	return
}

// Finds the root directory if not already found
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
	dc.foundRoot = true
	rootID, err := dc._findDir(dc.root, create)
	if err != nil {
		dc.foundRoot = false
		return err
	}
	dc.rootID = rootID

	// Find the parent of the root while we still have the root
	// directory tree cached
	rootParentPath, _ := SplitPath(dc.root)
	dc.rootParentID, _ = dc._get(rootParentPath)

	// Reset the tree based on dc.root
	dc._flush()
	// Put the root directory in
	dc._put("", dc.rootID)
	return nil
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
		return "", fmt.Errorf("Internal Error: RootID() called before FindRoot")
	}
	if dc.rootParentID == "" {
		return "", fmt.Errorf("Internal Error: Didn't find rootParentID")
	}
	if dc.rootID == dc.trueRootID {
		return "", fmt.Errorf("Is root directory")
	}
	return dc.rootParentID, nil
}

// Resets the root directory to the absolute root and clears the DirCache
func (dc *DirCache) ResetRoot() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.foundRoot = false
	dc._flush()

	// Put the true root in
	dc.rootID = dc.trueRootID

	// Put the root directory in
	dc._put("", dc.rootID)
}
