package vfs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/lib/file"
)

// RWFileHandle is a handle that can be open for read and write.
//
// It will be open to a temporary file which, when closed, will be
// transferred to the remote.
type RWFileHandle struct {
	*os.File
	mu          sync.Mutex
	closed      bool // set if handle has been closed
	file        *File
	d           *Dir
	opened      bool
	flags       int  // open flags
	writeCalled bool // if any Write() methods have been called
	changed     bool // file contents was changed in any other way
}

// Check interfaces
var (
	_ io.Reader   = (*RWFileHandle)(nil)
	_ io.ReaderAt = (*RWFileHandle)(nil)
	_ io.Writer   = (*RWFileHandle)(nil)
	_ io.WriterAt = (*RWFileHandle)(nil)
	_ io.Seeker   = (*RWFileHandle)(nil)
	_ io.Closer   = (*RWFileHandle)(nil)
)

func newRWFileHandle(d *Dir, f *File, flags int) (fh *RWFileHandle, err error) {
	// if O_CREATE and O_EXCL are set and if path already exists, then return EEXIST
	if flags&(os.O_CREATE|os.O_EXCL) == os.O_CREATE|os.O_EXCL && f.exists() {
		return nil, EEXIST
	}

	fh = &RWFileHandle{
		file:  f,
		d:     d,
		flags: flags,
	}

	// mark the file as open in the cache - must be done before the mkdir
	fh.d.VFS().cache.Open(fh.file.Path())

	// Make a place for the file
	_, err = d.VFS().cache.Mkdir(fh.file.Path())
	if err != nil {
		fh.d.VFS().cache.Close(fh.file.Path())
		return nil, errors.Wrap(err, "open RW handle failed to make cache directory")
	}

	rdwrMode := fh.flags & accessModeMask
	if rdwrMode != os.O_RDONLY {
		fh.file.addWriter(fh)
	}

	// truncate or create files immediately to prepare the cache
	if fh.flags&os.O_TRUNC != 0 || fh.flags&(os.O_CREATE) != 0 && !f.exists() {
		if err := fh.openPending(false); err != nil {
			fh.file.delWriter(fh, false)
			return nil, err
		}
	}

	return fh, nil
}

// openPending opens the file if there is a pending open
//
// call with the lock held
func (fh *RWFileHandle) openPending(truncate bool) (err error) {
	if fh.opened {
		return nil
	}

	fh.file.muRW.Lock()
	defer fh.file.muRW.Unlock()

	o := fh.file.getObject()

	var fd *os.File
	cacheFileOpenFlags := fh.flags
	// if not truncating the file, need to read it first
	if fh.flags&os.O_TRUNC == 0 && !truncate {
		// If the remote object exists AND its cached file exists locally AND there are no
		// other RW handles with it open, then attempt to update it.
		if o != nil && fh.file.rwOpens() == 0 {
			err = fh.d.VFS().cache.Check(context.TODO(), o, fh.file.Path())
			if err != nil {
				return errors.Wrap(err, "open RW handle failed to check cache file")
			}
		}

		// try to open an existing cache file
		fd, err = file.OpenFile(fh.file.osPath(), cacheFileOpenFlags&^os.O_CREATE, 0600)
		if os.IsNotExist(err) {
			// cache file does not exist, so need to fetch it if we have an object to fetch
			// it from
			if o != nil {
				err = fh.d.VFS().cache.Fetch(context.TODO(), o, fh.file.Path())
				if err != nil {
					cause := errors.Cause(err)
					if cause != fs.ErrorObjectNotFound && cause != fs.ErrorDirNotFound {
						// return any non NotFound errors
						return errors.Wrap(err, "open RW handle failed to cache file")
					}
					// continue here with err=fs.Error{Object,Dir}NotFound
				}
			}
			// if err == nil, then we have cached the file successfully, otherwise err is
			// indicating some kind of non existent file/directory either
			// os.IsNotExist(err) or fs.Error{Object,Dir}NotFound
			if err != nil {
				if fh.flags&os.O_CREATE != 0 {
					// if the object wasn't found AND O_CREATE is set then
					// ignore error as we are about to create the file
					fh.file.setSize(0)
					fh.changed = true
				} else {
					return errors.Wrap(err, "open RW handle failed to cache file")
				}
			}
		} else if err != nil {
			return errors.Wrap(err, "cache open file failed")
		} else {
			fs.Debugf(fh.logPrefix(), "Opened existing cached copy with flags=%s", decodeOpenFlags(fh.flags))
		}
	} else {
		// Set the size to 0 since we are truncating and flag we need to write it back
		fh.file.setSize(0)
		fh.changed = true
		if fh.flags&os.O_CREATE == 0 && fh.file.exists() {
			// create an empty file if it exists on the source
			err = ioutil.WriteFile(fh.file.osPath(), []byte{}, 0600)
			if err != nil {
				return errors.Wrap(err, "cache open failed to create zero length file")
			}
		}
		// Windows doesn't seem to deal well with O_TRUNC and
		// certain access modes so truncate the file if it
		// exists in these cases.
		if runtime.GOOS == "windows" && fh.flags&os.O_APPEND != 0 {
			cacheFileOpenFlags &^= os.O_TRUNC
			_, err = os.Stat(fh.file.osPath())
			if err == nil {
				err = os.Truncate(fh.file.osPath(), 0)
				if err != nil {
					return errors.Wrap(err, "cache open failed to truncate")
				}
			}
		}
	}

	if fd == nil {
		fs.Debugf(fh.logPrefix(), "Opening cached copy with flags=%s", decodeOpenFlags(fh.flags))
		fd, err = file.OpenFile(fh.file.osPath(), cacheFileOpenFlags, 0600)
		if err != nil {
			return errors.Wrap(err, "cache open file failed")
		}
	}
	fh.File = fd
	fh.opened = true
	fh.file.addRWOpen()
	fh.d.addObject(fh.file) // make sure the directory has this object in it now
	return nil
}

