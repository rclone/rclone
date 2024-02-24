package vfscache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/file"
	"github.com/rclone/rclone/lib/ranges"
	"github.com/rclone/rclone/vfs/vfscache/downloaders"
	"github.com/rclone/rclone/vfs/vfscache/writeback"
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
// - Cache.createItemDir
// - Cache.objectFingerprint
// - Cache.AddVirtual

// NB Item and downloader are tightly linked so it is necessary to
// have a total lock ordering between them. downloader.mu must always
// be taken before Item.mu. downloader may call into Item but Item may
// **not** call downloader methods with Item.mu held

// NB Item and writeback are tightly linked so it is necessary to
// have a total lock ordering between them. writeback.mu must always
// be taken before Item.mu. writeback may call into Item but Item may
// **not** call writeback methods with Item.mu held

// LL Item reset is invoked by cache cleaner for synchronous recovery
// from ENOSPC errors. The reset operation removes the cache file and
// closes/reopens the downloaders.  Although most parts of reset and
// other item operations are done with the item mutex held, the mutex
// is released during fd.WriteAt and downloaders calls. We use preAccess
// and postAccess calls to serialize reset and other item operations.

// Item is stored in the item map
//
// The Info field is written to the backing store to store status
type Item struct {
	// read only
	c               *Cache                   // cache this is part of
	mu              sync.Mutex               // protect the variables
	cond            sync.Cond                // synchronize with cache cleaner
	name            string                   // name in the VFS
	opens           int                      // number of times file is open
	downloaders     *downloaders.Downloaders // a record of the downloaders in action - may be nil
	o               fs.Object                // object we are caching - may be nil
	fd              *os.File                 // handle we are using to read and write to the file
	info            Info                     // info about the file to persist to backing store
	writeBackID     writeback.Handle         // id of any writebacks in progress
	pendingAccesses int                      // number of threads - cache reset not allowed if not zero
	modified        bool                     // set if the file has been modified since the last Open
	beingReset      bool                     // cache cleaner is resetting the cache file, access not allowed
}

// Info is persisted to backing store
type Info struct {
	ModTime     time.Time     // last time file was modified
	ATime       time.Time     // last time file was accessed
	Size        int64         // size of the file
	Rs          ranges.Ranges // which parts of the file are present
	Fingerprint string        // fingerprint of remote object
	Dirty       bool          // set if the backing file has been modified
}

// Items are a slice of *Item ordered by ATime
type Items []*Item

// ResetResult reports the actual action taken in the Reset function and reason
type ResetResult int

// Constants used to report actual action taken in the Reset function and reason
const (
	SkippedDirty         ResetResult = iota // Dirty item cannot be reset
	SkippedPendingAccess                    // Reset pending access can lead to deadlock
	SkippedEmpty                            // Reset empty item does not save space
	RemovedNotInUse                         // Item not used. Remove instead of reset
	ResetFailed                             // Reset failed with an error
	ResetComplete                           // Reset completed successfully
)

func (rr ResetResult) String() string {
	return [...]string{"Dirty item skipped", "In-access item skipped", "Empty item skipped",
		"Not-in-use item removed", "Item reset failed", "Item reset completed"}[rr]
}

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
	item.cond = sync.Cond{L: &item.mu}
	// check the cache file exists
	osPath := c.toOSPath(name)
	fi, statErr := os.Stat(osPath)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			item._removeMeta("cache file doesn't exist")
		} else {
			item.remove(fmt.Sprintf("failed to stat cache file: %v", statErr))
		}
	}

	// Try to load the metadata
	exists, err := item.load()
	if !exists {
		item._removeFile("metadata doesn't exist")
	} else if err != nil {
		item.remove(fmt.Sprintf("failed to load metadata: %v", err))
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
	return item.opens != 0 || item.info.Dirty
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
		return true, fmt.Errorf("vfs cache item: failed to read metadata: %w", err)
	}
	defer fs.CheckClose(in, &err)
	decoder := json.NewDecoder(in)
	err = decoder.Decode(&item.info)
	if err != nil {
		return true, fmt.Errorf("vfs cache item: corrupt metadata: %w", err)
	}
	return true, nil
}

