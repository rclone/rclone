package vfs

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/vfs/vfscache"
)

// RWFileHandle is a handle that can be open for read and write.
//
// It will be open to a temporary file which, when closed, will be
// transferred to the remote.
type RWFileHandle struct {
	// read only variables
	file  *File
	d     *Dir
	flags int            // open flags
	item  *vfscache.Item // cached file item

	// read write variables protected by mutex
	mu          sync.Mutex
	offset      int64 // file pointer offset
	closed      bool  // set if handle has been closed
	opened      bool
	writeCalled bool // if any Write() methods have been called
}

// Lock performs Unix locking, not supported
func (fh *RWFileHandle) Lock() error {
	return os.ErrInvalid
}

// Unlock performs Unix unlocking, not supported
func (fh *RWFileHandle) Unlock() error {
	return os.ErrInvalid
}

func newRWFileHandle(d *Dir, f *File, flags int) (fh *RWFileHandle, err error) {
	defer log.Trace(f.Path(), "")("err=%v", &err)
	// get an item to represent this from the cache
	item := d.vfs.cache.Item(f.Path())

	exists := f.exists() || (item.Exists() && !item.WrittenBack())

	// if O_CREATE and O_EXCL are set and if path already exists, then return EEXIST
	if flags&(os.O_CREATE|os.O_EXCL) == os.O_CREATE|os.O_EXCL && exists {
		return nil, EEXIST
	}

	fh = &RWFileHandle{
		file:  f,
		d:     d,
		flags: flags,
		item:  item,
	}

	// truncate immediately if O_TRUNC is set or O_CREATE is set and file doesn't exist
	if !fh.readOnly() && (fh.flags&os.O_TRUNC != 0 || (fh.flags&os.O_CREATE != 0 && !exists)) {
		err = fh.Truncate(0)
		if err != nil {
			return nil, fmt.Errorf("cache open with O_TRUNC: failed to truncate: %w", err)
		}
		// we definitely need to write back the item even if we don't write to it
		item.Dirty()
	}

	if !fh.readOnly() {
		fh.file.addWriter(fh)
	}

	return fh, nil
}

// readOnly returns whether flags say fh is read only
func (fh *RWFileHandle) readOnly() bool {
	return (fh.flags & accessModeMask) == os.O_RDONLY
}

// writeOnly returns whether flags say fh is write only
func (fh *RWFileHandle) writeOnly() bool {
	return (fh.flags & accessModeMask) == os.O_WRONLY
}

// openPending opens the file if there is a pending open
//
// call with the lock held
func (fh *RWFileHandle) openPending() (err error) {
	if fh.opened {
		return nil
	}
	defer log.Trace(fh.logPrefix(), "")("err=%v", &err)

	fh.file.muRW.Lock()
	defer fh.file.muRW.Unlock()

	o := fh.file.getObject()
	err = fh.item.Open(o)
	if err != nil {
		return fmt.Errorf("open RW handle failed to open cache file: %w", err)
	}

	size := fh._size() // update size in file and read size
	if fh.flags&os.O_APPEND != 0 {
		fh.offset = size
		fs.Debugf(fh.logPrefix(), "open at offset %d", fh.offset)
	} else {
		fh.offset = 0
	}
	fh.opened = true
	fh.d.addObject(fh.file) // make sure the directory has this object in it now
	return nil
}

// String converts it to printable
func (fh *RWFileHandle) String() string {
	if fh == nil {
		return "<nil *RWFileHandle>"
	}
	if fh.file == nil {
		return "<nil *RWFileHandle.file>"
	}
	return fh.file.String() + " (rw)"
}

// Node returns the Node associated with this - satisfies Noder interface
func (fh *RWFileHandle) Node() Node {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file
}

// updateSize updates the size of the file if necessary
//
// Must be called with fh.mu held
func (fh *RWFileHandle) updateSize() {
	// If read only or not opened then ignore
	if fh.readOnly() || !fh.opened {
		return
	}
	size := fh._size()
	fh.file.setSize(size)
}

