package vfscache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/ranges"
)

// NB as Cache and Item are tightly linked it is necessary to have a
// total lock ordering between them. So Cache.mu must always be
// taken before Item.mu to avoid deadlocks.
//
// Cache may call into Item but care is needed if Item calls Cache
//
// A lot of the Cache methods do not require locking, these include
//
// - Cache.toOSPath
// - Cache.toOSPathMeta
// - Cache.mkdir
// - Cache.objectFingerprint

// NB Item and downloader are tightly linked so it is necessary to
// have a total lock ordering between them. downloader.mu must always
// be taken before Item.mu. downloader may call into Item but Item may
// **not** call downloader methods with Item.mu held, except for
//
// - downloader.running

// Item is stored in the item map
//
// These are written to the backing store to store status
type Item struct {
	// read only
	c *Cache // cache this is part of

	mu         sync.Mutex  // protect the variables
	name       string      // name in the VFS
	opens      int         // number of times file is open
	downloader *downloader // if the file is being downloaded to cache
	o          fs.Object   // object we are caching - may be nil
	fd         *os.File    // handle we are using to read and write to the file
	metaDirty  bool        // set if the info needs writeback
	info       Info        // info about the file to persist to backing store

}

// Info is persisted to backing store
type Info struct {
	ModTime     time.Time     // last time file was modified
	ATime       time.Time     // last time file was accessed
	Size        int64         // size of the file
	Rs          ranges.Ranges // which parts of the file are present
	Fingerprint string        // fingerprint of remote object
	Dirty       bool          // set if the backing file has been modifed
}

// Items are a slice of *Item ordered by ATime
type Items []*Item

func (v Items) Len() int      { return len(v) }
func (v Items) Swap(i, j int) { v[i], v[j] = v[j], v[i] }
func (v Items) Less(i, j int) bool {
	if i == j {
		return false
	}
	iItem := v[i]
	jItem := v[j]
	iItem.mu.Lock()
	defer iItem.mu.Unlock()
	jItem.mu.Lock()
	defer jItem.mu.Unlock()

	return iItem.info.ATime.Before(jItem.info.ATime)
}

// clean the item after its cache file has been deleted
func (info *Info) clean() {
	*info = Info{}
	info.ModTime = time.Now()
	info.ATime = info.ModTime
}

// StoreFn is called back with an object after it has been uploaded
type StoreFn func(fs.Object)

// newItem returns an item for the cache
func newItem(c *Cache, name string) (item *Item) {
	now := time.Now()
	item = &Item{
		c:    c,
		name: name,
		info: Info{
			ModTime: now,
			ATime:   now,
		},
	}

	// check the cache file exists
	osPath := c.toOSPath(name)
	fi, statErr := os.Stat(osPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			item._removeMeta("cache file doesn't exist")
		} else {
			item._remove(fmt.Sprintf("failed to stat cache file: %v", statErr))
		}
	}

	// Try to load the metadata
	exists, err := item.load()
	if !exists {
		item._removeFile("metadata doesn't exist")
	} else if err != nil {
		item._remove(fmt.Sprintf("failed to load metadata: %v", err))
	}

	// Get size estimate (which is best we can do until Open() called)
	if statErr == nil {
		item.info.Size = fi.Size()
	}
	return item
}

// inUse returns true if the item is open or dirty
func (item *Item) inUse() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.opens != 0 || item.metaDirty || item.info.Dirty
}

// getATime returns the ATime of the item
func (item *Item) getATime() time.Time {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.info.ATime
}

// getName returns the name of the item
func (item *Item) getName() string {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.name
}

// getDiskSize returns the size on disk (approximately) of the item
//
// We return the sizes of the chunks we have fetched, however there is
// likely to be some overhead which we are not taking into account.
func (item *Item) getDiskSize() int64 {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.info.Rs.Size()
}

// load reads an item from the disk or returns nil if not found
func (item *Item) load() (exists bool, err error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	osPathMeta := item.c.toOSPathMeta(item.name) // No locking in Cache
	in, err := os.Open(osPathMeta)
	if err != nil {
		if os.IsNotExist(err) {
			return false, err
		}
		return true, errors.Wrap(err, "vfs cache item: failed to read metadata")
	}
	defer fs.CheckClose(in, &err)
	decoder := json.NewDecoder(in)
	err = decoder.Decode(&item.info)
	if err != nil {
		return true, errors.Wrap(err, "vfs cache item: corrupt metadata")
	}
	item.metaDirty = false
	return true, nil
}