// save writes an item to the disk
//
// call with the lock held
func (item *Item) _save() (err error) {
	osPathMeta := item.c.toOSPathMeta(item.name) // No locking in Cache
	out, err := os.Create(osPathMeta)
	if err != nil {
		return fmt.Errorf("vfs cache item: failed to write metadata: %w", err)
	}
	defer fs.CheckClose(out, &err)
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "\t")
	err = encoder.Encode(item.info)
	if err != nil {
		return fmt.Errorf("vfs cache item: failed to encode metadata: %w", err)
	}
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
		// If the metadata says we have some blocks cached then the
		// file should exist, so open without O_CREATE
		oFlags := os.O_WRONLY
		if item.info.Rs.Size() == 0 {
			oFlags |= os.O_CREATE
		}
		osPath := item.c.toOSPath(item.name) // No locking in Cache
		fd, err = file.OpenFile(osPath, oFlags, 0600)
		if err != nil && os.IsNotExist(err) {
			// If the metadata has info but the file doesn't
			// not exist then it has been externally removed
			fs.Errorf(item.name, "vfs cache: detected external removal of cache file")
			item.info.Rs = nil      // show we have no blocks cached
			item.info.Dirty = false // file can't be dirty if it doesn't exist
			item._removeMeta("cache file externally deleted")
			fd, err = file.OpenFile(osPath, os.O_CREATE|os.O_WRONLY, 0600)
		}
		if err != nil {
			return fmt.Errorf("vfs cache: truncate: failed to open cache file: %w", err)
		}

		defer fs.CheckClose(fd, &err)

		err = file.SetSparse(fd)
		if err != nil {
			fs.Errorf(item.name, "vfs cache: truncate: failed to set as a sparse file: %v", err)
		}
	}

	// Check to see what the current size is, and don't truncate
	// if it is already the correct size.
	//
	// Apparently Windows Defender likes to check executables each
	// time they are modified, and truncating a file to its
	// existing size is enough to trigger the Windows Defender
	// scan. This was causing a big slowdown for operations which
	// opened and closed the file a lot, such as looking at
	// properties on an executable.
	fi, err := fd.Stat()
	if err == nil && fi.Size() == size {
		fs.Debugf(item.name, "vfs cache: truncate to size=%d (not needed as size correct)", size)
	} else {
		fs.Debugf(item.name, "vfs cache: truncate to size=%d", size)

		err = fd.Truncate(size)
		if err != nil {
			return fmt.Errorf("vfs cache: truncate: %w", err)
		}
	}

	item.info.Size = size

	return nil
}

// Truncate the item to the current size, creating if necessary
//
// This does not mark the object as dirty.
//
// call with the lock held
func (item *Item) _truncateToCurrentSize() (err error) {
	size, err := item._getSize()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("truncate to current size: %w", err)
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
	item.preAccess()
	defer item.postAccess()
	item.mu.Lock()
	defer item.mu.Unlock()

	if item.fd == nil {
		return errors.New("vfs cache item truncate: internal error: didn't Open file")
	}

	// Read old size
	oldSize, err := item._getSize()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("truncate failed to read size: %w", err)
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

// _stat gets the current stat of the backing file
//
// Call with mutex held
func (item *Item) _stat() (fi os.FileInfo, err error) {
	if item.fd != nil {
		return item.fd.Stat()
	}
	osPath := item.c.toOSPath(item.name) // No locking in Cache
	return os.Stat(osPath)
}

// _getSize gets the current size of the item and updates item.info.Size
//
// Call with mutex held
func (item *Item) _getSize() (size int64, err error) {
	fi, err := item._stat()
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

// GetName gets the vfs name of the item
func (item *Item) GetName() (name string) {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.name
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
	if !item.modified {
		item.modified = true
		item.mu.Unlock()
		item.c.writeback.Remove(item.writeBackID)
		item.mu.Lock()
	}
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
	item.preAccess()
	defer item.postAccess()
	item.mu.Lock()
	item._dirty()
	item.mu.Unlock()
}

// IsDirty returns true if the item data is dirty
func (item *Item) IsDirty() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.info.Dirty
}

// Create the cache file and store the metadata on disk
// Called with item.mu locked
func (item *Item) _createFile(osPath string) (err error) {
	if item.fd != nil {
		return errors.New("vfs cache item: internal error: didn't Close file")
	}
	item.modified = false
	// t0 := time.Now()
	fd, err := file.OpenFile(osPath, os.O_RDWR, 0600)
	// fs.Debugf(item.name, "OpenFile took %v", time.Since(t0))
	if err != nil {
		return fmt.Errorf("vfs cache item: open failed: %w", err)
	}
	err = file.SetSparse(fd)
	if err != nil {
		fs.Errorf(item.name, "vfs cache: failed to set as a sparse file: %v", err)
	}
	item.fd = fd

	err = item._save()
	if err != nil {
		closeErr := item.fd.Close()
		if closeErr != nil {
			fs.Errorf(item.name, "vfs cache: item.fd.Close: closeErr: %v", err)
		}
		item.fd = nil
		return fmt.Errorf("vfs cache item: _save failed: %w", err)
	}
	return err
}

// Open the local file from the object passed in.  Wraps open()
// to provide recovery from out of space error.
func (item *Item) Open(o fs.Object) (err error) {
	for retries := 0; retries < fs.GetConfig(context.TODO()).LowLevelRetries; retries++ {
		item.preAccess()
		err = item.open(o)
		item.postAccess()
		if err == nil {
			break
		}
		fs.Errorf(item.name, "vfs cache: failed to open item: %v", err)
		if !fserrors.IsErrNoSpace(err) && err.Error() != "no space left on device" {
			fs.Errorf(item.name, "Non-out-of-space error encountered during open")
			break
		}
		item.c.KickCleaner()
	}
	return err
}

// Open the local file from the object passed in (which may be nil)
// which implies we are about to create the file
func (item *Item) open(o fs.Object) (err error) {
	// defer log.Trace(o, "item=%p", item)("err=%v", &err)
	item.mu.Lock()
	defer item.mu.Unlock()

	item.info.ATime = time.Now()

	osPath, err := item.c.createItemDir(item.name) // No locking in Cache
	if err != nil {
		return fmt.Errorf("vfs cache item: createItemDir failed: %w", err)
	}

	err = item._checkObject(o)
	if err != nil {
		return fmt.Errorf("vfs cache item: check object failed: %w", err)
	}

	item.opens++
	if item.opens != 1 {
		return nil
	}

	err = item._createFile(osPath)
	if err != nil {
		item._remove("item.open failed on _createFile, remove cache data/metadata files")
		item.fd = nil
		item.opens--
		return fmt.Errorf("vfs cache item: create cache file failed: %w", err)
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
			err = fmt.Errorf("internal error: item %q already open in the cache", item.name)
		}
		oldItem.mu.Unlock()
	}

	// Relock the Item.mu for the return
	item.mu.Lock()

	// Create the downloaders
	if item.o != nil {
		item.downloaders = downloaders.New(item, item.c.opt, item.name, item.o)
	}

	return err
}

