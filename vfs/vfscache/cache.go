// Package vfscache deals with caching of files locally for the VFS layer
package vfscache

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

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	fscache "github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// NB as Cache and Item are tightly linked it is necessary to have a
// total lock ordering between them. So Cache.mu must always be
// taken before Item.mu to avoid deadlocks.
//
// Cache may call into Item but care is needed if Item calls Cache

// FIXME size in cache needs to be size on disk if we have sparse files...

// Cache opened files
type Cache struct {
	// read only - no locking needed to read these
	fremote    fs.Fs              // fs for the remote we are caching
	fcache     fs.Fs              // fs for the cache directory
	fcacheMeta fs.Fs              // fs for the cache metadata directory
	opt        *vfscommon.Options // vfs Options
	root       string             // root of the cache directory
	metaRoot   string             // root of the cache metadata directory
	hashType   hash.Type          // hash to use locally and remotely
	hashOption *fs.HashesOption   // corresponding OpenOption

	mu   sync.Mutex       // protects the following variables
	item map[string]*Item // files/directories in the cache
	used int64            // total size of files in the cache
}

// New creates a new cache heirachy for fremote
//
// This starts background goroutines which can be cancelled with the
// context passed in.
func New(ctx context.Context, fremote fs.Fs, opt *vfscommon.Options) (*Cache, error) {
	fRoot := filepath.FromSlash(fremote.Root())
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(fRoot, `\\?`) {
			fRoot = fRoot[3:]
		}
		fRoot = strings.Replace(fRoot, ":", "", -1)
	}
	root := filepath.Join(config.CacheDir, "vfs", fremote.Name(), fRoot)
	fs.Debugf(nil, "vfs cache root is %q", root)
	metaRoot := filepath.Join(config.CacheDir, "vfsMeta", fremote.Name(), fRoot)
	fs.Debugf(nil, "vfs metadata cache root is %q", root)

	fcache, err := fscache.Get(root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cache remote")
	}
	fcacheMeta, err := fscache.Get(root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cache meta remote")
	}

	hashType, hashOption := operations.CommonHash(fcache, fremote)

	c := &Cache{
		fremote:    fremote,
		fcache:     fcache,
		fcacheMeta: fcacheMeta,
		opt:        opt,
		root:       root,
		metaRoot:   metaRoot,
		item:       make(map[string]*Item),
		hashType:   hashType,
		hashOption: hashOption,
	}

	// Make sure cache directories exist
	_, err = c.mkdir("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to make cache directory")
	}

	// load in the cache and metadata off disk
	err = c.reload()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load cache")
	}

	// Remove any empty directories
	c.purgeEmptyDirs()

	go c.cleaner(ctx)

	return c, nil
}

// clean returns the cleaned version of name for use in the index map
//
// name should be a remote path not an osPath
func clean(name string) string {
	name = strings.Trim(name, "/")
	name = path.Clean(name)
	if name == "." || name == "/" {
		name = ""
	}
	return name
}

// toOSPath turns a remote relative name into an OS path in the cache
func (c *Cache) toOSPath(name string) string {
	return filepath.Join(c.root, filepath.FromSlash(name))
}

// toOSPathMeta turns a remote relative name into an OS path in the
// cache for the metadata
func (c *Cache) toOSPathMeta(name string) string {
	return filepath.Join(c.metaRoot, filepath.FromSlash(name))
}

// mkdir makes the directory for name in the cache and returns an os
// path for the file
func (c *Cache) mkdir(name string) (string, error) {
	parent := vfscommon.FindParent(name)
	leaf := filepath.Base(name)
	parentPath := c.toOSPath(parent)
	err := os.MkdirAll(parentPath, 0700)
	if err != nil {
		return "", errors.Wrap(err, "make cache directory failed")
	}
	parentPathMeta := c.toOSPathMeta(parent)
	err = os.MkdirAll(parentPathMeta, 0700)
	if err != nil {
		return "", errors.Wrap(err, "make cache meta directory failed")
	}
	return filepath.Join(parentPath, leaf), nil
}

