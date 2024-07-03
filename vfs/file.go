package vfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// The File object is tightly coupled to the Dir object. Since they
// both have locks there is plenty of potential for deadlocks. In
// order to mitigate this, we use the following conventions
//
// File may **only** call these methods from Dir with the File lock
// held.
//
//     Dir.Fs
//     Dir.VFS
//
// (As these are read only and do not need to take the Dir mutex.)
//
// File may **not** call any other Dir methods with the File lock
// held. This preserves total lock ordering and makes File subordinate
// to Dir as far as locking is concerned, preventing deadlocks.
//
// File may **not** read any members of Dir directly.

// File represents a file
type File struct {
	inode uint64       // inode number - read only
	size  atomic.Int64 // size of file

	muRW sync.Mutex // synchronize RWFileHandle.openPending(), RWFileHandle.close() and File.Remove

	mu               sync.RWMutex                    // protects the following
	d                *Dir                            // parent directory
	dPath            string                          // path of parent directory. NB dir rename means all Files are flushed
	o                fs.Object                       // NB o may be nil if file is being written
	leaf             string                          // leaf name of the object
	writers          []Handle                        // writers for this file
	virtualModTime   *time.Time                      // modtime for backends with Precision == fs.ModTimeNotSupported
	pendingModTime   time.Time                       // will be applied once o becomes available, i.e. after file was written
	pendingRenameFun func(ctx context.Context) error // will be run/renamed after all writers close
	sys              atomic.Value                    // user defined info to be attached here
	nwriters         atomic.Int32                    // len(writers)
	appendMode       bool                            // file was opened with O_APPEND
}

// newFile creates a new File
//
// o may be nil
func newFile(d *Dir, dPath string, o fs.Object, leaf string) *File {
	f := &File{
		d:     d,
		dPath: dPath,
		o:     o,
		leaf:  leaf,
		inode: newInode(),
	}
	if o != nil {
		f.size.Store(o.Size())
	}
	return f
}

// String converts it to printable
func (f *File) String() string {
	if f == nil {
		return "<nil *File>"
	}
	return f.Path()
}

// IsFile returns true for File - satisfies Node interface
func (f *File) IsFile() bool {
	return true
}

// IsDir returns false for File - satisfies Node interface
func (f *File) IsDir() bool {
	return false
}

// Mode bits of the file or directory - satisfies Node interface
func (f *File) Mode() (mode os.FileMode) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	mode = os.FileMode(f.d.vfs.Opt.FilePerms)
	if f.appendMode {
		mode |= os.ModeAppend
	}
	return mode
}

// Name (base) of the directory - satisfies Node interface
func (f *File) Name() (name string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.leaf
}

// _path returns the full path of the file
// use when lock is held
func (f *File) _path() string {
	return path.Join(f.dPath, f.leaf)
}

// Path returns the full path of the file
func (f *File) Path() string {
	f.mu.RLock()
	dPath, leaf := f.dPath, f.leaf
	f.mu.RUnlock()
	return path.Join(dPath, leaf)
}

// Sys returns underlying data source (can be nil) - satisfies Node interface
func (f *File) Sys() interface{} {
	return f.sys.Load()
}

// SetSys sets the underlying data source (can be nil) - satisfies Node interface
func (f *File) SetSys(x interface{}) {
	f.sys.Store(x)
}

// Inode returns the inode number - satisfies Node interface
func (f *File) Inode() uint64 {
	return f.inode
}

// Node returns the Node associated with this - satisfies Noder interface
func (f *File) Node() Node {
	return f
}

// renameDir - call when parent directory has been renamed
func (f *File) renameDir(dPath string) {
	f.mu.RLock()
	f.dPath = dPath
	f.mu.RUnlock()
}