// Store stores the local cache file to the remote object, returning
// the new remote object. objOld is the old object if known.
//
// Call with lock held
func (item *Item) _store(ctx context.Context, storeFn StoreFn) (err error) {
	// defer log.Trace(item.name, "item=%p", item)("err=%v", &err)

	// Transfer the temp file to the remote
	cacheObj, err := item.c.fcache.NewObject(ctx, item.name)
	if err != nil && err != fs.ErrorObjectNotFound {
		return fmt.Errorf("vfs cache: failed to find cache file: %w", err)
	}

	// Object has disappeared if cacheObj == nil
	if cacheObj != nil {
		o, name := item.o, item.name
		item.mu.Unlock()
		o, err := operations.Copy(ctx, item.c.fremote, o, name, cacheObj)
		item.mu.Lock()
		if err != nil {
			if errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
				fs.Errorf(name, "Writeback failed: %v", err)
				return nil
			}
			return fmt.Errorf("vfs cache: failed to transfer file from cache to remote: %w", err)
		}
		item.o = o
		item._updateFingerprint()
	}

	// Write the object back to the VFS layer before we mark it as
	// clean, otherwise it will become eligible for removal which
	// can cause a deadlock
	if storeFn != nil && item.o != nil {
		fs.Debugf(item.name, "vfs cache: writeback object to VFS layer")
		// Write the object back to the VFS layer last with mutex unlocked
		o := item.o
		item.mu.Unlock()
		storeFn(o)
		item.mu.Lock()
	}

	// Show item is clean and is eligible for cache removal
	item.info.Dirty = false
	err = item._save()
	if err != nil {
		fs.Errorf(item.name, "vfs cache: failed to write metadata file: %v", err)
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
	// defer log.Trace(item.o, "Item.Close")("err=%v", &err)
	item.preAccess()
	defer item.postAccess()
	var (
		downloaders   *downloaders.Downloaders
		syncWriteBack = item.c.opt.WriteBack <= 0
	)
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

	// If the file is dirty ensure any segments not transferred
	// are brought in first.
	//
	// FIXME It would be nice to do this asynchronously however it
	// would require keeping the downloaders alive after the item
	// has been closed
	if item.info.Dirty && item.o != nil {
		err = item._ensure(0, item.info.Size)
		if err != nil {
			return fmt.Errorf("vfs cache: failed to download missing parts of cache file: %w", err)
		}
	}

	// Accumulate and log errors
	checkErr := func(e error) {
		if e != nil {
			fs.Errorf(item.o, "vfs cache: item close failed: %v", e)
			if err == nil {
				err = e
			}
		}
	}

	// Close the downloaders
	if downloaders = item.downloaders; downloaders != nil {
		item.downloaders = nil
		// FIXME need to unlock to kill downloader - should we
		// re-arrange locking so this isn't necessary?  maybe
		// downloader should use the item mutex for locking? or put a
		// finer lock on Rs?
		//
		// downloader.Write calls ensure which needs the lock
		// close downloader with mutex unlocked
		item.mu.Unlock()
		checkErr(downloaders.Close(nil))
		item.mu.Lock()
	}

	// close the file handle
	if item.fd == nil {
		checkErr(errors.New("vfs cache item: internal error: didn't Open file"))
	} else {
		checkErr(item.fd.Close())
		item.fd = nil
	}

	// save the metadata once more since it may be dirty
	// after the downloader
	checkErr(item._save())

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
		fs.Infof(item.name, "vfs cache: queuing for upload in %v", item.c.opt.WriteBack)
		if syncWriteBack {
			// do synchronous writeback
			checkErr(item._store(context.Background(), storeFn))
		} else {
			// asynchronous writeback
			item.c.writeback.SetID(&item.writeBackID)
			id := item.writeBackID
			item.mu.Unlock()
			item.c.writeback.Add(id, item.name, item.modified, func(ctx context.Context) error {
				return item.store(ctx, storeFn)
			})
			item.mu.Lock()
		}
	}

	// mark as not modified now we have uploaded or queued for upload
	item.modified = false

	return err
}

