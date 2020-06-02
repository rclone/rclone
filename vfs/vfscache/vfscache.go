// Package vfscache deals with caching of files locally for the VFS layer
package vfscache

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/djherbis/times"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	fscache "github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Cache opened files
type Cache struct {
	fremote fs.Fs                 // fs for the remote we are caching
	fcache  fs.Fs                 // fs for the cache directory
	opt     *vfscommon.Options    // vfs Options
	root    string                // root of the cache directory
	itemMu  sync.Mutex            // protects the following variables
	item    map[string]*cacheItem // files/directories in the cache
	used    int64                 // total size of files in the cache
}

// cacheItem is stored in the item map
type cacheItem struct {
	opens  int       // number of times file is open
	atime  time.Time // last time file was accessed
	isFile bool      // if this is a file or a directory
	size   int64     // size of the cached item
}

// newCacheItem returns an item for the cache
func newCacheItem(isFile bool) *cacheItem {
	return &cacheItem{atime: time.Now(), isFile: isFile}
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

	fcache, err := fscache.Get(root)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cache remote")
	}

	c := &Cache{
		fremote: fremote,
		fcache:  fcache,
		opt:     opt,
		root:    root,
		item:    make(map[string]*cacheItem),
	}

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

// ToOSPath turns a remote relative name into an OS path in the cache
func (c *Cache) ToOSPath(name string) string {
	return filepath.Join(c.root, filepath.FromSlash(name))
}

