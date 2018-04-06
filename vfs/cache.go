// This deals with caching of files locally

package vfs

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/djherbis/times"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/pkg/errors"
)

// CacheMode controls the functionality of the cache
type CacheMode byte

// CacheMode options
const (
	CacheModeOff     CacheMode = iota // cache nothing - return errors for writes which can't be satisfied
	CacheModeMinimal                  // cache only the minimum, eg read/write opens
	CacheModeWrites                   // cache all files opened with write intent
	CacheModeFull                     // cache all files opened in any mode
)

var cacheModeToString = []string{
	CacheModeOff:     "off",
	CacheModeMinimal: "minimal",
	CacheModeWrites:  "writes",
	CacheModeFull:    "full",
}

// String turns a CacheMode into a string
func (l CacheMode) String() string {
	if l >= CacheMode(len(cacheModeToString)) {
		return fmt.Sprintf("CacheMode(%d)", l)
	}
	return cacheModeToString[l]
}

// Set a CacheMode
func (l *CacheMode) Set(s string) error {
	for n, name := range cacheModeToString {
		if s != "" && name == s {
			*l = CacheMode(n)
			return nil
		}
	}
	return errors.Errorf("Unknown cache mode level %q", s)
}

// Type of the value
func (l *CacheMode) Type() string {
	return "string"
}

// cache opened files
type cache struct {
	f      fs.Fs                 // fs for the cache directory
	opt    *Options              // vfs Options
	root   string                // root of the cache directory
	itemMu sync.Mutex            // protects the next two maps
	item   map[string]*cacheItem // files/directories in the cache
}

// cacheItem is stored in the item map
type cacheItem struct {
	opens  int       // number of times file is open
	atime  time.Time // last time file was accessed
	isFile bool      // if this is a file or a directory
}

// newCacheItem returns an item for the cache
func newCacheItem(isFile bool) *cacheItem {
	return &cacheItem{atime: time.Now(), isFile: isFile}
}

// newCache creates a new cache heirachy for f
//
// This starts background goroutines which can be cancelled with the
// context passed in.
func newCache(ctx context.Context, f fs.Fs, opt *Options) (*cache, error) {
	fRoot := filepath.FromSlash(f.Root())
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(fRoot, `\\?`) {
			fRoot = fRoot[3:]
		}
		fRoot = strings.Replace(fRoot, ":", "", -1)
	}
	root := filepath.Join(config.CacheDir, "vfs", f.Name(), fRoot)
	fs.Debugf(nil, "vfs cache root is %q", root)

	f, err := fs.NewFs(root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cache remote")
	}

	c := &cache{
		f:    f,
		opt:  opt,
		root: root,
		item: make(map[string]*cacheItem),
	}

	go c.cleaner(ctx)

	return c, nil
}

// findParent returns the parent directory of name, or "" for the root
func findParent(name string) string {
	parent := path.Dir(name)
	if parent == "." || parent == "/" {
		parent = ""
	}
	return parent
}

// clean returns the cleaned version of name for use in the index map
func clean(name string) string {
	name = strings.Trim(name, "/")
	name = path.Clean(name)
	if name == "." || name == "/" {
		name = ""
	}
	return name
}

// toOSPath turns a remote relative name into an OS path in the cache
func (c *cache) toOSPath(name string) string {
	return filepath.Join(c.root, filepath.FromSlash(name))
}

// mkdir makes the directory for name in the cache and returns an os
// path for the file
func (c *cache) mkdir(name string) (string, error) {
	parent := findParent(name)
	leaf := path.Base(name)
	parentPath := c.toOSPath(parent)
	err := os.MkdirAll(parentPath, 0700)
	if err != nil {
		return "", errors.Wrap(err, "make cache directory failed")
	}
	c.cacheDir(parent)
	return filepath.Join(parentPath, leaf), nil
}

// _get gets name from the cache or creates a new one
//
// It returns the item and found as to whether this item was found in
// the cache (or just created).
//
// name should be a remote path not an osPath
//
// must be called with itemMu held
func (c *cache) _get(isFile bool, name string) (item *cacheItem, found bool) {
	item = c.item[name]
	found = item != nil
	if !found {
		item = newCacheItem(isFile)
		c.item[name] = item
	}
	return item, found
}

// opens returns the number of opens that are on the file
//
// name should be a remote path not an osPath
func (c *cache) opens(name string) int {
	name = clean(name)
	c.itemMu.Lock()
	defer c.itemMu.Unlock()
	item := c.item[name]
	if item == nil {
		return 0
	}
	return item.opens
}

// get gets name from the cache or creates a new one
//
// name should be a remote path not an osPath
func (c *cache) get(name string) *cacheItem {
	name = clean(name)
	c.itemMu.Lock()
	item, _ := c._get(true, name)
	c.itemMu.Unlock()
	return item
}

// updateTime sets the atime of the name to that passed in if it is
// newer than the existing or there isn't an existing time.
//
// name should be a remote path not an osPath
func (c *cache) updateTime(name string, when time.Time) {
	name = clean(name)
	c.itemMu.Lock()
	item, found := c._get(true, name)
	if !found || when.Sub(item.atime) > 0 {
		fs.Debugf(name, "updateTime: setting atime to %v", when)
		item.atime = when
	}
	c.itemMu.Unlock()
}

// _open marks name as open, must be called with the lock held
//
// name should be a remote path not an osPath
func (c *cache) _open(isFile bool, name string) {
	for {
		item, _ := c._get(isFile, name)
		item.opens++
		item.atime = time.Now()
		if name == "" {
			break
		}
		isFile = false
		name = findParent(name)
	}
}