// save writes an item to the disk
//
// call with the lock held
func (item *Item) _save() (err error) {
	osPathMeta := item.c.toOSPathMeta(item.name) // No locking in Cache
	out, err := os.Create(osPathMeta)
	if err != nil {
		return errors.Wrap(err, "vfs cache item: failed to write metadata")
	}
	defer fs.CheckClose(out, &err)
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "\t")
	err = encoder.Encode(item.info)
	if err != nil {
		return errors.Wrap(err, "vfs cache item: failed to encode metadata")
	}
	item.metaDirty = false
	return nil
}

// truncate the item to the given size, creating it if necessary
//
// this does not mark the object as dirty
//
// call with the lock held
func (item *Item) _truncate(size int64) (err error) {
	if size < 0 {
		// FIXME ignore unknown length files
		return nil
	}

	// Use open handle if available
	fd := item.fd
	if fd == nil {
		osPath := item.c.toOSPath(item.name) // No locking in Cache
		fd, err = file.OpenFile(osPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return errors.Wrap(err, "vfs item truncate: failed to open cache file")
		}

		defer fs.CheckClose(fd, &err)

		err = file.SetSparse(fd)
		if err != nil {
			fs.Debugf(item.name, "vfs item truncate: failed to set as a sparse file: %v", err)
		}
	}

	fs.Debugf(item.name, "vfs cache: truncate to size=%d", size)

	err = fd.Truncate(size)
	if err != nil {
		return errors.Wrap(err, "vfs truncate: failed to truncate")
	}

	item.info.Size = size

	return nil
}

// Truncate the item to the current size, creating if necessary
//
// This does not mark the object as dirty
//
// call with the lock held
func (item *Item) _truncateToCurrentSize() (err error) {
	size, err := item._getSize()
	if err != nil && !os.IsNotExist(errors.Cause(err)) {
		return errors.Wrap(err, "truncate to current size")
	}
	if size < 0 {
		// FIXME ignore unknown length files
		return nil
	}
	err = item._truncate(size)
	if err != nil {
		return err
	}
	return nil
}

// Truncate the item to the given size, creating it if necessary
//
// If the new size is shorter than the existing size then the object
// will be shortened and marked as dirty.
//
// If the new size is longer than the old size then the object will be
// extended and the extended data will be filled with zeros. The
// object will be marked as dirty in this case also.
func (item *Item) Truncate(size int64) (err error) {
	item.mu.Lock()
	defer item.mu.Unlock()

	// Read old size
	oldSize, err := item._getSize()
	if err != nil {
		if !os.IsNotExist(errors.Cause(err)) {
			return errors.Wrap(err, "truncate failed to read size")
		}
		oldSize = 0
	}

	err = item._truncate(size)
	if err != nil {
		return err
	}

	changed := true
	if size > oldSize {
		// Truncate extends the file in which case all new bytes are
		// read as zeros. In this case we must show we have written to
		// the new parts of the file.
		item._written(oldSize, size)
	} else if size < oldSize {
		// Truncate shrinks the file so clip the downloaded ranges
		item.info.Rs = item.info.Rs.Intersection(ranges.Range{Pos: 0, Size: size})
	} else {
		changed = item.o == nil
	}
	if changed {
		item._dirty()
	}

	return nil
}

// _getSize gets the current size of the item and updates item.info.Size
//
// Call with mutex held
func (item *Item) _getSize() (size int64, err error) {
	var fi os.FileInfo
	if item.fd != nil {
		fi, err = item.fd.Stat()
	} else {
		osPath := item.c.toOSPath(item.name) // No locking in Cache
		fi, err = os.Stat(osPath)
	}
	if err != nil {
		if os.IsNotExist(err) && item.o != nil {
			size = item.o.Size()
			err = nil
		}
	} else {
		size = fi.Size()
	}
	if err == nil {
		item.info.Size = size
	}
	return size, err
}

// GetSize gets the current size of the item
func (item *Item) GetSize() (size int64, err error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._getSize()
}

