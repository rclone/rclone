// Package cache implements the Fs cache
package cache

import (
	"runtime"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/cache"
)

var (
	c     = cache.New()
	mu    sync.Mutex            // mutex to protect remap
	remap = map[string]string{} // map user supplied names to canonical names
)

// Canonicalize looks up fsString in the mapping from user supplied
// names to canonical names and return the canonical form
func Canonicalize(fsString string) string {
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

// GetFn gets an fs.Fs named fsString either from the cache or creates
// it afresh with the create function
func GetFn(fsString string, create func(fsString string) (fs.Fs, error)) (f fs.Fs, err error) {
	fsString = Canonicalize(fsString)
	created := false
	value, err := c.Get(fsString, func(fsString string) (f interface{}, ok bool, err error) {
		f, err = create(fsString)
		ok = err == nil || err == fs.ErrorIsFile
		created = ok
		return f, ok, err
	})
	if err != nil && err != fs.ErrorIsFile {
		return nil, err
	}
	f = value.(fs.Fs)
	// Check we stored the Fs at the canonical name
	if created {
		canonicalName := fs.ConfigString(f)
		if canonicalName != fsString {
			// Note that if err == fs.ErrorIsFile at this moment
			// then we can't rename the remote as it will have the
			// wrong error status, we need to add a new one.
			if err == nil {
				fs.Debugf(nil, "fs cache: renaming cache item %q to be canonical %q", fsString, canonicalName)
				value, found := c.Rename(fsString, canonicalName)
				if found {
					f = value.(fs.Fs)
				}
				addMapping(fsString, canonicalName)
			} else {
				fs.Debugf(nil, "fs cache: adding new entry for parent of %q, %q", fsString, canonicalName)
				Put(canonicalName, f)
			}
		}
	}
	return f, err
}

// Pin f into the cache until Unpin is called
func Pin(f fs.Fs) {
	c.Pin(fs.ConfigString(f))
}

// PinUntilFinalized pins f into the cache until x is garbage collected
//
// This calls runtime.SetFinalizer on x so it shouldn't have a
// finalizer already.
func PinUntilFinalized(f fs.Fs, x interface{}) {
	Pin(f)
	runtime.SetFinalizer(x, func(_ interface{}) {
		Unpin(f)
	})

}

// Unpin f from the cache
func Unpin(f fs.Fs) {
	c.Pin(fs.ConfigString(f))
}

// Get gets an fs.Fs named fsString either from the cache or creates it afresh
func Get(fsString string) (f fs.Fs, err error) {
	return GetFn(fsString, fs.NewFs)
}

// Put puts an fs.Fs named fsString into the cache
func Put(fsString string, f fs.Fs) {
	canonicalName := fs.ConfigString(f)
	c.Put(canonicalName, f)
	addMapping(fsString, canonicalName)
}

// Clear removes everything from the cache
func Clear() {
	c.Clear()
}