// open marks name as open
//
// name should be a remote path not an osPath
func (c *cache) open(name string) {
	name = clean(name)
	c.itemMu.Lock()
	c._open(true, name)
	c.itemMu.Unlock()
}

// cacheDir marks a directory and its parents as being in the cache
//
// name should be a remote path not an osPath
func (c *cache) cacheDir(name string) {
	name = clean(name)
	c.itemMu.Lock()
	defer c.itemMu.Unlock()
	for {
		item := c.item[name]
		if item != nil {
			break
		}
		c.item[name] = newCacheItem(false)
		if name == "" {
			break
		}
		name = findParent(name)
	}
}

// _close marks name as closed - must be called with the lock held
func (c *cache) _close(isFile bool, name string) {
	for {
		item, _ := c._get(isFile, name)
		item.opens--
		item.atime = time.Now()
		if item.opens < 0 {
			fs.Errorf(name, "cache: double close")
		}
		if name == "" {
			break
		}
		isFile = false
		name = findParent(name)
	}
}

// close marks name as closed
//
// name should be a remote path not an osPath
func (c *cache) close(name string) {
	name = clean(name)
	c.itemMu.Lock()
	c._close(true, name)
	c.itemMu.Unlock()
}

// remove should be called if name is deleted
func (c *cache) remove(name string) {
	osPath := c.toOSPath(name)
	err := os.Remove(osPath)
	if err != nil && !os.IsNotExist(err) {
		fs.Errorf(name, "Failed to remove from cache: %v", err)
	} else {
		fs.Debugf(name, "Removed from cache")
	}
}

// removeDir should be called if dir is deleted and returns true if
// the directory is gone.
func (c *cache) removeDir(dir string) bool {
	osPath := c.toOSPath(dir)
	err := os.Remove(osPath)
	if err == nil || os.IsNotExist(err) {
		if err == nil {
			fs.Debugf(dir, "Removed empty directory")
		}
		return true
	}
	if !os.IsExist(err) {
		fs.Errorf(dir, "Failed to remove cached dir: %v", err)
	}
	return false
}

// cleanUp empties the cache of everything
func (c *cache) cleanUp() error {
	return os.RemoveAll(c.root)
}

// walk walks the cache calling the function
func (c *cache) walk(fn func(osPath string, fi os.FileInfo, name string) error) error {
	return filepath.Walk(c.root, func(osPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Find path relative to the cache root
		name, err := filepath.Rel(c.root, osPath)
		if err != nil {
			return errors.Wrap(err, "filepath.Rel failed in walk")
		}
		if name == "." {
			name = ""
		}
		// And convert into slashes
		name = filepath.ToSlash(name)

		return fn(osPath, fi, name)
	})
}

// updateAtimes walks the cache updating any atimes it finds
func (c *cache) updateAtimes() error {
	return c.walk(func(osPath string, fi os.FileInfo, name string) error {
		if !fi.IsDir() {
			// Update the atime with that of the file
			atime := times.Get(fi).AccessTime()
			c.updateTime(name, atime)
		} else {
			c.cacheDir(name)
		}
		return nil
	})
}

// purgeOld gets rid of any files that are over age
func (c *cache) purgeOld(maxAge time.Duration) {
	c._purgeOld(maxAge, c.remove, c.removeDir)
}

func (c *cache) _purgeOld(maxAge time.Duration, remove func(name string), removeDir func(name string) bool) {
	c.itemMu.Lock()
	defer c.itemMu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for name, item := range c.item {
		if item.isFile && item.opens == 0 {
			// If not locked and access time too long ago - delete the file
			dt := item.atime.Sub(cutoff)
			// fs.Debugf(name, "atime=%v cutoff=%v, dt=%v", item.atime, cutoff, dt)
			if dt < 0 {
				remove(name)
				// Remove the entry
				delete(c.item, name)
			}
		}
	}
	// now find any empty directories
	var dirs []string
	for name, item := range c.item {
		if !item.isFile && item.opens == 0 {
			dirs = append(dirs, name)
		}
	}
	// remove empty directories in reverse alphabetical order
	sort.Strings(dirs)
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		// Remove the entry
		if removeDir(dir) {
			delete(c.item, dir)
		}
	}
}

// clean empties the cache of stuff if it can
func (c *cache) clean() {
	// Cache may be empty so end
	_, err := os.Stat(c.root)
	if os.IsNotExist(err) {
		return
	}

	fs.Debugf(nil, "Cleaning the cache")

	// first walk the FS to update the atimes
	err = c.updateAtimes()
	if err != nil {
		fs.Errorf(nil, "Error traversing cache %q: %v", c.root, err)
	}

	// Now remove any files that are over age and any empty
	// directories
	c.purgeOld(c.opt.CacheMaxAge)
}

// cleaner calls clean at regular intervals
//
// doesn't return until context is cancelled
func (c *cache) cleaner(ctx context.Context) {
	if c.opt.CachePollInterval <= 0 {
		fs.Debugf(nil, "Cache cleaning thread disabled because poll interval <= 0")
		return
	}
	// Start cleaning the cache immediately
	c.clean()
	// Then every interval specified
	timer := time.NewTicker(c.opt.CachePollInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			c.clean()
		case <-ctx.Done():
			fs.Debugf(nil, "cache cleaner exiting")
			return
		}
	}
}