// reload is called with valid items recovered from a cache reload.
//
// If they are dirty then it makes sure they get uploaded.
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
	// put the file into the directory listings
	size, err := item._getSize()
	if err != nil {
		return fmt.Errorf("reload: failed to read size: %w", err)
	}
	err = item.c.AddVirtual(item.name, size, false)
	if err != nil {
		return fmt.Errorf("reload: failed to add virtual dir entry: %w", err)
	}
	return nil
}

// check the fingerprint of an object and update the item or delete
// the cached file accordingly
//
// If we have local modifications then they take precedence
// over a change in the remote
//
// It ensures the file is the correct size for the object.
//
// call with lock held
func (item *Item) _checkObject(o fs.Object) error {
	if o == nil {
		if item.info.Fingerprint != "" {
			// no remote object && local object
			// remove local object unless dirty
			if !item.info.Dirty {
				item._remove("stale (remote deleted)")
			} else {
				fs.Debugf(item.name, "vfs cache: remote object has gone but local object modified - keeping it")
			}
			//} else {
			// no remote object && no local object
			// OK
		}
	} else {
		remoteFingerprint := fs.Fingerprint(context.TODO(), o, item.c.opt.FastFingerprint)
		fs.Debugf(item.name, "vfs cache: checking remote fingerprint %q against cached fingerprint %q", remoteFingerprint, item.info.Fingerprint)
		if item.info.Fingerprint != "" {
			// remote object && local object
			if remoteFingerprint != item.info.Fingerprint {
				if !item.info.Dirty {
					fs.Debugf(item.name, "vfs cache: removing cached entry as stale (remote fingerprint %q != cached fingerprint %q)", remoteFingerprint, item.info.Fingerprint)
					item._remove("stale (remote is different)")
					item.info.Fingerprint = remoteFingerprint
				} else {
					fs.Debugf(item.name, "vfs cache: remote object has changed but local object modified - keeping it (remote fingerprint %q != cached fingerprint %q)", remoteFingerprint, item.info.Fingerprint)
				}
			}
		} else {
			// remote object && no local object
			// Set fingerprint
			item.info.Fingerprint = remoteFingerprint
		}
		item.info.Size = o.Size()
	}
	item.o = o

	err := item._truncateToCurrentSize()
	if err != nil {
		return fmt.Errorf("vfs cache item: open truncate failed: %w", err)
	}

	return nil
}

// WrittenBack checks to see if the item has been written back or not
func (item *Item) WrittenBack() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.info.Fingerprint != ""
}

// remove the cached file
//
// call with lock held
func (item *Item) _removeFile(reason string) {
	osPath := item.c.toOSPath(item.name) // No locking in Cache
	err := os.Remove(osPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fs.Errorf(item.name, "vfs cache: failed to remove cache file as %s: %v", reason, err)
		}
	} else {
		fs.Infof(item.name, "vfs cache: removed cache file as %s", reason)
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
			fs.Errorf(item.name, "vfs cache: failed to remove metadata from cache as %s: %v", reason, err)
		}
	} else {
		fs.Debugf(item.name, "vfs cache: removed metadata from cache as %s", reason)
	}
}