// String converts it to printable
func (fh *RWFileHandle) String() string {
	if fh == nil {
		return "<nil *RWFileHandle>"
	}
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.file == nil {
		return "<nil *RWFileHandle.file>"
	}
	return fh.file.String() + " (rw)"
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (fh *RWFileHandle) Node() Node {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file
}

// Returns whether the file needs to be written back.
//
// If write hasn't been called and the file hasn't been changed in any other
// way we haven't modified it so we don't need to transfer it
//
// Must be called with fh.mu held
func (fh *RWFileHandle) modified() bool {
	if !fh.writeCalled && !fh.changed {
		fs.Debugf(fh.logPrefix(), "not modified so not transferring")
		return false
	}
	return true
}

// flushWrites flushes any pending writes to cloud storage
//
// Must be called with fh.muRW held
func (fh *RWFileHandle) flushWrites(closeFile bool) error {
	if fh.closed && !closeFile {
		return nil
	}

	rdwrMode := fh.flags & accessModeMask
	writer := rdwrMode != os.O_RDONLY

	// If read only then return
	if !fh.opened && rdwrMode == os.O_RDONLY {
		return nil
	}

	isCopied := false
	if writer {
		isCopied = fh.file.delWriter(fh, fh.modified())
		defer fh.file.finishWriterClose()
	}

	// If we aren't creating or truncating the file then
	// we haven't modified it so don't need to transfer it
	if fh.flags&(os.O_CREATE|os.O_TRUNC) != 0 {
		if err := fh.openPending(false); err != nil {
			return err
		}
	}

	if writer && fh.opened {
		fi, err := fh.File.Stat()
		if err != nil {
			fs.Errorf(fh.logPrefix(), "Failed to stat cache file: %v", err)
		} else {
			fh.file.setSize(fi.Size())
		}
	}

	// Close the underlying file
	if fh.opened && closeFile {
		err := fh.File.Close()
		if err != nil {
			err = errors.Wrap(err, "failed to close cache file")
			return err
		}
	}

	if isCopied {
		o, err := fh.d.VFS().cache.Store(context.TODO(), fh.file.getObject(), fh.file.Path())
		if err != nil {
			fs.Errorf(fh.logPrefix(), "%v", err)
			return err
		}
		fh.file.setObject(o)
		fs.Debugf(o, "transferred to remote")
	}

	return nil
}

// close the file handle returning EBADF if it has been
// closed already.
//
// Must be called with fh.mu held
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
	defer func() {
		if fh.opened {
			fh.file.delRWOpen()
		}
		fh.d.VFS().cache.Close(fh.file.Path())
	}()

	return fh.flushWrites(true)
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
	defer fh.mu.Unlock()
	if !fh.opened {
		return nil
	}
	if fh.closed {
		fs.Debugf(fh.logPrefix(), "RWFileHandle.Flush nothing to do")
		return nil
	}
	// fs.Debugf(fh.logPrefix(), "RWFileHandle.Flush")
	if !fh.opened {
		fs.Debugf(fh.logPrefix(), "RWFileHandle.Flush ignoring flush on unopened handle")
		return nil
	}

	// If Write hasn't been called then ignore the Flush - Release
	// will pick it up
	if !fh.writeCalled {
		fs.Debugf(fh.logPrefix(), "RWFileHandle.Flush ignoring flush on unwritten handle")
		return nil
	}

	fh.file.muRW.Lock()
	defer fh.file.muRW.Unlock()
	err := fh.flushWrites(false)

	if err != nil {
		fs.Errorf(fh.logPrefix(), "RWFileHandle.Flush error: %v", err)
	} else {
		// fs.Debugf(fh.logPrefix(), "RWFileHandle.Flush OK")
	}
	return err
}

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *RWFileHandle) Release() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		fs.Debugf(fh.logPrefix(), "RWFileHandle.Release nothing to do")
		return nil
	}
	fs.Debugf(fh.logPrefix(), "RWFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.logPrefix(), "RWFileHandle.Release error: %v", err)
	} else {
		// fs.Debugf(fh.logPrefix(), "RWFileHandle.Release OK")
	}
	return err
}