// applyPendingRename runs a previously set rename operation if there are no
// more remaining writers. Call without lock held.
func (f *File) applyPendingRename() {
	f.mu.RLock()
	fun := f.pendingRenameFun
	writing := f._writingInProgress()
	f.mu.RUnlock()
	if fun == nil || writing {
		return
	}
	fs.Debugf(f.Path(), "Running delayed rename now")
	if err := fun(context.TODO()); err != nil {
		fs.Errorf(f.Path(), "delayed File.Rename error: %v", err)
	}
}

// rename attempts to immediately rename a file if there are no open writers.
// Otherwise it will queue the rename operation on the remote until no writers
// remain.
func (f *File) rename(ctx context.Context, destDir *Dir, newName string) error {
	f.mu.RLock()
	d := f.d
	oldPendingRenameFun := f.pendingRenameFun
	f.mu.RUnlock()

	if features := d.Fs().Features(); features.Move == nil && features.Copy == nil {
		err := fmt.Errorf("Fs %q can't rename files (no server-side Move or Copy)", d.Fs())
		fs.Errorf(f.Path(), "Dir.Rename error: %v", err)
		return err
	}

	oldPath := f.Path()
	// File.mu is unlocked here to call Dir.Path()
	newPath := path.Join(destDir.Path(), newName)

	renameCall := func(ctx context.Context) (err error) {
		// chain rename calls if any
		if oldPendingRenameFun != nil {
			err := oldPendingRenameFun(ctx)
			if err != nil {
				return err
			}
		}

		f.mu.RLock()
		o := f.o
		d := f.d
		f.mu.RUnlock()
		var newObject fs.Object
		// if o is nil then are writing the file so no need to rename the object
		if o != nil {
			if o.Remote() == newPath {
				return nil // no need to rename
			}

			// do the move of the remote object
			dstOverwritten, _ := d.Fs().NewObject(ctx, newPath)
			newObject, err = operations.Move(ctx, d.Fs(), dstOverwritten, newPath, o)
			if err != nil {
				fs.Errorf(f.Path(), "File.Rename error: %v", err)
				return err
			}

			// newObject can be nil here for example if --dry-run
			if newObject == nil {
				err = errors.New("rename failed: nil object returned")
				fs.Errorf(f.Path(), "File.Rename %v", err)
				return err
			}
		}
		// Rename in the cache
		if d.vfs.cache != nil && d.vfs.cache.Exists(oldPath) {
			if err := d.vfs.cache.Rename(oldPath, newPath, newObject); err != nil {
				fs.Infof(f.Path(), "File.Rename failed in Cache: %v", err)
			}
		}
		// Update the node with the new details
		fs.Debugf(f.Path(), "Updating file with %v %p", newObject, f)
		// f.rename(destDir, newObject)
		f.mu.Lock()
		if newObject != nil {
			f.o = newObject
		}
		f.pendingRenameFun = nil
		f.mu.Unlock()
		return nil
	}

	// rename the file object
	dPath := destDir.Path()
	f.mu.Lock()
	f.d = destDir
	f.dPath = dPath
	f.leaf = newName
	writing := f._writingInProgress()
	f.mu.Unlock()

	// Delay the rename if not using RW caching. For the minimal case we
	// need to look in the cache to see if caching is in use.
	CacheMode := d.vfs.Opt.CacheMode
	if writing &&
		(CacheMode < vfscommon.CacheModeMinimal ||
			(CacheMode == vfscommon.CacheModeMinimal && !destDir.vfs.cache.Exists(oldPath))) {
		fs.Debugf(oldPath, "File is currently open, delaying rename %p", f)
		f.mu.Lock()
		f.pendingRenameFun = renameCall
		f.mu.Unlock()
		return nil
	}

	return renameCall(ctx)
}

// addWriter adds a write handle to the file
func (f *File) addWriter(h Handle) {
	f.mu.Lock()
	f.writers = append(f.writers, h)
	f.nwriters.Add(1)
	f.mu.Unlock()
}