// remove the cached file and empty the metadata
//
// This returns true if the file was in the transfer queue so may not
// have completely uploaded yet.
//
// call with lock held
func (item *Item) _remove(reason string) (wasWriting bool) {
	// Cancel writeback, if any
	item.mu.Unlock()
	wasWriting = item.c.writeback.Remove(item.writeBackID)
	item.mu.Lock()
	item.info.clean()
	item._removeFile(reason)
	item._removeMeta(reason)
	return wasWriting
}

// remove the cached file and empty the metadata
//
// This returns true if the file was in the transfer queue so may not
// have completely uploaded yet.
func (item *Item) remove(reason string) (wasWriting bool) {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._remove(reason)
}

// RemoveNotInUse is called to remove cache file that has not been accessed recently
// It may also be called for removing empty cache files too when the quota is already reached.
func (item *Item) RemoveNotInUse(maxAge time.Duration, emptyOnly bool) (removed bool, spaceFreed int64) {
	item.mu.Lock()
	defer item.mu.Unlock()

	spaceFreed = 0
	removed = false

	if item.opens != 0 || item.info.Dirty {
		return
	}

	removeIt := false
	if maxAge == 0 {
		removeIt = true // quota-driven removal
	}
	if maxAge != 0 {
		cutoff := time.Now().Add(-maxAge)
		// If not locked and access time too long ago - delete the file
		accessTime := item.info.ATime
		if accessTime.Sub(cutoff) <= 0 {
			removeIt = true
		}
	}
	if removeIt {
		spaceUsed := item.info.Rs.Size()
		if !emptyOnly || spaceUsed == 0 {
			spaceFreed = spaceUsed
			removed = true
			if item._remove("Removing old cache file not in use") {
				fs.Errorf(item.name, "item removed when it was writing/uploaded")
			}
		}
	}
	return
}

// Reset is called by the cache purge functions only to reset (empty the contents) cache files that
// are not dirty.  It is used when cache space runs out and we see some ENOSPC error.
func (item *Item) Reset() (rr ResetResult, spaceFreed int64, err error) {
	item.mu.Lock()
	defer item.mu.Unlock()

	// The item is not being used now.  Just remove it instead of resetting it.
	if item.opens == 0 && !item.info.Dirty {
		spaceFreed = item.info.Rs.Size()
		if item._remove("Removing old cache file not in use") {
			fs.Errorf(item.name, "item removed when it was writing/uploaded")
		}
		return RemovedNotInUse, spaceFreed, nil
	}

	// do not reset dirty file
	if item.info.Dirty {
		return SkippedDirty, 0, nil
	}

	/* A wait on pendingAccessCnt to become 0 can lead to deadlock when an item.Open bumps
	   up the pendingAccesses count, calls item.open, which calls cache.put. The cache.put
	   operation needs the cache mutex, which is held here.  We skip this file now. The
	   caller (the cache cleaner thread) may retry resetting this item if the cache size does
	   not reduce below quota. */
	if item.pendingAccesses > 0 {
		return SkippedPendingAccess, 0, nil
	}

	/* Do not need to reset an empty cache file unless it was being reset and the reset failed.
	   Some thread(s) may be waiting on the reset's successful completion in that case. */
	if item.info.Rs.Size() == 0 && !item.beingReset {
		return SkippedEmpty, 0, nil
	}

	item.beingReset = true

	/* Error handling from this point on (setting item.fd and item.beingReset):
	   Since Reset is called by the cache cleaner thread, there is no direct way to return
	   the error to the io threads.  Set item.fd to nil upon internal errors, so that the
	   io threads will return internal errors seeing a nil fd. In the case when the error
	   is ENOSPC, keep the item in isBeingReset state and that will keep the item.ReadAt
	   waiting at its beginning. The cache purge loop will try to redo the reset after cache
	   space is made available again. This recovery design should allow most io threads to
	   eventually go through, unless large files are written/overwritten concurrently and
	   the total size of these files exceed the cache storage limit. */

	// Close the downloaders
	// Accumulate and log errors
	checkErr := func(e error) {
		if e != nil {
			fs.Errorf(item.o, "vfs cache: item reset failed: %v", e)
			if err == nil {
				err = e
			}
		}
	}

	if downloaders := item.downloaders; downloaders != nil {
		item.downloaders = nil
		// FIXME need to unlock to kill downloader - should we
		// re-arrange locking so this isn't necessary?  maybe
		// downloader should use the item mutex for locking? or put a
		// finer lock on Rs?
		//
		// downloader.Write calls ensure which needs the lock
		// close downloader with mutex unlocked
		item.mu.Unlock()
		checkErr(downloaders.Close(nil))
		item.mu.Lock()
	}

	// close the file handle
	// fd can be nil if we tried Reset and failed before because of ENOSPC during reset
	if item.fd != nil {
		checkErr(item.fd.Close())
		if err != nil {
			// Could not close the cache file
			item.beingReset = false
			item.cond.Broadcast()
			return ResetFailed, 0, err
		}
		item.fd = nil
	}

	spaceFreed = item.info.Rs.Size()

	// This should not be possible.  We get here only if cache data is not dirty.
	if item._remove("cache out of space, item is clean") {
		fs.Errorf(item.o, "vfs cache item removed when it was writing/uploaded")
	}

	// can we have an item with no dirty data (so that we can get here) and nil item.o at the same time?
	fso := item.o
	checkErr(item._checkObject(fso))
	if err != nil {
		item.beingReset = false
		item.cond.Broadcast()
		return ResetFailed, spaceFreed, err
	}

	osPath := item.c.toOSPath(item.name)
	checkErr(item._createFile(osPath))
	if err != nil {
		item._remove("cache reset failed on _createFile, removed cache data file")
		item.fd = nil // This allows a new Reset redo to have a clean state to deal with
		if !fserrors.IsErrNoSpace(err) {
			item.beingReset = false
			item.cond.Broadcast()
		}
		return ResetFailed, spaceFreed, err
	}

	// Create the downloaders
	if item.o != nil {
		item.downloaders = downloaders.New(item, item.c.opt, item.name, item.o)
	}

	/* The item will stay in the beingReset state if we get an error that prevents us from
	reaching this point.  The cache purge loop will redo the failed Reset. */
	item.beingReset = false
	item.cond.Broadcast()

	return ResetComplete, spaceFreed, err
}