// _get gets name from the cache or creates a new one
//
// It returns the item and found as to whether this item was found in
// the cache (or just created).
//
// name should be a remote path not an osPath
//
// must be called with mu held
func (c *Cache) _get(name string) (item *Item, found bool) {
	item = c.item[name]
	found = item != nil
	if !found {
		item = newItem(c, name)
		c.item[name] = item
	}
	return item, found
}

// put puts item under name in the cache
//
// It returns an old item if there was one or nil if not.
//
// name should be a remote path not an osPath
func (c *Cache) put(name string, item *Item) (oldItem *Item) {
	name = clean(name)
	c.mu.Lock()
	oldItem = c.item[name]
	if oldItem != item {
		c.item[name] = item
	} else {
		oldItem = nil
	}
	c.mu.Unlock()
	return oldItem
}

// InUse returns whether the name is in use in the cache
//
// name should be a remote path not an osPath
func (c *Cache) InUse(name string) bool {
	name = clean(name)
	c.mu.Lock()
	item := c.item[name]
	c.mu.Unlock()
	if item == nil {
		return false
	}
	return item.inUse()
}

// get gets a file name from the cache or creates a new one
//
// It returns the item and found as to whether this item was found in
// the cache (or just created).
//
// name should be a remote path not an osPath
func (c *Cache) get(name string) (item *Item, found bool) {
	name = clean(name)
	c.mu.Lock()
	item, found = c._get(name)
	c.mu.Unlock()
	return item, found
}

// Item gets a cache item for name
//
// To use it item.Open will need to be called
//
// name should be a remote path not an osPath
func (c *Cache) Item(name string) (item *Item) {
	item, _ = c.get(name)
	return item
}

// Exists checks to see if the file exists in the cache or not
//
// FIXME check the metadata exists here too?
func (c *Cache) Exists(name string) bool {
	osPath := c.toOSPath(name)
	fi, err := os.Stat(osPath)
	if err != nil {
		return false
	}
	// checks for non-regular files (e.g. directories, symlinks, devices, etc.)
	if !fi.Mode().IsRegular() {
		return false
	}
	return true
}

// rename with os.Rename and more checking
func rename(osOldPath, osNewPath string) error {
	sfi, err := os.Stat(osOldPath)
	if err != nil {
		// Just do nothing if the source does not exist
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "Failed to stat source: %s", osOldPath)
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories, symlinks, devices, etc.)
		return errors.Errorf("Non-regular source file: %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(osNewPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "Failed to stat destination: %s", osNewPath)
		}
		parent := vfscommon.OsFindParent(osNewPath)
		err = os.MkdirAll(parent, 0700)
		if err != nil {
			return errors.Wrapf(err, "Failed to create parent dir: %s", parent)
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return errors.Errorf("Non-regular destination file: %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return nil
		}
	}
	if err = os.Rename(osOldPath, osNewPath); err != nil {
		return errors.Wrapf(err, "Failed to rename in cache: %s to %s", osOldPath, osNewPath)
	}
	return nil
}

// Rename the item in cache
func (c *Cache) Rename(name string, newName string, newObj fs.Object) (err error) {
	item, _ := c.get(name)
	err = item.rename(name, newName, newObj)
	if err != nil {
		return err
	}

	// Move the item in the cache
	c.mu.Lock()
	if item, ok := c.item[name]; ok {
		c.item[newName] = item
		delete(c.item, name)
	}
	c.mu.Unlock()

	fs.Infof(name, "Renamed in cache to %q", newName)
	return nil
}

// Remove should be called if name is deleted
func (c *Cache) Remove(name string) {
	item, _ := c.get(name)
	item.remove("file deleted")
}

// SetModTime should be called to set the modification time of the cache file
func (c *Cache) SetModTime(name string, modTime time.Time) {
	item, _ := c.get(name)
	item.setModTime(modTime)
}

// CleanUp empties the cache of everything
func (c *Cache) CleanUp() error {
	err1 := os.RemoveAll(c.root)
	err2 := os.RemoveAll(c.metaRoot)
	if err1 != nil {
		return err1
	}
	return err2
}

