package vfs

import (
	"io"
	"os"
	"sync"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// RWFileHandle is a handle that can be open for read and write.
//
// It will be open to a temporary file which, when closed, will be
// transferred to the remote.
type RWFileHandle struct {
	*os.File
	mu          sync.Mutex
	closed      bool      // set if handle has been closed
	o           fs.Object // may be nil
	remote      string
	file        *File
	d           *Dir
	opened      bool
	flags       int    // open flags
	osPath      string // path to the file in the cache
	writeCalled bool   // if any Write() methods have been called
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

func newRWFileHandle(d *Dir, f *File, remote string, flags int) (fh *RWFileHandle, err error) {
	// Make a place for the file
	osPath, err := d.vfs.cache.mkdir(remote)
	if err != nil {
		return nil, errors.Wrap(err, "open RW handle failed to make cache directory")
	}

	fh = &RWFileHandle{
		o:      f.o,
		file:   f,
		d:      d,
		remote: remote,
		flags:  flags,
		osPath: osPath,
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

	rdwrMode := fh.flags & accessModeMask

	// if not truncating the file, need to read it first
	if fh.flags&os.O_TRUNC == 0 && !truncate {
		// Fetch the file if it hasn't changed
		// FIXME retries
		err = fs.CopyFile(fh.d.vfs.cache.f, fh.d.vfs.f, fh.remote, fh.remote)
		if err != nil {
			// if the object wasn't found AND O_CREATE is set then...
			cause := errors.Cause(err)
			notFound := cause == fs.ErrorObjectNotFound || cause == fs.ErrorDirNotFound
			if notFound {
				// Remove cached item if there is one
				err = os.Remove(fh.osPath)
				if err != nil && !os.IsNotExist(err) {
					return errors.Wrap(err, "open RW handle failed to delete stale cache file")
				}
			}
			if notFound && fh.flags&os.O_CREATE != 0 {
				// ...ignore error as we are about to create the file
			} else {
				return errors.Wrap(err, "open RW handle failed to cache file")
			}
		}
	} else {
		// Set the size to 0 since we are truncating
		fh.file.setSize(0)
	}

	if rdwrMode != os.O_RDONLY {
		fh.file.addWriters(1)
	}

	fs.Debugf(fh.remote, "Opening cached copy with flags=0x%02X", fh.flags)
	fd, err := os.OpenFile(fh.osPath, fh.flags|os.O_CREATE, 0600)
	if err != nil {
		return errors.Wrap(err, "cache open file failed")
	}
	fh.File = fd
	fh.opened = true
	fh.d.vfs.cache.open(fh.osPath)
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

// close the file handle returning EBADF if it has been
// closed already.
//
// Must be called with fh.mu held
//
// Note that we leave the file around in the cache on error conditions
// to give the user a chance to recover it.
func (fh *RWFileHandle) close() (err error) {
	defer fs.Trace(fh.remote, "")("err=%v", &err)
	if fh.closed {
		return ECLOSED
	}
	fh.closed = true
	rdwrMode := fh.flags & accessModeMask
	if !fh.opened {
		// If read only then return
		if rdwrMode == os.O_RDONLY {
			return nil
		}
		// If we aren't creating or truncating the file then
		// we haven't modified it so don't need to transfer it
		if fh.flags&(os.O_CREATE|os.O_TRUNC) == 0 {
			return nil
		}
		// Otherwise open the file
		// FIXME this could be more efficient
		if err := fh.openPending(false); err != nil {
			return err
		}
	}
	if rdwrMode != os.O_RDONLY {
		fh.file.addWriters(-1)
		fi, err := fh.File.Stat()
		if err != nil {
			fs.Errorf(fh.remote, "Failed to stat cache file: %v", err)
		} else {
			fh.file.setSize(fi.Size())
		}
	}
	fh.d.vfs.cache.close(fh.osPath)

	// Close the underlying file
	err = fh.File.Close()
	if err != nil {
		return err
	}

	// FIXME measure whether we actually did any writes or not -
	// no writes means no transfer?
	if rdwrMode == os.O_RDONLY {
		fs.Debugf(fh.remote, "read only so not transferring")
		return nil
	}

	// If write hasn't been called and we aren't creating or
	// truncating the file then we haven't modified it so don't
	// need to transfer it
	if !fh.writeCalled && fh.flags&(os.O_CREATE|os.O_TRUNC) == 0 {
		fs.Debugf(fh.remote, "not modified so not transferring")
		return nil
	}

	// Transfer the temp file to the remote
	// FIXME retries
	if fh.d.vfs.Opt.CacheMode < CacheModeFull {
		err = fs.MoveFile(fh.d.vfs.f, fh.d.vfs.cache.f, fh.remote, fh.remote)
	} else {
		err = fs.CopyFile(fh.d.vfs.f, fh.d.vfs.cache.f, fh.remote, fh.remote)
	}
	if err != nil {
		err = errors.Wrap(err, "failed to transfer file from cache to remote")
		fs.Errorf(fh.remote, "%v", err)
		return err
	}

	// FIXME get MoveFile to return this object
	o, err := fh.d.vfs.f.NewObject(fh.remote)
	if err != nil {
		err = errors.Wrap(err, "failed to find object after transfer to remote")
		fs.Errorf(fh.remote, "%v", err)
		return err
	}
	fh.file.setObject(o)
	fs.Debugf(o, "transferred to remote")

	return nil
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
		fs.Debugf(fh.remote, "RWFileHandle.Flush nothing to do")
		return nil
	}
	// fs.Debugf(fh.remote, "RWFileHandle.Flush")
	if !fh.opened {
		fs.Debugf(fh.remote, "RWFileHandle.Flush ignoring flush on unopened handle")
		return nil
	}

	// If Write hasn't been called then ignore the Flush - Release
	// will pick it up
	if !fh.writeCalled {
		fs.Debugf(fh.remote, "RWFileHandle.Flush ignoring flush on unwritten handle")
		return nil
	}
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.remote, "RWFileHandle.Flush error: %v", err)
	} else {
		// fs.Debugf(fh.remote, "RWFileHandle.Flush OK")
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
		fs.Debugf(fh.remote, "RWFileHandle.Release nothing to do")
		return nil
	}
	fs.Debugf(fh.remote, "RWFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.remote, "RWFileHandle.Release error: %v", err)
	} else {
		// fs.Debugf(fh.remote, "RWFileHandle.Release OK")
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

// Read bytes from the file
func (fh *RWFileHandle) Read(b []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return 0, ECLOSED
	}
	if err = fh.openPending(false); err != nil {
		return n, err
	}
	return fh.File.Read(b)
}

// ReadAt bytes from the file at off
func (fh *RWFileHandle) ReadAt(b []byte, off int64) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return 0, ECLOSED
	}
	if err = fh.openPending(false); err != nil {
		return n, err
	}
	return fh.File.ReadAt(b, off)
}

// Seek to new file position
func (fh *RWFileHandle) Seek(offset int64, whence int) (ret int64, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		return 0, ECLOSED
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
	fh.writeCalled = true
	fh.file.setSize(size)
	return fh.File.Truncate(size)
}