// ProtectCache either waits for an ongoing cache reset to finish or increases pendingReads
// to protect against cache reset on this item while the thread potentially uses the cache file
// Cache cleaner waits until pendingReads is zero before resetting cache.
func (item *Item) preAccess() {
	item.mu.Lock()
	defer item.mu.Unlock()

	if item.beingReset {
		for {
			item.cond.Wait()
			if !item.beingReset {
				break
			}
		}
	}
	item.pendingAccesses++
}

// postAccess reduces the pendingReads count enabling cache reset upon ENOSPC
func (item *Item) postAccess() {
	item.mu.Lock()
	defer item.mu.Unlock()

	item.pendingAccesses--
	item.cond.Broadcast()
}

// _present returns true if the whole file has been downloaded
//
// call with the lock held
func (item *Item) _present() bool {
	return item.info.Rs.Present(ranges.Range{Pos: 0, Size: item.info.Size})
}

// present returns true if the whole file has been downloaded
func (item *Item) present() bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item._present()
}

// HasRange returns true if the current ranges entirely include range
func (item *Item) HasRange(r ranges.Range) bool {
	item.mu.Lock()
	defer item.mu.Unlock()
	return item.info.Rs.Present(r)
}

// FindMissing adjusts r returning a new ranges.Range which only
// contains the range which needs to be downloaded. This could be
// empty - check with IsEmpty. It also adjust this to make sure it is
// not larger than the file.
func (item *Item) FindMissing(r ranges.Range) (outr ranges.Range) {
	item.mu.Lock()
	defer item.mu.Unlock()
	outr = item.info.Rs.FindMissing(r)
	// Clip returned block to size of file
	outr.Clip(item.info.Size)
	return outr
}