// walk walks the cache calling the function
func (c *Cache) walk(dir string, fn func(osPath string, fi os.FileInfo, name string) error) error {
	return filepath.Walk(dir, func(osPath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Find path relative to the cache root
		name, err := filepath.Rel(dir, osPath)
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

// reload walks the cache loading metadata files
func (c *Cache) reload() error {
	err := c.walk(c.root, func(osPath string, fi os.FileInfo, name string) error {
		if !fi.IsDir() {
			_, _ = c.get(name)
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk cache")
	}
	err = c.walk(c.root, func(osPathMeta string, fi os.FileInfo, name string) error {
		if !fi.IsDir() {
			_, _ = c.get(name)
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to walk meta cache")
	}
	return err
}

// purgeOld gets rid of any files that are over age
func (c *Cache) purgeOld(maxAge time.Duration) {
	c._purgeOld(maxAge, func(item *Item) {
		item.remove("too old")
	})
}

func (c *Cache) _purgeOld(maxAge time.Duration, remove func(item *Item)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for name, item := range c.item {
		if !item.inUse() {
			// If not locked and access time too long ago - delete the file
			dt := item.getATime().Sub(cutoff)
			// fs.Debugf(name, "atime=%v cutoff=%v, dt=%v", item.info.ATime, cutoff, dt)
			if dt < 0 {
				remove(item)
				// Remove the entry
				delete(c.item, name)
			}
		}
	}
}

// Purge any empty directories
func (c *Cache) purgeEmptyDirs() {
	ctx := context.Background()
	err := operations.Rmdirs(ctx, c.fcache, "", true)
	if err != nil {
		fs.Errorf(c.fcache, "Failed to remove empty directories from cache: %v", err)
	}
	err = operations.Rmdirs(ctx, c.fcacheMeta, "", true)
	if err != nil {
		fs.Errorf(c.fcache, "Failed to remove empty directories from metadata cache: %v", err)
	}
}

// Remove any files that are over quota starting from the
// oldest first
func (c *Cache) purgeOverQuota(quota int64) {
	c._purgeOverQuota(quota, func(item *Item) {
		item.remove("over quota")
	})
}

// updateUsed updates c.used so it is accurate
func (c *Cache) updateUsed() {
	c.mu.Lock()
	defer c.mu.Unlock()

	newUsed := int64(0)
	for _, item := range c.item {
		newUsed += item.getDiskSize()
	}
	c.used = newUsed
}

func (c *Cache) _purgeOverQuota(quota int64, remove func(item *Item)) {
	c.updateUsed()

	c.mu.Lock()
	defer c.mu.Unlock()

	if quota <= 0 || c.used < quota {
		return
	}

	var items Items

	// Make a slice of unused files
	for _, item := range c.item {
		if !item.inUse() {
			items = append(items, item)
		}
	}

	sort.Sort(items)

	// Remove items until the quota is OK
	for _, item := range items {
		if c.used < quota {
			break
		}
		c.used -= item.getDiskSize()
		remove(item)
		// Remove the entry
		delete(c.item, item.name)
	}
}

// clean empties the cache of stuff if it can
func (c *Cache) clean() {
	// Cache may be empty so end
	_, err := os.Stat(c.root)
	if os.IsNotExist(err) {
		return
	}

	c.mu.Lock()
	oldItems, oldUsed := len(c.item), fs.SizeSuffix(c.used)
	c.mu.Unlock()

	// Remove any files that are over age
	c.purgeOld(c.opt.CacheMaxAge)

	// Now remove any files that are over quota starting from the
	// oldest first
	c.purgeOverQuota(int64(c.opt.CacheMaxSize))

	// Stats
	c.mu.Lock()
	newItems, newUsed := len(c.item), fs.SizeSuffix(c.used)
	c.mu.Unlock()

	fs.Infof(nil, "Cleaned the cache: objects %d (was %d), total size %v (was %v)", newItems, oldItems, newUsed, oldUsed)
}

// cleaner calls clean at regular intervals
//
// doesn't return until context is cancelled
func (c *Cache) cleaner(ctx context.Context) {
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

// Check the local file is up to date in the cache
func (c *Cache) Check(ctx context.Context, o fs.Object, remote string) (err error) {
	defer log.Trace(o, "remote=%q", remote)("err=%v", &err)
	item, _ := c.get(remote)
	return item.checkObject(o)
}