// delWriter removes a write handle from the file
func (f *File) delWriter(h Handle) {
	f.mu.Lock()
	defer f.applyPendingRename()
	defer f.mu.Unlock()
	var found = -1
	for i := range f.writers {
		if f.writers[i] == h {
			found = i
			break
		}
	}
	if found >= 0 {
		f.writers = append(f.writers[:found], f.writers[found+1:]...)
		f.nwriters.Add(-1)
	} else {
		fs.Debugf(f._path(), "File.delWriter couldn't find handle")
	}
}

// activeWriters returns the number of writers on the file
//
// Note that we don't take the mutex here.  If we do then we can get a
// deadlock.
func (f *File) activeWriters() int {
	return int(f.nwriters.Load())
}

// _roundModTime rounds the time passed in to the Precision of the
// underlying Fs
//
// It should be called with the lock held
func (f *File) _roundModTime(modTime time.Time) time.Time {
	precision := f.d.f.Precision()
	if precision == fs.ModTimeNotSupported {
		return modTime
	}
	return modTime.Truncate(precision)
}

// ModTime returns the modified time of the file
//
// if NoModTime is set then it returns the mod time of the directory
func (f *File) ModTime() (modTime time.Time) {
	f.mu.RLock()
	d, o, pendingModTime, virtualModTime := f.d, f.o, f.pendingModTime, f.virtualModTime
	f.mu.RUnlock()

	// Set the virtual modtime up for backends which don't support setting modtime
	//
	// Note that we only cache modtime values that we have returned to the OS
	// if we haven't returned a value to the OS then we can change it
	defer func() {
		if f.d.f.Precision() == fs.ModTimeNotSupported && (virtualModTime == nil || !virtualModTime.Equal(modTime)) {
			f.virtualModTime = &modTime
			fs.Debugf(f._path(), "Set virtual modtime to %v", f.virtualModTime)
		}
	}()

	if d.vfs.Opt.NoModTime {
		return d.ModTime()
	}
	// Read the modtime from a dirty item if it exists
	if f.d.vfs.Opt.CacheMode >= vfscommon.CacheModeMinimal {
		if item := f.d.vfs.cache.DirtyItem(f._path()); item != nil {
			modTime, err := item.GetModTime()
			if err != nil {
				fs.Errorf(f._path(), "ModTime: Item GetModTime failed: %v", err)
			} else {
				return f._roundModTime(modTime)
			}
		}
	}
	if !pendingModTime.IsZero() {
		return f._roundModTime(pendingModTime)
	}
	if virtualModTime != nil && !virtualModTime.IsZero() {
		fs.Debugf(f._path(), "Returning virtual modtime %v", f.virtualModTime)
		return f._roundModTime(*virtualModTime)
	}
	if o == nil {
		return time.Now()
	}
	return o.ModTime(context.TODO())
}

// nonNegative returns 0 if i is -ve, i otherwise
func nonNegative(i int64) int64 {
	if i >= 0 {
		return i
	}
	return 0
}

// Size of the file
func (f *File) Size() int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Read the size from a dirty item if it exists
	if f.d.vfs.Opt.CacheMode >= vfscommon.CacheModeMinimal {
		if item := f.d.vfs.cache.DirtyItem(f._path()); item != nil {
			size, err := item.GetSize()
			if err != nil {
				fs.Errorf(f._path(), "Size: Item GetSize failed: %v", err)
			} else {
				return size
			}
		}
	}

	// if o is nil it isn't valid yet or there are writers, so return the size so far
	if f._writingInProgress() {
		return f.size.Load()
	}
	return nonNegative(f.o.Size())
}