// Size returns the size of the underlying file
func (fh *RWFileHandle) Size() int64 {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if !fh.opened {
		return fh.file.Size()
	}
	fi, err := fh.File.Stat()
	if err != nil {
		return 0
	}
	return fi.Size()
}

// Stat returns info about the file
func (fh *RWFileHandle) Stat() (os.FileInfo, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file, nil
}

// readFn is a general purpose read function - pass in a closure to do
// the actual read
func (fh *RWFileHandle) readFn(read func() (int, error)) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return 0, ECLOSED
	}
	if fh.flags&accessModeMask == os.O_WRONLY {
		return 0, EBADF
	}
	if err = fh.openPending(false); err != nil {
		return n, err
	}
	return read()
}

// Read bytes from the file
func (fh *RWFileHandle) Read(b []byte) (n int, err error) {
	return fh.readFn(func() (int, error) {
		return fh.File.Read(b)
	})
}

// ReadAt bytes from the file at off
func (fh *RWFileHandle) ReadAt(b []byte, off int64) (n int, err error) {
	return fh.readFn(func() (int, error) {
		return fh.File.ReadAt(b, off)
	})
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
	if err = fh.openPending(false); err != nil {
		return ret, err
	}
	return fh.File.Seek(offset, whence)
}

// writeFn general purpose write call
//
// Pass a closure to do the actual write
func (fh *RWFileHandle) writeFn(write func() error) (err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return ECLOSED
	}
	if fh.flags&accessModeMask == os.O_RDONLY {
		return EBADF
	}
	if err = fh.openPending(false); err != nil {
		return err
	}
	fh.writeCalled = true
	err = write()
	if err != nil {
		return err
	}
	fi, err := fh.File.Stat()
	if err != nil {
		return errors.Wrap(err, "failed to stat cache file")
	}
	fh.file.setSize(fi.Size())
	return nil
}

// Write bytes to the file
func (fh *RWFileHandle) Write(b []byte) (n int, err error) {
	err = fh.writeFn(func() error {
		n, err = fh.File.Write(b)
		return err
	})
	return n, err
}

// WriteAt bytes to the file at off
func (fh *RWFileHandle) WriteAt(b []byte, off int64) (n int, err error) {
	if fh.flags&os.O_APPEND != 0 {
		// if append is set, call Write as WriteAt returns an error if append is set
		return fh.Write(b)
	}
	err = fh.writeFn(func() error {
		n, err = fh.File.WriteAt(b, off)
		return err
	})
	return n, err
}

// WriteString a string to the file
func (fh *RWFileHandle) WriteString(s string) (n int, err error) {
	err = fh.writeFn(func() error {
		n, err = fh.File.WriteString(s)
		return err
	})
	return n, err
}

// Truncate file to given size
func (fh *RWFileHandle) Truncate(size int64) (err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return ECLOSED
	}
	if err = fh.openPending(size == 0); err != nil {
		return err
	}
	fh.changed = true
	fh.file.setSize(size)
	return fh.File.Truncate(size)
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
	if fh.flags&accessModeMask == os.O_RDONLY {
		return nil
	}
	return fh.File.Sync()
}

func (fh *RWFileHandle) logPrefix() string {
	return fmt.Sprintf("%s(%p)", fh.file.Path(), fh)
}