// _exists returns whether the backing file for the item exists or not
//
// call with mutex held
func (item *Item) _exists() bool {
	osPath := item.c.toOSPath(item.name) // No locking in Cache
	_, err := os.Stat(osPath)
	return err == nil
}

// Exists returns whether the backing file for the item exists or not
func (item *Item) Exists() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._exists()
}

// _dirty marks the item as changed and needing writeback
//
// call with lock held
func (item *Item) _dirty() {
	item.info.ModTime = time.Now()
	item.info.ATime = item.info.ModTime
	item.metaDirty = true
	if !item.info.Dirty {
		item.info.Dirty = true
		err := item._save()
		if err != nil {
			fs.Errorf(item.name, "vfs cache: failed to save item info: %v", err)
		}
	}
}

// Dirty marks the item as changed and needing writeback
func (item *Item) Dirty() {
	item.mu.Lock()
	item._dirty()
	item.mu.Unlock()
}

// IsDirty returns true if the item is dirty
func (item *Item) IsDirty() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.metaDirty || item.info.Dirty
}

// Open the local file from the object passed in (which may be nil)
// which implies we are about to create the file
func (item *Item) Open(o fs.Object) (err error) {
	defer log.Trace(o, "item=%p", item)("err=%v", &err)
	item.mu.Lock()
	defer item.mu.Unlock()

	item.info.ATime = time.Now()
	item.opens++

	osPath, err := item.c.mkdir(item.name) // No locking in Cache
	if err != nil {
		return errors.Wrap(err, "vfs cache item: open mkdir failed")
	}

	err = item._checkObject(o)
	if err != nil {
		return errors.Wrap(err, "vfs cache item: check object failed")
	}

	if item.opens != 1 {
		return nil
	}
	if item.fd != nil {
		return errors.New("vfs cache item: internal error: didn't Close file")
	}

	fd, err := file.OpenFile(osPath, os.O_RDWR, 0600)
	if err != nil {
		return errors.Wrap(err, "vfs cache item: open failed")
	}
	err = file.SetSparse(fd)
	if err != nil {
		fs.Debugf(item.name, "vfs cache item: failed to set as a sparse file: %v", err)
	}
	item.fd = fd

	err = item._save()
	if err != nil {
		return err
	}

	// Unlock the Item.mu so we can call some methods which take Cache.mu
	item.mu.Unlock()

	// Ensure this item is in the cache. It is possible a cache
	// expiry has run and removed the item if it had no opens so
	// we put it back here. If there was an item with opens
	// already then return an error. This shouldn't happen because
	// there should only be one vfs.File with a pointer to this
	// item in at a time.
	oldItem := item.c.put(item.name, item) // LOCKING in Cache method
	if oldItem != nil {
		oldItem.mu.Lock()
		if oldItem.opens != 0 {
			// Put the item back and return an error
			item.c.put(item.name, oldItem) // LOCKING in Cache method
			err = errors.Errorf("internal error: item %q already open in the cache", item.name)
		}
		oldItem.mu.Unlock()
	}

	// Relock the Item.mu for the return
	item.mu.Lock()

	return err
}

// Store stores the local cache file to the remote object, returning
// the new remote object. objOld is the old object if known.
//
// Call with lock held
func (item *Item) _store(ctx context.Context, storeFn StoreFn) (err error) {
	defer log.Trace(item.name, "item=%p", item)("err=%v", &err)

	// Ensure any segments not transferred are brought in
	err = item._ensure(0, item.info.Size)
	if err != nil {
		return errors.Wrap(err, "vfs cache: failed to download missing parts of cache file")
	}

	// Transfer the temp file to the remote
	cacheObj, err := item.c.fcache.NewObject(ctx, item.name)
	if err != nil {
		return errors.Wrap(err, "vfs cache: failed to find cache file")
	}

	item.mu.Unlock()
	o, err := operations.Copy(ctx, item.c.fremote, item.o, item.name, cacheObj)
	item.mu.Lock()
	if err != nil {
		return errors.Wrap(err, "vfs cache: failed to transfer file from cache to remote")
	}
	item.o = o
	item._updateFingerprint()
	item.info.Dirty = false
	err = item._save()
	if err != nil {
		fs.Errorf(item.name, "Failed to write metadata file: %v", err)
	}
	if storeFn != nil && item.o != nil {
		// Write the object back to the VFS layer as last
		// thing we do with mutex unlocked
		item.mu.Unlock()
		storeFn(item.o)
		item.mu.Lock()
	}
	return nil
}