// ensure the range from offset, size is present in the backing file
//
// call with the item lock held
func (item *Item) _ensure(offset, size int64) (err error) {
	// defer log.Trace(item.name, "offset=%d, size=%d", offset, size)("err=%v", &err)
	if offset+size > item.info.Size {
		size = item.info.Size - offset
	}
	r := ranges.Range{Pos: offset, Size: size}
	present := item.info.Rs.Present(r)
	/* This statement simulates a cache space error for test purpose */
	/* if present != true && item.info.Rs.Size() > 32*1024*1024 {
		return errors.New("no space left on device")
	} */
	fs.Debugf(nil, "vfs cache: looking for range=%+v in %+v - present %v", r, item.info.Rs, present)
	item.mu.Unlock()
	defer item.mu.Lock()
	if present {
		// This is a file we are writing so no downloaders needed
		if item.downloaders == nil {
			return nil
		}
		// Otherwise start the downloader for the future if required
		return item.downloaders.EnsureDownloader(r)
	}
	if item.downloaders == nil {
		// Downloaders can be nil here if the file has been
		// renamed, so need to make some more downloaders
		// OK to call downloaders constructor with item.mu held

		// item.o can also be nil under some circumstances
		// See: https://github.com/rclone/rclone/issues/6190
		// See: https://github.com/rclone/rclone/issues/6235
		if item.o == nil {
			o, err := item.c.fremote.NewObject(context.Background(), item.name)
			if err != nil {
				return err
			}
			item.o = o
		}
		item.downloaders = downloaders.New(item, item.c.opt, item.name, item.o)
	}
	return item.downloaders.Download(r)
}

// _written marks the (offset, size) as present in the backing file
//
// This is called by the downloader downloading file segments and the
// vfs layer writing to the file.
//
// This doesn't mark the item as Dirty - that the responsibility
// of the caller as we don't know here whether we are adding reads or
// writes to the cache file.
//
// call with lock held
func (item *Item) _written(offset, size int64) {
	// defer log.Trace(item.name, "offset=%d, size=%d", offset, size)("")
	item.info.Rs.Insert(ranges.Range{Pos: offset, Size: size})
}

// update the fingerprint of the object if any
//
// call with lock held
func (item *Item) _updateFingerprint() {
	if item.o == nil {
		return
	}
	oldFingerprint := item.info.Fingerprint
	item.info.Fingerprint = fs.Fingerprint(context.TODO(), item.o, item.c.opt.FastFingerprint)
	if oldFingerprint != item.info.Fingerprint {
		fs.Debugf(item.o, "vfs cache: fingerprint now %q", item.info.Fingerprint)
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
		fs.Errorf(item.name, "vfs cache: failed to set modification time of cached file: %v", err)
	}
}

// setModTime of the cache file and in the Item
func (item *Item) setModTime(modTime time.Time) {
	// defer log.Trace(item.name, "modTime=%v", modTime)("")
	item.mu.Lock()
	item._updateFingerprint()
	item._setModTime(modTime)
	item.info.ModTime = modTime
	err := item._save()
	if err != nil {
		fs.Errorf(item.name, "vfs cache: setModTime: failed to save item info: %v", err)
	}
	item.mu.Unlock()
}

// GetModTime of the cache file
func (item *Item) GetModTime() (modTime time.Time, err error) {
	// defer log.Trace(item.name, "modTime=%v", modTime)("")
	item.mu.Lock()
	defer item.mu.Unlock()
	fi, err := item._stat()
	if err == nil {
		modTime = fi.ModTime()
	}
	return modTime, nil
}

// ReadAt bytes from the file at off
func (item *Item) ReadAt(b []byte, off int64) (n int, err error) {
	n = 0
	var expBackOff int
	for retries := 0; retries < fs.GetConfig(context.TODO()).LowLevelRetries; retries++ {
		item.preAccess()
		n, err = item.readAt(b, off)
		item.postAccess()
		if err == nil || err == io.EOF {
			break
		}
		fs.Errorf(item.name, "vfs cache: failed to _ensure cache %v", err)
		if !fserrors.IsErrNoSpace(err) && err.Error() != "no space left on device" {
			fs.Debugf(item.name, "vfs cache: failed to _ensure cache %v is not out of space", err)
			break
		}
		item.c.KickCleaner()
		expBackOff = 2 << uint(retries)
		time.Sleep(time.Duration(expBackOff) * time.Millisecond) // Exponential back-off the retries
	}

	if fserrors.IsErrNoSpace(err) {
		fs.Errorf(item.name, "vfs cache: failed to _ensure cache after retries %v", err)
	}

	return n, err
}

// ReadAt bytes from the file at off
func (item *Item) readAt(b []byte, off int64) (n int, err error) {
	item.mu.Lock()
	if item.fd == nil {
		item.mu.Unlock()
		return 0, errors.New("vfs cache item ReadAt: internal error: didn't Open file")
	}
	if off < 0 {
		item.mu.Unlock()
		return 0, io.EOF
	}
	defer item.mu.Unlock()

	err = item._ensure(off, int64(len(b)))
	if err != nil {
		return 0, err
	}

	item.info.ATime = time.Now()
	// Do the reading with Item.mu unlocked and cache protected by preAccess
	n, err = item.fd.ReadAt(b, off)
	return n, err
}

