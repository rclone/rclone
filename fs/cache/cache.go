// Package cache implements the Fs cache
package cache

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/lib/cache"
)

var (
	once           sync.Once // creation
	c              *cache.Cache
	mu             sync.Mutex            // mutex to protect remap
	remap          = map[string]string{} // map user supplied names to canonical names - [fsString]canonicalName
	childParentMap = map[string]string{} // tracks a one-to-many relationship between parent dirs and their direct children files - [child]parent
)

// Create the cache just once
func createOnFirstUse() {
	once.Do(func() {
		ci := fs.GetConfig(context.Background())
		c = cache.New()
		c.SetExpireDuration(time.Duration(ci.FsCacheExpireDuration))
		c.SetExpireInterval(time.Duration(ci.FsCacheExpireInterval))
		c.SetFinalizer(func(value any) {
			if s, ok := value.(fs.Shutdowner); ok {
				_ = fs.CountError(context.Background(), s.Shutdown(context.Background()))
			}
		})
	})
}

// Canonicalize looks up fsString in the mapping from user supplied
// names to canonical names and return the canonical form
func Canonicalize(fsString string) string {
	createOnFirstUse()
	mu.Lock()
	canonicalName, ok := remap[fsString]
	mu.Unlock()
	if !ok {
		return fsString
	}
	fs.Debugf(nil, "fs cache: switching user supplied name %q for canonical name %q", fsString, canonicalName)
	return canonicalName
}

// Put in a mapping from fsString => canonicalName if they are different
func addMapping(fsString, canonicalName string) {
	if canonicalName == fsString {
		return
	}
	mu.Lock()
	remap[fsString] = canonicalName
	mu.Unlock()
}

// addChild tracks known file (child) to directory (parent) relationships.
// Note that the canonicalName of a child will always equal that of its parent,
// but not everything with an equal canonicalName is a child.
// It could be an alias or overridden version of a directory.
func addChild(child, parent string) {
	if child == parent {
		return
	}
	mu.Lock()
	childParentMap[child] = parent
	mu.Unlock()
}

// returns true if name is definitely known to be a child (i.e. a file, not a dir).
// returns false if name is a dir or if we don't know.
func isChild(child string) bool {
	mu.Lock()
	_, found := childParentMap[child]
	mu.Unlock()
	return found
}

// ensures that we return fs.ErrorIsFile when necessary
func getError(fsString string, err error) error {
	if err != nil && err != fs.ErrorIsFile {
		return err
	}
	if isChild(fsString) {
		return fs.ErrorIsFile
	}
	return nil
}

// GetFn gets an fs.Fs named fsString either from the cache or creates
// it afresh with the create function
func GetFn(ctx context.Context, fsString string, create func(ctx context.Context, fsString string) (fs.Fs, error)) (f fs.Fs, err error) {
	createOnFirstUse()
	canonicalFsString := Canonicalize(fsString)
	created := false
	value, err := c.Get(canonicalFsString, func(canonicalFsString string) (f any, ok bool, err error) {
		f, err = create(ctx, fsString) // always create the backend with the original non-canonicalised string
		ok = err == nil || err == fs.ErrorIsFile
		created = ok
		return f, ok, err
	})
	f, ok := value.(fs.Fs)
	if err != nil && err != fs.ErrorIsFile {
		if ok {
			return f, err // for possible future uses of PutErr
		}
		return nil, err
	}
	// Check we stored the Fs at the canonical name
	if created {
		canonicalName := fs.ConfigString(f)
		if canonicalName != canonicalFsString {
			if err == nil { // it's a dir
				fs.Debugf(nil, "fs cache: renaming cache item %q to be canonical %q", canonicalFsString, canonicalName)
				value, found := c.Rename(canonicalFsString, canonicalName)
				if found {
					f = value.(fs.Fs)
				}
				addMapping(canonicalFsString, canonicalName)
			} else { // it's a file
				// the fs we cache is always the file's parent, never the file,
				// but we use the childParentMap to return the correct error status based on the fsString passed in.
				fs.Debugf(nil, "fs cache: renaming child cache item %q to be canonical for parent %q", canonicalFsString, canonicalName)
				value, found := c.Rename(canonicalFsString, canonicalName) // rename the file entry to parent
				if found {
					f = value.(fs.Fs) // if parent already exists, use it
				}
				Put(canonicalName, f)                        // force err == nil for the cache
				addMapping(canonicalFsString, canonicalName) // note the fsString-canonicalName connection for future lookups
				addChild(fsString, canonicalName)            // note the file-directory connection for future lookups
			}
		}
	}
	return f, getError(fsString, err) // ensure fs.ErrorIsFile is returned when necessary
}