// Store stores the local cache file to the remote object, returning
// the new remote object. objOld is the old object if known.
func (item *Item) store(ctx context.Context, storeFn StoreFn) (err error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._store(ctx, storeFn)
}

// Close the cache file
func (item *Item) Close(storeFn StoreFn) (err error) {
	defer log.Trace(item.o, "Item.Close")("err=%v", &err)
	var (
		downloader    *downloader
		syncWriteBack = item.c.opt.WriteBack <= 0
	)
	// FIXME need to unlock to kill downloader - should we
	// re-arrange locking so this isn't necessary?  maybe
	// downloader should use the item mutex for locking? or put a
	// finer lock on Rs?
	//
	// close downloader with mutex unlocked
	// downloader.Write calls ensure which needs the lock
	defer func() {
		if downloader != nil {
			closeErr := downloader.close(nil)
			if closeErr != nil && err == nil {
				err = closeErr
			}
		}
		// save the metadata once more since it may be dirty
		// after the downloader
		saveErr := item._save()
		if saveErr != nil && err == nil {
			err = errors.Wrap(saveErr, "close failed to save item")
		}
	}()
	item.mu.Lock()
	defer item.mu.Unlock()

	item.info.ATime = time.Now()
	item.opens--

	if item.opens < 0 {
		return os.ErrClosed
	} else if item.opens > 0 {
		return nil
	}

	// Update the size on close
	_, _ = item._getSize()
	err = item._save()
	if err != nil {
		return errors.Wrap(err, "close failed to save item")
	}

	// close the downloader
	downloader = item.downloader
	item.downloader = nil

	// close the file handle
	if item.fd == nil {
		return errors.New("vfs cache item: internal error: didn't Open file")
	}
	err = item.fd.Close()
	item.fd = nil

	// if the item hasn't been changed but has been completed then
	// set the modtime from the object otherwise set it from the info
	if item._exists() {
		if !item.info.Dirty && item.o != nil {
			item._setModTime(item.o.ModTime(context.Background()))
		} else {
			item._setModTime(item.info.ModTime)
		}
	}

	// upload the file to backing store if changed
	if item.info.Dirty {
		fs.Debugf(item.name, "item changed - writeback in %v", item.c.opt.WriteBack)
		if syncWriteBack {
			// do synchronous writeback
			err = item._store(context.Background(), storeFn)
		} else {
			// asynchronous writeback
			item.c.writeback.add(item, storeFn)
		}
	}

	return err
}

// reload is called with valid items recovered from a cache reload.
//
// it is called before the cache has started so opens will be 0 and
// metaDirty will be false.
func (item *Item) reload(ctx context.Context) error {
	item.mu.Lock()
	dirty := item.info.Dirty
	item.mu.Unlock()
	if !dirty {
		return nil
	}
	// see if the object still exists
	obj, _ := item.c.fremote.NewObject(ctx, item.name)
	// open the file with the object (or nil)
	err := item.Open(obj)
	if err != nil {
		return err
	}
	// close the file to execute the writeback if needed
	err = item.Close(nil)
	if err != nil {
		return err
	}
	return nil
}

// check the fingerprint of an object and update the item or delete
// the cached file accordingly
//
// It ensures the file is the correct size for the object
//
// call with lock held
func (item *Item) _checkObject(o fs.Object) error {
	if o == nil {
		if item.info.Fingerprint != "" {
			// no remote object && local object
			// remove local object
			item._remove("stale (remote deleted)")
		} else {
			// no remote object && no local object
			// OK
		}
	} else {
		remoteFingerprint := fs.Fingerprint(context.TODO(), o, false)
		fs.Debugf(item.name, "vfs cache: checking remote fingerprint %q against cached fingerprint %q", remoteFingerprint, item.info.Fingerprint)
		if item.info.Fingerprint != "" {
			// remote object && local object
			if remoteFingerprint != item.info.Fingerprint {
				fs.Debugf(item.name, "vfs cache: removing cached entry as stale (remote fingerprint %q != cached fingerprint %q)", remoteFingerprint, item.info.Fingerprint)
				item._remove("stale (remote is different)")
			}
		} else {
			// remote object && no local object
			// Set fingerprint
			item.info.Fingerprint = remoteFingerprint
			item.metaDirty = true
		}
		item.info.Size = o.Size()
	}
	item.o = o

	err := item._truncateToCurrentSize()
	if err != nil {
		return errors.Wrap(err, "vfs cache item: open truncate failed")
	}

	return nil
}