// SetModTime sets the modtime for the file
//
// if NoModTime is set then it does nothing
func (f *File) SetModTime(modTime time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.d.vfs.Opt.NoModTime {
		return nil
	}
	if f.d.vfs.Opt.ReadOnly {
		return EROFS
	}

	f.pendingModTime = modTime

	// set the time of the file in the cache
	if f.d.vfs.cache != nil && f.d.vfs.cache.Exists(f._path()) {
		f.d.vfs.cache.SetModTime(f._path(), f.pendingModTime)
	}

	// Only update the ModTime when there are no writers, setObject will do it
	if !f._writingInProgress() {
		return f._applyPendingModTime()
	}

	// queue up for later, hoping f.o becomes available
	return nil
}

// Apply a pending mod time
// Call with the mutex held
func (f *File) _applyPendingModTime() error {
	if f.pendingModTime.IsZero() {
		return nil
	}
	defer func() { f.pendingModTime = time.Time{} }()

	if f.o == nil {
		return errors.New("cannot apply ModTime, file object is not available")
	}

	dt := f.pendingModTime.Sub(f.o.ModTime(context.Background()))
	modifyWindow := f.o.Fs().Precision()
	if dt < modifyWindow && dt > -modifyWindow {
		fs.Debugf(f.o, "Not setting pending mod time %v as it is already set", f.pendingModTime)
		return nil
	}

	// set the time of the object
	err := f.o.SetModTime(context.TODO(), f.pendingModTime)
	switch err {
	case nil:
		fs.Debugf(f.o, "Applied pending mod time %v OK", f.pendingModTime)
	case fs.ErrorCantSetModTime, fs.ErrorCantSetModTimeWithoutDelete:
		// do nothing, in order to not break "touch somefile" if it exists already
	default:
		fs.Errorf(f.o, "Failed to apply pending mod time %v: %v", f.pendingModTime, err)
		return err
	}

	return nil
}

// Apply a pending mod time
func (f *File) applyPendingModTime() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f._applyPendingModTime()
}

// _writingInProgress returns true of there are any open writers
// Call with read lock held
func (f *File) _writingInProgress() bool {
	return f.o == nil || len(f.writers) != 0
}

// writingInProgress returns true of there are any open writers
func (f *File) writingInProgress() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.o == nil || len(f.writers) != 0
}

// Update the size while writing
func (f *File) setSize(n int64) {
	f.size.Store(n)
}

// Update the object when written and add it to the directory
func (f *File) setObject(o fs.Object) {
	f.mu.Lock()
	f.o = o
	_ = f._applyPendingModTime()
	d := f.d
	f.mu.Unlock()

	// Release File.mu before calling Dir method
	d.addObject(f)
}

// Update the object but don't update the directory cache - for use by
// the directory cache
func (f *File) setObjectNoUpdate(o fs.Object) {
	f.mu.Lock()
	f.o = o
	f.virtualModTime = nil
	fs.Debugf(f._path(), "Reset virtual modtime")
	f.mu.Unlock()
}

// Get the current fs.Object - may be nil
func (f *File) getObject() fs.Object {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.o
}

// exists returns whether the file exists already
func (f *File) exists() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.o != nil
}