// close the file handle returning EBADF if it has been
// closed already.
//
// Must be called with fh.mu held.
//
// Note that we leave the file around in the cache on error conditions
// to give the user a chance to recover it.
func (fh *RWFileHandle) close() (err error) {
	defer log.Trace(fh.logPrefix(), "")("err=%v", &err)
	fh.file.muRW.Lock()
	defer fh.file.muRW.Unlock()

	if fh.closed {
		return ECLOSED
	}

	fh.closed = true
	fh.updateSize()
	if fh.opened {
		err = fh.item.Close(fh.file.setObject)
		fh.opened = false
	} else {
		// apply any pending mod times if any
		_ = fh.file.applyPendingModTime()
	}

	if !fh.readOnly() {
		fh.file.delWriter(fh)
	}

	return err
}

// Close closes the file
func (fh *RWFileHandle) Close() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.close()
}

// Flush is called each time the file or directory is closed.
// Because there can be multiple file descriptors referring to a
// single opened file, Flush can be called multiple times.
func (fh *RWFileHandle) Flush() error {
	fh.mu.Lock()
	fs.Debugf(fh.logPrefix(), "RWFileHandle.Flush")
	fh.updateSize()
	fh.mu.Unlock()
	return nil
}

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *RWFileHandle) Release() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	fs.Debugf(fh.logPrefix(), "RWFileHandle.Release")
	if fh.closed {
		// Don't return an error if called twice
		return nil
	}
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.logPrefix(), "RWFileHandle.Release error: %v", err)
	}
	return err
}

// _size returns the size of the underlying file and also sets it in
// the owning file
//
// call with the lock held
func (fh *RWFileHandle) _size() int64 {
	size, err := fh.item.GetSize()
	if err != nil {
		o := fh.file.getObject()
		if o != nil {
			size = o.Size()
		} else {
			fs.Errorf(fh.logPrefix(), "Couldn't read size of file")
			size = 0
		}
	}
	fh.file.setSize(size)
	return size
}

// Size returns the size of the underlying file
func (fh *RWFileHandle) Size() int64 {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh._size()
}

// Stat returns info about the file
func (fh *RWFileHandle) Stat() (os.FileInfo, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file, nil
}

// _readAt bytes from the file at off
//
// if release is set then it releases the mutex just before doing the IO
//
// call with lock held
func (fh *RWFileHandle) _readAt(b []byte, off int64, release bool) (n int, err error) {
	defer log.Trace(fh.logPrefix(), "size=%d, off=%d", len(b), off)("n=%d, err=%v", &n, &err)
	if fh.closed {
		return n, ECLOSED
	}
	if fh.writeOnly() {
		return n, EBADF
	}
	if off >= fh._size() {
		return n, io.EOF
	}
	if err = fh.openPending(); err != nil {
		return n, err
	}
	if release {
		// Do the writing with fh.mu unlocked
		fh.mu.Unlock()
	}

	n, err = fh.item.ReadAt(b, off)

	if release {
		fh.mu.Lock()
	}
	return n, err
}

// ReadAt bytes from the file at off
func (fh *RWFileHandle) ReadAt(b []byte, off int64) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh._readAt(b, off, true)
}

// Read bytes from the file
func (fh *RWFileHandle) Read(b []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	n, err = fh._readAt(b, fh.offset, false)
	fh.offset += int64(n)
	return n, err
}

// Seek to new file position
func (fh *RWFileHandle) Seek(offset int64, whence int) (ret int64, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return 0, ECLOSED
	}
	if !fh.opened && offset == 0 && whence != 2 {
		return 0, nil
	}
	if err = fh.openPending(); err != nil {
		return ret, err
	}
	switch whence {
	case io.SeekStart:
		fh.offset = 0
	case io.SeekEnd:
		fh.offset = fh._size()
	}
	fh.offset += offset
	// we don't check the offset - the next Read will
	return fh.offset, nil
}