// remove the cached file
//
// call with lock held
func (item *Item) _removeFile(reason string) {
	osPath := item.c.toOSPath(item.name) // No locking in Cache
	err := os.Remove(osPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fs.Errorf(item.name, "Failed to remove cache file as %s: %v", reason, err)
		}
	} else {
		fs.Infof(item.name, "Removed cache file as %s", reason)
	}
}

// remove the metadata
//
// call with lock held
func (item *Item) _removeMeta(reason string) {
	osPathMeta := item.c.toOSPathMeta(item.name) // No locking in Cache
	err := os.Remove(osPathMeta)
	if err != nil {
		if !os.IsNotExist(err) {
			fs.Errorf(item.name, "Failed to remove metadata from cache as %s: %v", reason, err)
		}
	} else {
		fs.Infof(item.name, "Removed metadata from cache as %s", reason)
	}
}

// remove the cached file and empty the metadata
//
// call with lock held
func (item *Item) _remove(reason string) {
	item.info.clean()
	item.metaDirty = false
	item._removeFile(reason)
	item._removeMeta(reason)
}

// remove the cached file and empty the metadata
func (item *Item) remove(reason string) {
	item.mu.Lock()
	item._remove(reason)
	item.mu.Unlock()
}

// create a downloader for the item
//
// call with item mutex held
func (item *Item) _newDownloader() (err error) {
	// If no cached object then can't download
	if item.o == nil {
		return errors.New("vfs cache: internal error: tried to download nil object")
	}
	// If downloading the object already stop the downloader and restart it
	if item.downloader != nil {
		item.mu.Unlock()
		_ = item.downloader.close(nil)
		item.mu.Lock()
		item.downloader = nil
	}
	item.downloader, err = newDownloader(item, item.c.fremote, item.name, item.o)
	return err
}

// _present returns true if the whole file has been downloaded
//
// call with the lock held
func (item *Item) _present() bool {
	if item.downloader != nil && item.downloader.running() {
		return false
	}
	return item.info.Rs.Present(ranges.Range{Pos: 0, Size: item.info.Size})
}

// ensure the range from offset, size is present in the backing file
//
// call with the item lock held
func (item *Item) _ensure(offset, size int64) (err error) {
	defer log.Trace(item.name, "offset=%d, size=%d", offset, size)("err=%v", &err)
	if offset+size > item.info.Size {
		size = item.info.Size - offset
	}
	r := ranges.Range{Pos: offset, Size: size}
	present := item.info.Rs.Present(r)
	downloader := item.downloader
	fs.Debugf(nil, "looking for range=%+v in %+v - present %v", r, item.info.Rs, present)
	if present {
		return nil
	}
	// FIXME pass in offset here to decide to seek?
	err = item._newDownloader()
	if err != nil {
		return errors.Wrap(err, "Ensure: failed to start downloader")
	}
	downloader = item.downloader
	if downloader == nil {
		return errors.New("internal error: downloader is nil")
	}
	if !downloader.running() {
		// FIXME need to make sure we start in the correct place because some of offset,size might exist
		// FIXME this could stop an old download
		item.mu.Unlock()
		err = downloader.start(offset)
		item.mu.Lock()
		if err != nil {
			return errors.Wrap(err, "Ensure: failed to run downloader")
		}
	}
	item.mu.Unlock()
	defer item.mu.Lock()
	return item.downloader.ensure(r)
}

// _written marks the (offset, size) as present in the backing file
//
// This is called by the downloader downloading file segments and the
// vfs layer writing to the file.
//
// call with lock held
func (item *Item) _written(offset, size int64) {
	defer log.Trace(item.name, "offset=%d, size=%d", offset, size)("")
	item.info.Rs.Insert(ranges.Range{Pos: offset, Size: offset + size})
	item.metaDirty = true
}