// Wait for f.o to become non nil for a short time returning it or an
// error.  Use when opening a read handle.
//
// Call without the mutex held
func (f *File) waitForValidObject() (o fs.Object, err error) {
	for i := 0; i < 50; i++ {
		f.mu.RLock()
		o = f.o
		nwriters := len(f.writers)
		f.mu.RUnlock()
		if o != nil {
			return o, nil
		}
		if nwriters == 0 {
			return nil, errors.New("can't open file - writer failed")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, ENOENT
}

// openRead open the file for read
func (f *File) openRead() (fh *ReadFileHandle, err error) {
	// if o is nil it isn't valid yet
	_, err = f.waitForValidObject()
	if err != nil {
		return nil, err
	}
	// fs.Debugf(f.Path(), "File.openRead")

	fh, err = newReadFileHandle(f)
	if err != nil {
		fs.Debugf(f.Path(), "File.openRead failed: %v", err)
		return nil, err
	}
	return fh, nil
}

// openWrite open the file for write
func (f *File) openWrite(flags int) (fh *WriteFileHandle, err error) {
	f.mu.RLock()
	d := f.d
	f.mu.RUnlock()

	if d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	// fs.Debugf(f.Path(), "File.openWrite")

	fh, err = newWriteFileHandle(d, f, f.Path(), flags)
	if err != nil {
		fs.Debugf(f.Path(), "File.openWrite failed: %v", err)
		return nil, err
	}
	return fh, nil
}

// openRW open the file for read and write using a temporary file
//
// It uses the open flags passed in.
func (f *File) openRW(flags int) (fh *RWFileHandle, err error) {
	f.mu.RLock()
	d := f.d
	f.mu.RUnlock()

	// FIXME chunked
	if flags&accessModeMask != os.O_RDONLY && d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	// fs.Debugf(f.Path(), "File.openRW")

	fh, err = newRWFileHandle(d, f, flags)
	if err != nil {
		fs.Debugf(f.Path(), "File.openRW failed: %v", err)
		return nil, err
	}
	return fh, nil
}

// Sync the file
//
// Note that we don't do anything except return OK
func (f *File) Sync() error {
	return nil
}

// Remove the file
func (f *File) Remove() (err error) {
	defer log.Trace(f.Path(), "")("err=%v", &err)
	f.mu.RLock()
	d := f.d
	f.mu.RUnlock()

	if d.vfs.Opt.ReadOnly {
		return EROFS
	}

	// Remove the object from the cache
	wasWriting := false
	if d.vfs.cache != nil && d.vfs.cache.Exists(f.Path()) {
		wasWriting = d.vfs.cache.Remove(f.Path())
	}

	f.muRW.Lock() // muRW must be locked before mu to avoid
	f.mu.Lock()   // deadlock in RWFileHandle.openPending and .close
	if f.o != nil {
		err = f.o.Remove(context.TODO())
	}
	f.mu.Unlock()
	f.muRW.Unlock()
	if err != nil {
		if wasWriting {
			// Ignore error deleting file if was writing it as it may not be uploaded yet
			err = nil
			fs.Debugf(f._path(), "Ignoring File.Remove file error as uploading: %v", err)
		} else {
			fs.Debugf(f._path(), "File.Remove file error: %v", err)
		}
	}

	// Remove the item from the directory listing
	// called with File.mu released when there is no error removing the underlying file
	if err == nil {
		d.delObject(f.Name())
	}
	return err
}

// RemoveAll the file - same as remove for files
func (f *File) RemoveAll() error {
	return f.Remove()
}

// DirEntry returns the underlying fs.DirEntry - may be nil
func (f *File) DirEntry() (entry fs.DirEntry) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.o
}

// Dir returns the directory this file is in
func (f *File) Dir() *Dir {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.d
}

// VFS returns the instance of the VFS
func (f *File) VFS() *VFS {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.d.vfs
}

// Fs returns the underlying Fs for the file
func (f *File) Fs() fs.Fs {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.d.Fs()
}

// Open a file according to the flags provided
//
//	O_RDONLY open the file read-only.
//	O_WRONLY open the file write-only.
//	O_RDWR   open the file read-write.
//
//	O_APPEND append data to the file when writing.
//	O_CREATE create a new file if none exists.
//	O_EXCL   used with O_CREATE, file must not exist
//	O_SYNC   open for synchronous I/O.
//	O_TRUNC  if possible, truncate file when opened
//
// We ignore O_SYNC and O_EXCL
func (f *File) Open(flags int) (fd Handle, err error) {
	defer log.Trace(f.Path(), "flags=%s", decodeOpenFlags(flags))("fd=%v, err=%v", &fd, &err)
	var (
		write    bool // if set need write support
		read     bool // if set need read support
		rdwrMode = flags & accessModeMask
	)

	// http://pubs.opengroup.org/onlinepubs/7908799/xsh/open.html
	// The result of using O_TRUNC with O_RDONLY is undefined.
	// Linux seems to truncate the file, but we prefer to return EINVAL
	if rdwrMode == os.O_RDONLY && flags&os.O_TRUNC != 0 {
		return nil, EINVAL
	}

	// Figure out the read/write intents
	switch {
	case rdwrMode == os.O_RDONLY:
		read = true
	case rdwrMode == os.O_WRONLY:
		write = true
	case rdwrMode == os.O_RDWR:
		read = true
		write = true
	default:
		fs.Debugf(f.Path(), "Can't figure out how to open with flags: 0x%X", flags)
		return nil, EPERM
	}

	// If append is set then set read to force openRW
	if flags&os.O_APPEND != 0 {
		read = true
		f.mu.Lock()
		f.appendMode = true
		f.mu.Unlock()
	}

	// If truncate is set then set write to force openRW
	if flags&os.O_TRUNC != 0 {
		write = true
	}

	// If create is set then set write to force openRW
	if flags&os.O_CREATE != 0 {
		write = true
	}

	// Open the correct sort of handle
	f.mu.RLock()
	d := f.d
	f.mu.RUnlock()
	CacheMode := d.vfs.Opt.CacheMode
	if CacheMode >= vfscommon.CacheModeMinimal && (d.vfs.cache.InUse(f.Path()) || d.vfs.cache.Exists(f.Path())) {
		fd, err = f.openRW(flags)
	} else if read && write {
		if CacheMode >= vfscommon.CacheModeMinimal {
			fd, err = f.openRW(flags)
		} else {
			// Open write only and hope the user doesn't
			// want to read.  If they do they will get an
			// EPERM plus an Error log.
			fd, err = f.openWrite(flags)
		}
	} else if write {
		if CacheMode >= vfscommon.CacheModeWrites {
			fd, err = f.openRW(flags)
		} else {
			fd, err = f.openWrite(flags)
		}
	} else if read {
		if CacheMode >= vfscommon.CacheModeFull {
			fd, err = f.openRW(flags)
		} else {
			fd, err = f.openRead()
		}
	} else {
		fs.Debugf(f.Path(), "Can't figure out how to open with flags: 0x%X", flags)
		return nil, EPERM
	}
	// if creating a file, add the file to the directory
	if err == nil && flags&os.O_CREATE != 0 {
		// called without File.mu held
		d.addObject(f)
	}
	return fd, err
}

// Truncate changes the size of the named file.
func (f *File) Truncate(size int64) (err error) {
	// make a copy of fh.writers with the lock held then unlock so
	// we can call other file methods.
	f.mu.Lock()
	writers := make([]Handle, len(f.writers))
	copy(writers, f.writers)
	f.mu.Unlock()

	// If have writers then call truncate for each writer
	if len(writers) != 0 {
		var openWriters = len(writers)
		fs.Debugf(f.Path(), "Truncating %d file handles", len(writers))
		for _, h := range writers {
			truncateErr := h.Truncate(size)
			if truncateErr == ECLOSED {
				// Ignore ECLOSED since file handle can get closed while this is running
				openWriters--
			} else if truncateErr != nil {
				err = truncateErr
			}
		}
		// If at least one open writer return here
		if openWriters > 0 {
			return err
		}
	}

	// if o is nil it isn't valid yet
	o, err := f.waitForValidObject()
	if err != nil {
		return err
	}

	// If no writers, and size is already correct then all done
	if o.Size() == size {
		return nil
	}

	fs.Debugf(f.Path(), "Truncating file")

	// Otherwise if no writers then truncate the file by opening
	// the file and truncating it.
	flags := os.O_WRONLY
	if size == 0 {
		flags |= os.O_TRUNC
	}
	fh, err := f.Open(flags)
	if err != nil {
		return err
	}
	defer fs.CheckClose(fh, &err)
	if size != 0 {
		return fh.Truncate(size)
	}
	return nil
}