// _writeAt bytes to the file at off
//
// if release is set then it releases the mutex just before doing the IO
//
// call with lock held
func (fh *RWFileHandle) _writeAt(b []byte, off int64, release bool) (n int, err error) {
	defer log.Trace(fh.logPrefix(), "size=%d, off=%d", len(b), off)("n=%d, err=%v", &n, &err)
	if fh.closed {
		return n, ECLOSED
	}
	if fh.readOnly() {
		return n, EBADF
	}
	if err = fh.openPending(); err != nil {
		return n, err
	}
	if fh.flags&os.O_APPEND != 0 {
		// From open(2): Before each write(2), the file offset is
		// positioned at the end of the file, as if with lseek(2).
		size := fh._size()
		fh.offset = size
		off = fh.offset
	}
	fh.writeCalled = true
	if release {
		// Do the writing with fh.mu unlocked
		fh.mu.Unlock()
	}
	n, err = fh.item.WriteAt(b, off)
	if release {
		fh.mu.Lock()
	}
	if err != nil {
		return n, err
	}

	_ = fh._size()
	return n, err
}

// WriteAt bytes to the file at off
func (fh *RWFileHandle) WriteAt(b []byte, off int64) (n int, err error) {
	fh.mu.Lock()
	n, err = fh._writeAt(b, off, true)
	if fh.flags&os.O_APPEND != 0 {
		fh.offset += int64(n)
	}
	fh.mu.Unlock()
	return n, err
}

// Write bytes to the file
func (fh *RWFileHandle) Write(b []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	n, err = fh._writeAt(b, fh.offset, false)
	fh.offset += int64(n)
	return n, err
}

// WriteString a string to the file
func (fh *RWFileHandle) WriteString(s string) (n int, err error) {
	return fh.Write([]byte(s))
}

// Truncate file to given size
//
// Call with mutex held
func (fh *RWFileHandle) _truncate(size int64) (err error) {
	if size == fh._size() {
		return nil
	}
	fh.file.setSize(size)
	return fh.item.Truncate(size)
}

// Truncate file to given size
func (fh *RWFileHandle) Truncate(size int64) (err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return ECLOSED
	}
	if err = fh.openPending(); err != nil {
		return err
	}
	return fh._truncate(size)
}

// Sync commits the current contents of the file to stable storage. Typically,
// this means flushing the file system's in-memory copy of recently written
// data to disk.
func (fh *RWFileHandle) Sync() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return ECLOSED
	}
	if !fh.opened {
		return nil
	}
	if fh.readOnly() {
		return nil
	}
	return fh.item.Sync()
}

func (fh *RWFileHandle) logPrefix() string {
	return fmt.Sprintf("%s(%p)", fh.file.Path(), fh)
}

// Chdir changes the current working directory to the file, which must
// be a directory.
func (fh *RWFileHandle) Chdir() error {
	return ENOSYS
}

// Chmod changes the mode of the file to mode.
func (fh *RWFileHandle) Chmod(mode os.FileMode) error {
	return ENOSYS
}

// Chown changes the numeric uid and gid of the named file.
func (fh *RWFileHandle) Chown(uid, gid int) error {
	return ENOSYS
}

// Fd returns the integer Unix file descriptor referencing the open file.
func (fh *RWFileHandle) Fd() uintptr {
	return 0xdeadbeef // FIXME
}

// Name returns the name of the file from the underlying Object.
func (fh *RWFileHandle) Name() string {
	return fh.file.String()
}

// Readdir reads the contents of the directory associated with file.
func (fh *RWFileHandle) Readdir(n int) ([]os.FileInfo, error) {
	return nil, ENOSYS
}

// Readdirnames reads the contents of the directory associated with file.
func (fh *RWFileHandle) Readdirnames(n int) (names []string, err error) {
	return nil, ENOSYS
}
