// Package cache implements the Fs cache
package cache

import (
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

var (
	fsCacheMu           sync.Mutex
	fsCache             = map[string]*cacheEntry{}
	fsNewFs             = fs.NewFs // for tests
	expireRunning       = false
	cacheExpireDuration = 300 * time.Second // expire the cache entry when it is older than this
	cacheExpireInterval = 60 * time.Second  // interval to run the cache expire
)

type cacheEntry struct {
	f        fs.Fs     // cached f
	err      error     // nil or fs.ErrorIsFile
	fsString string    // remote string
	lastUsed time.Time // time used for expiry
}

// Get gets a fs.Fs named fsString either from the cache or creates it afresh
func Get(fsString string) (f fs.Fs, err error) {
	fsCacheMu.Lock()
	entry, ok := fsCache[fsString]
	if !ok {
		fsCacheMu.Unlock() // Unlock in case Get is called recursively
		f, err = fsNewFs(fsString)
		if err != nil && err != fs.ErrorIsFile {
			return f, err
		}
		entry = &cacheEntry{
			f:        f,
			fsString: fsString,
			err:      err,
		}
		fsCacheMu.Lock()
		fsCache[fsString] = entry
	}
	defer fsCacheMu.Unlock()
	entry.lastUsed = time.Now()
	if !expireRunning {
		time.AfterFunc(cacheExpireInterval, cacheExpire)
		expireRunning = true
	}
	return entry.f, entry.err
}

// Put puts an fs.Fs named fsString into the cache
func Put(fsString string, f fs.Fs) {
	fsCacheMu.Lock()
	defer fsCacheMu.Unlock()
	fsCache[fsString] = &cacheEntry{
		f:        f,
		fsString: fsString,
		lastUsed: time.Now(),
	}
	if !expireRunning {
		time.AfterFunc(cacheExpireInterval, cacheExpire)
		expireRunning = true
	}
}

// cacheExpire expires any entries that haven't been used recently
func cacheExpire() {
	fsCacheMu.Lock()
	defer fsCacheMu.Unlock()
	now := time.Now()
	for fsString, entry := range fsCache {
		if now.Sub(entry.lastUsed) > cacheExpireDuration {
			delete(fsCache, fsString)
		}
	}
	if len(fsCache) != 0 {
		time.AfterFunc(cacheExpireInterval, cacheExpire)
		expireRunning = true
	} else {
		expireRunning = false
	}
}

// Clear removes everything from the cahce
func Clear() {
	fsCacheMu.Lock()
	for k := range fsCache {
		delete(fsCache, k)
	}
	fsCacheMu.Unlock()
}