// update the fingerprint of the object if any
//
// call with lock held
func (item *Item) _updateFingerprint() {
	if item.o == nil {
		return
	}
	oldFingerprint := item.info.Fingerprint
	item.info.Fingerprint = fs.Fingerprint(context.TODO(), item.o, false)
	if oldFingerprint != item.info.Fingerprint {
		fs.Debugf(item.o, "fingerprint now %q", item.info.Fingerprint)
		item.metaDirty = true
	}
}

// setModTime of the cache file
//
// call with lock held
func (item *Item) _setModTime(modTime time.Time) {
	fs.Debugf(item.name, "vfs cache: setting modification time to %v", modTime)
	osPath := item.c.toOSPath(item.name) // No locking in Cache
	err := os.Chtimes(osPath, modTime, modTime)
	if err != nil {
		fs.Errorf(item.name, "Failed to set modification time of cached file: %v", err)
	}
}

// setModTime of the cache file and in the Item
func (item *Item) setModTime(modTime time.Time) {
	defer log.Trace(item.name, "modTime=%v", modTime)("")
	item.mu.Lock()
	item._updateFingerprint()
	item._setModTime(modTime)
	err := item._save()
	if err != nil {
		fs.Errorf(item.name, "vfs cache: setModTime: failed to save item info: %v", err)
	}
	item.mu.Unlock()
}

// ReadAt bytes from the file at off
func (item *Item) ReadAt(b []byte, off int64) (n int, err error) {
	item.mu.Lock()
	if item.fd == nil {
		item.mu.Unlock()
		return 0, errors.New("vfs cache item ReadAt: internal error: didn't Open file")
	}
	err = item._ensure(off, int64(len(b)))
	if err != nil {
		item.mu.Unlock()
		return n, err
	}
	item.info.ATime = time.Now()
	item.mu.Unlock()
	// Do the reading with Item.mu unlocked
	return item.fd.ReadAt(b, off)
}

// WriteAt bytes to the file at off
func (item *Item) WriteAt(b []byte, off int64) (n int, err error) {
	item.mu.Lock()
	if item.fd == nil {
		item.mu.Unlock()
		return 0, errors.New("vfs cache item WriteAt: internal error: didn't Open file")
	}
	item.mu.Unlock()
	// Do the writing with Item.mu unlocked
	n, err = item.fd.WriteAt(b, off)
	item.mu.Lock()
	item._written(off, int64(n))
	if n > 0 {
		item._dirty()
	}
	end := off + int64(n)
	if end > item.info.Size {
		item.info.Size = end
	}
	item.mu.Unlock()
	return n, err
}

// Sync commits the current contents of the file to stable storage. Typically,
// this means flushing the file system's in-memory copy of recently written
// data to disk.
func (item *Item) Sync() (err error) {
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.fd == nil {
		return errors.New("vfs cache item sync: internal error: didn't Open file")
	}
	// sync the file and the metadata to disk
	err = item.fd.Sync()
	if err != nil {
		return errors.Wrap(err, "vfs cache item sync: failed to sync file")
	}
	err = item._save()
	if err != nil {
		return errors.Wrap(err, "vfs cache item sync: failed to sync metadata")
	}
	return nil
}

// rename the item
func (item *Item) rename(name string, newName string, newObj fs.Object) (err error) {
	var downloader *downloader
	// close downloader with mutex unlocked
	defer func() {
		if downloader != nil {
			_ = downloader.close(nil)
		}
	}()

	item.mu.Lock()
	defer item.mu.Unlock()

	// stop downloader
	downloader = item.downloader
	item.downloader = nil

	// Set internal state
	item.name = newName
	item.o = newObj

	// Rename cache file if it exists
	err = rename(item.c.toOSPath(name), item.c.toOSPath(newName)) // No locking in Cache
	if err != nil {
		return err
	}

	// Rename meta file if it exists
	err = rename(item.c.toOSPathMeta(name), item.c.toOSPathMeta(newName)) // No locking in Cache
	if err != nil {
		return err
	}

	return nil
}