// Mkdir makes the directory for name in the cache and returns an os
// path for the file
//
// name should be a remote path not an osPath
func (c *Cache) Mkdir(name string) (string, error) {
	parent := vfscommon.FindParent(name)
	leaf := path.Base(name)
	parentPath := c.ToOSPath(parent)
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
func (c *Cache) _get(isFile bool, name string) (item *cacheItem, found bool) {
	item = c.item[name]
	found = item != nil
	if !found {
		item = newCacheItem(isFile)
		c.item[name] = item
	}
	return item, found
}

// Opens returns the number of opens that are on the file
//
// name should be a remote path not an osPath
func (c *Cache) Opens(name string) int {
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
func (c *Cache) get(name string) *cacheItem {
	name = clean(name)
	c.itemMu.Lock()
	item, _ := c._get(true, name)
	c.itemMu.Unlock()
	return item
}

// updateStat sets the atime of the name to that passed in if it is
// newer than the existing or there isn't an existing time.
//
// it also sets the size
//
// name should be a remote path not an osPath
func (c *Cache) updateStat(name string, when time.Time, size int64) {
	name = clean(name)
	c.itemMu.Lock()
	item, found := c._get(true, name)
	if !found || when.Sub(item.atime) > 0 {
		fs.Debugf(name, "updateTime: setting atime to %v", when)
		item.atime = when
	}
	item.size = size
	c.itemMu.Unlock()
}

// _open marks name as open, must be called with the lock held
//
// name should be a remote path not an osPath
func (c *Cache) _open(isFile bool, name string) {
	for {
		item, _ := c._get(isFile, name)
		item.opens++
		item.atime = time.Now()
		if name == "" {
			break
		}
		isFile = false
		name = vfscommon.FindParent(name)
	}
}

// Open marks name as open
//
// name should be a remote path not an osPath
func (c *Cache) Open(name string) {
	name = clean(name)
	c.itemMu.Lock()
	c._open(true, name)
	c.itemMu.Unlock()
}

// cacheDir marks a directory and its parents as being in the cache
//
// name should be a remote path not an osPath
func (c *Cache) cacheDir(name string) {
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
		name = vfscommon.FindParent(name)
	}
}

// Exists checks to see if the file exists in the cache or not
func (c *Cache) Exists(name string) bool {
	osPath := c.ToOSPath(name)
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

// Rename the file in cache
func (c *Cache) Rename(name string, newName string) (err error) {
	osOldPath := c.ToOSPath(name)
	osNewPath := c.ToOSPath(newName)
	sfi, err := os.Stat(osOldPath)
	if err != nil {
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
	// Rename the cache item
	c.itemMu.Lock()
	if oldItem, ok := c.item[name]; ok {
		c.item[newName] = oldItem
		delete(c.item, name)
	}
	c.itemMu.Unlock()
	fs.Infof(name, "Renamed in cache")
	return nil
}

// _close marks name as closed - must be called with the lock held
func (c *Cache) _close(isFile bool, name string) {
	for {
		item, _ := c._get(isFile, name)
		item.opens--
		item.atime = time.Now()
		if item.opens < 0 {
			fs.Errorf(name, "cache: double close")
		}
		osPath := c.ToOSPath(name)
		fi, err := os.Stat(osPath)
		// Update the size on close
		if err == nil && !fi.IsDir() {
			item.size = fi.Size()
		}
		if name == "" {
			break
		}
		isFile = false
		name = vfscommon.FindParent(name)
	}
}

// Close marks name as closed
//
// name should be a remote path not an osPath
func (c *Cache) Close(name string) {
	name = clean(name)
	c.itemMu.Lock()
	c._close(true, name)
	c.itemMu.Unlock()
}

// Remove should be called if name is deleted
func (c *Cache) Remove(name string) {
	osPath := c.ToOSPath(name)
	err := os.Remove(osPath)
	if err != nil && !os.IsNotExist(err) {
		fs.Errorf(name, "Failed to remove from cache: %v", err)
	} else {
		fs.Infof(name, "Removed from cache")
	}
}

// removeDir should be called if dir is deleted and returns true if
// the directory is gone.
func (c *Cache) removeDir(dir string) bool {
	osPath := c.ToOSPath(dir)
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

// SetModTime should be called to set the modification time of the cache file
func (c *Cache) SetModTime(name string, modTime time.Time) {
	osPath := c.ToOSPath(name)
	err := os.Chtimes(osPath, modTime, modTime)
	if err != nil {
		fs.Errorf(name, "Failed to set modification time of cached file: %v", err)
	}
}

// CleanUp empties the cache of everything
func (c *Cache) CleanUp() error {
	return os.RemoveAll(c.root)
}

// walk walks the cache calling the function
func (c *Cache) walk(fn func(osPath string, fi os.FileInfo, name string) error) error {
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

// updateStats walks the cache updating any atimes and sizes it finds
//
// it also updates used
func (c *Cache) updateStats() error {
	var newUsed int64
	err := c.walk(func(osPath string, fi os.FileInfo, name string) error {
		if !fi.IsDir() {
			// Update the atime with that of the file
			atime := times.Get(fi).AccessTime()
			c.updateStat(name, atime, fi.Size())
			newUsed += fi.Size()
		} else {
			c.cacheDir(name)
		}
		return nil
	})
	c.itemMu.Lock()
	c.used = newUsed
	c.itemMu.Unlock()
	return err
}

// purgeOld gets rid of any files that are over age
func (c *Cache) purgeOld(maxAge time.Duration) {
	c._purgeOld(maxAge, c.Remove)
}

func (c *Cache) _purgeOld(maxAge time.Duration, remove func(name string)) {
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
}

// Purge any empty directories
func (c *Cache) purgeEmptyDirs() {
	c._purgeEmptyDirs(c.removeDir)
}

func (c *Cache) _purgeEmptyDirs(removeDir func(name string) bool) {
	c.itemMu.Lock()
	defer c.itemMu.Unlock()
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

// This is a cacheItem with a name for sorting
type cacheNamedItem struct {
	name string
	item *cacheItem
}
type cacheNamedItems []cacheNamedItem

func (v cacheNamedItems) Len() int           { return len(v) }
func (v cacheNamedItems) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v cacheNamedItems) Less(i, j int) bool { return v[i].item.atime.Before(v[j].item.atime) }

// Remove any files that are over quota starting from the
// oldest first
func (c *Cache) purgeOverQuota(quota int64) {
	c._purgeOverQuota(quota, c.Remove)
}

func (c *Cache) _purgeOverQuota(quota int64, remove func(name string)) {
	c.itemMu.Lock()
	defer c.itemMu.Unlock()

	if quota <= 0 || c.used < quota {
		return
	}

	var items cacheNamedItems

	// Make a slice of unused files
	for name, item := range c.item {
		if item.isFile && item.opens == 0 {
			items = append(items, cacheNamedItem{
				name: name,
				item: item,
			})
		}
	}
	sort.Sort(items)

	// Remove items until the quota is OK
	for _, item := range items {
		if c.used < quota {
			break
		}
		remove(item.name)
		// Remove the entry
		delete(c.item, item.name)
		c.used -= item.item.size
	}
}

// clean empties the cache of stuff if it can
func (c *Cache) clean() {
	// Cache may be empty so end
	_, err := os.Stat(c.root)
	if os.IsNotExist(err) {
		return
	}

	c.itemMu.Lock()
	oldItems, oldUsed := len(c.item), fs.SizeSuffix(c.used)
	c.itemMu.Unlock()

	// first walk the FS to update the atimes and sizes
	err = c.updateStats()
	if err != nil {
		fs.Errorf(nil, "Error traversing cache %q: %v", c.root, err)
	}

	// Remove any files that are over age
	c.purgeOld(c.opt.CacheMaxAge)

	// Now remove any files that are over quota starting from the
	// oldest first
	c.purgeOverQuota(int64(c.opt.CacheMaxSize))

	// Remove any empty directories
	c.purgeEmptyDirs()

	// Stats
	c.itemMu.Lock()
	newItems, newUsed := len(c.item), fs.SizeSuffix(c.used)
	c.itemMu.Unlock()

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

// copy an object to or from the remote while accounting for it
func copyObj(f fs.Fs, dst fs.Object, remote string, src fs.Object) (newDst fs.Object, err error) {
	if operations.NeedTransfer(context.TODO(), dst, src) {
		newDst, err = operations.Copy(context.TODO(), f, dst, remote, src)
	} else {
		newDst = dst
	}
	return newDst, err
}

// Check the local file is up to date in the cache
func (c *Cache) Check(ctx context.Context, o fs.Object, remote string) error {
	cacheObj, err := c.fcache.NewObject(ctx, remote)
	if err == nil && cacheObj != nil {
		_, err = copyObj(c.fcache, cacheObj, remote, o)
		if err != nil {
			return errors.Wrap(err, "failed to update cached file")
		}
	}
	return nil
}

// Fetch fetches the object to the cache file
func (c *Cache) Fetch(ctx context.Context, o fs.Object, remote string) error {
	_, err := copyObj(c.fcache, nil, remote, o)
	return err
}

// Store stores the local cache file to the remote object, returning
// the new remote object. objOld is the old object if known.
func (c *Cache) Store(ctx context.Context, objOld fs.Object, remote string) (fs.Object, error) {
	// Transfer the temp file to the remote
	cacheObj, err := c.fcache.NewObject(ctx, remote)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find cache file")
	}

	if objOld != nil {
		remote = objOld.Remote() // use the path of the actual object if available
	}
	o, err := copyObj(c.fremote, objOld, remote, cacheObj)
	if err != nil {
		return nil, errors.Wrap(err, "failed to transfer file from cache to remote")
	}
	return o, nil
}