// WriteAt bytes to the file at off
func (item *Item) WriteAt(b []byte, off int64) (n int, err error) {
	item.preAccess()
	defer item.postAccess()
	item.mu.Lock()
	if item.fd == nil {
		item.mu.Unlock()
		return 0, errors.New("vfs cache item WriteAt: internal error: didn't Open file")
	}
	item.mu.Unlock()
	// Do the writing with Item.mu unlocked
	n, err = item.fd.WriteAt(b, off)
	if err == nil && n != len(b) {
		err = fmt.Errorf("short write: tried to write %d but only %d written", len(b), n)
	}
	item.mu.Lock()
	item._written(off, int64(n))
	if n > 0 {
		item._dirty()
	}
	end := off + int64(n)
	// Writing off the end of the file so need to make some
	// zeroes.  we do this by showing that we have written to the
	// new parts of the file.
	if off > item.info.Size {
		item._written(item.info.Size, off-item.info.Size)
		item._dirty()
	}
	// Update size
	if end > item.info.Size {
		item.info.Size = end
	}
	item.mu.Unlock()
	return n, err
}

// WriteAtNoOverwrite writes b to the file, but will not overwrite
// already present ranges.
//
// This is used by the downloader to write bytes to the file.
//
// It returns n the total bytes processed and skipped the number of
// bytes which were processed but not actually written to the file.
func (item *Item) WriteAtNoOverwrite(b []byte, off int64) (n int, skipped int, err error) {
	item.mu.Lock()

	var (
		// Range we wish to write
		r = ranges.Range{Pos: off, Size: int64(len(b))}
		// Ranges that we need to write
		foundRanges = item.info.Rs.FindAll(r)
		// Length of each write
		nn int
	)

	// Write the range out ignoring already written chunks
	// fs.Debugf(item.name, "Ranges = %v", item.info.Rs)
	for i := range foundRanges {
		foundRange := &foundRanges[i]
		// fs.Debugf(item.name, "foundRange[%d] = %v", i, foundRange)
		if foundRange.R.Pos != off {
			err = errors.New("internal error: offset of range is wrong")
			break
		}
		size := int(foundRange.R.Size)
		if foundRange.Present {
			// if present want to skip this range
			// fs.Debugf(item.name, "skip chunk offset=%d size=%d", off, size)
			nn = size
			skipped += size
		} else {
			// if range not present then we want to write it
			// fs.Debugf(item.name, "write chunk offset=%d size=%d", off, size)
			nn, err = item.fd.WriteAt(b[:size], off)
			if err == nil && nn != size {
				err = fmt.Errorf("downloader: short write: tried to write %d but only %d written", size, nn)
			}
			item._written(off, int64(nn))
		}
		off += int64(nn)
		b = b[nn:]
		n += nn
		if err != nil {
			break
		}
	}
	item.mu.Unlock()
	return n, skipped, err
}

// Sync commits the current contents of the file to stable storage. Typically,
// this means flushing the file system's in-memory copy of recently written
// data to disk.
func (item *Item) Sync() (err error) {
	item.preAccess()
	defer item.postAccess()
	item.mu.Lock()
	defer item.mu.Unlock()
	if item.fd == nil {
		return errors.New("vfs cache item sync: internal error: didn't Open file")
	}
	// sync the file and the metadata to disk
	err = item.fd.Sync()
	if err != nil {
		return fmt.Errorf("vfs cache item sync: failed to sync file: %w", err)
	}
	err = item._save()
	if err != nil {
		return fmt.Errorf("vfs cache item sync: failed to sync metadata: %w", err)
	}
	return nil
}

// rename the item
func (item *Item) rename(name string, newName string, newObj fs.Object) (err error) {
	item.preAccess()
	defer item.postAccess()
	item.mu.Lock()

	// stop downloader
	downloaders := item.downloaders
	item.downloaders = nil

	// id for writeback cancel
	id := item.writeBackID

	// Set internal state
	item.name = newName
	item.o = newObj

	// Rename cache file if it exists
	err = rename(item.c.toOSPath(name), item.c.toOSPath(newName)) // No locking in Cache

	// Rename meta file if it exists
	err2 := rename(item.c.toOSPathMeta(name), item.c.toOSPathMeta(newName)) // No locking in Cache
	if err2 != nil {
		err = err2
	}

	item.mu.Unlock()

	// close downloader and cancel writebacks with mutex unlocked
	if downloaders != nil {
		_ = downloaders.Close(nil)
	}
	item.c.writeback.Rename(id, newName)
	return err
}
