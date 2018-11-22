// This implements the Fs cache

package rc

import (
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
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
	f        fs.Fs
	fsString string
	lastUsed time.Time
}

// GetCachedFs gets a fs.Fs named fsString either from the cache or creates it afresh
func GetCachedFs(fsString string) (f fs.Fs, err error) {
	fsCacheMu.Lock()
	defer fsCacheMu.Unlock()
	entry, ok := fsCache[fsString]
	if !ok {
		f, err = fsNewFs(fsString)
		if err != nil {
			return nil, err
		}
		entry = &cacheEntry{
			f:        f,
			fsString: fsString,
		}
		fsCache[fsString] = entry
	}
	entry.lastUsed = time.Now()
	if !expireRunning {
		time.AfterFunc(cacheExpireInterval, cacheExpire)
		expireRunning = true
	}
	return entry.f, err
}

// PutCachedFs puts an fs.Fs named fsString into the cache
func PutCachedFs(fsString string, f fs.Fs) {
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

// GetFsNamed gets a fs.Fs named fsName either from the cache or creates it afresh
func GetFsNamed(in Params, fsName string) (f fs.Fs, err error) {
	fsString, err := in.GetString(fsName)
	if err != nil {
		return nil, err
	}

	return GetCachedFs(fsString)
}

// GetFs gets a fs.Fs named "fs" either from the cache or creates it afresh
func GetFs(in Params) (f fs.Fs, err error) {
	return GetFsNamed(in, "fs")
}

// GetFsAndRemoteNamed gets the fsName parameter from in, makes a
// remote or fetches it from the cache then gets the remoteName
// parameter from in too.
func GetFsAndRemoteNamed(in Params, fsName, remoteName string) (f fs.Fs, remote string, err error) {
	remote, err = in.GetString(remoteName)
	if err != nil {
		return
	}
	f, err = GetFsNamed(in, fsName)
	return

}

// GetFsAndRemote gets the `fs` parameter from in, makes a remote or
// fetches it from the cache then gets the `remote` parameter from in
// too.
func GetFsAndRemote(in Params) (f fs.Fs, remote string, err error) {
	return GetFsAndRemoteNamed(in, "fs", "remote")
}
