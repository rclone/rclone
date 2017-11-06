// This deals with caching of files locally

package vfs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/djherbis/times"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
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
	item   map[string]*cacheItem // files in the cache
}

// cacheItem is stored in the item map
type cacheItem struct {
	opens int       // number of times file is open
	atime time.Time // last time file was accessed
}

// newCacheItem returns an item for the cache
func newCacheItem() *cacheItem {
	return &cacheItem{atime: time.Now()}
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
	root := filepath.Join(fs.CacheDir, "vfs", f.Name(), fRoot)
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

// mkdir makes the directory for name in the cache and returns an os
// path for the file
func (c *cache) mkdir(name string) (string, error) {
	parent := path.Dir(name)
	if parent == "." {
		parent = ""
	}
	leaf := path.Base(name)
	parentPath := filepath.Join(c.root, filepath.FromSlash(parent))
	err := os.MkdirAll(parentPath, 0700)
	if err != nil {
		return "", errors.Wrap(err, "make cache directory failed")
	}
	return filepath.Join(parentPath, leaf), nil
}

// _get gets name from the cache or creates a new one
//
// must be called with itemMu held
func (c *cache) _get(name string) *cacheItem {
	item := c.item[name]
	if item == nil {
		item = newCacheItem()
		c.item[name] = item
	}
	return item
}

// get gets name from the cache or creates a new one
func (c *cache) get(name string) *cacheItem {
	c.itemMu.Lock()
	item := c._get(name)
	c.itemMu.Unlock()
	return item
}

// updateTime sets the atime of the name to that passed in if it is
// newer than the existing or there isn't an existing time.
func (c *cache) updateTime(name string, when time.Time) {
	c.itemMu.Lock()
	item := c._get(name)
	if when.Sub(item.atime) > 0 {
		fs.Debugf(name, "updateTime: setting atime to %v", when)
		item.atime = when
	}
	c.itemMu.Unlock()
}

// open marks name as open
func (c *cache) open(name string) {
	c.itemMu.Lock()
	item := c._get(name)
	item.opens++
	item.atime = time.Now()
	c.itemMu.Unlock()
}

// close marks name as closed
func (c *cache) close(name string) {
	c.itemMu.Lock()
	item := c._get(name)
	item.opens--
	item.atime = time.Now()
	if item.opens < 0 {
		fs.Errorf(name, "cache: double close")
	}
	c.itemMu.Unlock()
}

// cleanUp empties the cache of everything
func (c *cache) cleanUp() error {
	return os.RemoveAll(c.root)
}

// updateAtimes walks the cache updating any atimes it finds
func (c *cache) updateAtimes() error {
	return filepath.Walk(c.root, func(osPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			// Find path relative to the cache root
			name, err := filepath.Rel(c.root, osPath)
			if err != nil {
				return errors.Wrap(err, "filepath.Rel failed in updatAtimes")
			}
			// And convert into slashes
			name = filepath.ToSlash(name)

			// Update the atime with that of the file
			atime := times.Get(fi).AccessTime()
			c.updateTime(name, atime)
		}
		return nil
	})
}

// purgeOld gets rid of any files that are over age
func (c *cache) purgeOld(maxAge time.Duration) {
	c.itemMu.Lock()
	defer c.itemMu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for name, item := range c.item {
		// If not locked and access time too long ago - delete the file
		dt := item.atime.Sub(cutoff)
		// fs.Debugf(name, "atime=%v cutoff=%v, dt=%v", item.atime, cutoff, dt)
		if item.opens == 0 && dt < 0 {
			osPath := filepath.Join(c.root, filepath.FromSlash(name))
			err := os.Remove(osPath)
			if err != nil {
				fs.Errorf(name, "Failed to remove from cache: %v", err)
			} else {
				fs.Debugf(name, "Removed from cache")
			}
			// Remove the entry
			delete(c.item, name)
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

	// Now remove any files that are over age
	c.purgeOld(c.opt.CacheMaxAge)

	// Now tidy up any empty directories
	err = fs.Rmdirs(c.f, "")
	if err != nil {
		fs.Errorf(c.f, "Failed to remove empty directories from cache: %v", err)
	}
}

// cleaner calls clean at regular intervals
//
// doesn't return until context is cancelled
func (c *cache) cleaner(ctx context.Context) {
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