// Pin f into the cache until Unpin is called
func Pin(f fs.Fs) {
	createOnFirstUse()
	c.Pin(fs.ConfigString(f))
}

// PinUntilFinalized pins f into the cache until x is garbage collected
//
// This calls runtime.SetFinalizer on x so it shouldn't have a
// finalizer already.
func PinUntilFinalized(f fs.Fs, x any) {
	Pin(f)
	runtime.SetFinalizer(x, func(_ any) {
		Unpin(f)
	})
}

// Unpin f from the cache
func Unpin(f fs.Fs) {
	createOnFirstUse()
	c.Unpin(fs.ConfigString(f))
}

// To avoid circular dependencies these are filled in by fs/rc/jobs/job.go
var (
	// JobGetJobID for internal use only
	JobGetJobID func(context.Context) (int64, bool)
	// JobOnFinish for internal use only
	JobOnFinish func(int64, func()) (func(), error)
)

// Get gets an fs.Fs named fsString either from the cache or creates it afresh
func Get(ctx context.Context, fsString string) (f fs.Fs, err error) {
	// If we are making a long lived backend which lives longer
	// than this request, we want to disconnect it from the
	// current context and in particular any WithCancel contexts,
	// but we want to preserve the config embedded in the context.
	newCtx := context.Background()
	newCtx = fs.CopyConfig(newCtx, ctx)
	newCtx = filter.CopyConfig(newCtx, ctx)
	f, err = GetFn(newCtx, fsString, fs.NewFs)
	if f == nil || (err != nil && err != fs.ErrorIsFile) {
		return f, err
	}
	// If this is part of an rc job then pin the backend until it finishes
	if JobOnFinish != nil && JobGetJobID != nil {
		if jobID, ok := JobGetJobID(ctx); ok {
			// fs.Debugf(f, "Pin for job %d", jobID)
			Pin(f)
			_, _ = JobOnFinish(jobID, func() {
				// fs.Debugf(f, "Unpin for job %d", jobID)
				Unpin(f)
			})
		}
	}
	return f, err
}

// GetArr gets []fs.Fs from []fsStrings either from the cache or creates it afresh
func GetArr(ctx context.Context, fsStrings []string) (f []fs.Fs, err error) {
	var fArr []fs.Fs
	for _, fsString := range fsStrings {
		f1, err1 := GetFn(ctx, fsString, fs.NewFs)
		if err1 != nil {
			return fArr, err1
		}
		fArr = append(fArr, f1)
	}
	return fArr, nil
}

// PutErr puts an fs.Fs named fsString into the cache with err
func PutErr(fsString string, f fs.Fs, err error) {
	createOnFirstUse()
	canonicalName := fs.ConfigString(f)
	c.PutErr(canonicalName, f, err)
	addMapping(fsString, canonicalName)
	if err == fs.ErrorIsFile {
		addChild(fsString, canonicalName)
	}
}

// Put puts an fs.Fs named fsString into the cache
func Put(fsString string, f fs.Fs) {
	PutErr(fsString, f, nil)
}

// ClearConfig deletes all entries which were based on the config name passed in
//
// Returns number of entries deleted
func ClearConfig(name string) (deleted int) {
	createOnFirstUse()
	ClearMappingsPrefix(name)
	return c.DeletePrefix(name + ":")
}

// Clear removes everything from the cache
func Clear() {
	createOnFirstUse()
	c.Clear()
	ClearMappings()
}

// Entries returns the number of entries in the cache
func Entries() int {
	createOnFirstUse()
	return c.Entries()
}

// ClearMappings removes everything from remap and childParentMap
func ClearMappings() {
	mu.Lock()
	defer mu.Unlock()
	remap = map[string]string{}
	childParentMap = map[string]string{}
}

// ClearMappingsPrefix deletes all mappings to parents with given prefix
//
// Returns number of entries deleted
func ClearMappingsPrefix(prefix string) (deleted int) {
	mu.Lock()
	do := func(mapping map[string]string) {
		for key, val := range mapping {
			if !strings.HasPrefix(val, prefix) {
				continue
			}
			delete(mapping, key)
			deleted++
		}
	}
	do(remap)
	do(childParentMap)
	mu.Unlock()
	return deleted
}

// EntriesWithPinCount returns the number of pinned and unpinned entries in the cache
//
// Each entry is counted only once, regardless of entry.pinCount
func EntriesWithPinCount() (pinned, unpinned int) {
	createOnFirstUse()
	return c.EntriesWithPinCount()
}
