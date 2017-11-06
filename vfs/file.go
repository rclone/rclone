package vfs

import (
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// File represents a file
type File struct {
	inode          uint64       // inode number
	size           int64        // size of file - read and written with atomic int64 - must be 64 bit aligned
	d              *Dir         // parent directory - read only
	mu             sync.RWMutex // protects the following
	o              fs.Object    // NB o may be nil if file is being written
	leaf           string       // leaf name of the object
	writers        int          // number of writers for this file
	pendingModTime time.Time    // will be applied once o becomes available, i.e. after file was written
}

// newFile creates a new File
func newFile(d *Dir, o fs.Object, leaf string) *File {
	return &File{
		d:     d,
		o:     o,
		leaf:  leaf,
		inode: newInode(),
	}
}

// path returns the full path of the file
func (f *File) path() string {
	return path.Join(f.d.path, f.leaf)
}

// String converts it to printable
func (f *File) String() string {
	if f == nil {
		return "<nil *File>"
	}
	return f.path()
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
	return f.d.vfs.Opt.FilePerms
}

// Name (base) of the directory - satisfies Node interface
func (f *File) Name() (name string) {
	return f.leaf
}

// Sys returns underlying data source (can be nil) - satisfies Node interface
func (f *File) Sys() interface{} {
	return nil
}

// Inode returns the inode number - satisfies Node interface
func (f *File) Inode() uint64 {
	return f.inode
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (f *File) Node() Node {
	return f
}

// rename should be called to update the internals after a rename
func (f *File) rename(d *Dir, o fs.Object) {
	f.mu.Lock()
	f.o = o
	f.d = d
	f.leaf = path.Base(o.Remote())
	f.mu.Unlock()
}

// addWriters increments or decrements the writers
func (f *File) addWriters(n int) {
	f.mu.Lock()
	f.writers += n
	f.mu.Unlock()
}

// ModTime returns the modified time of the file
//
// if NoModTime is set then it returns the mod time of the directory
func (f *File) ModTime() (modTime time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.d.vfs.Opt.NoModTime {
		// if o is nil it isn't valid yet or there are writers, so return the size so far
		if f.o == nil || f.writers != 0 {
			if !f.pendingModTime.IsZero() {
				return f.pendingModTime
			}
		} else {
			return f.o.ModTime()
		}
	}

	return f.d.modTime
}

// Size of the file
func (f *File) Size() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	// if o is nil it isn't valid yet or there are writers, so return the size so far
	if f.o == nil || f.writers != 0 {
		return atomic.LoadInt64(&f.size)
	}
	return f.o.Size()
}

// SetModTime sets the modtime for the file
func (f *File) SetModTime(modTime time.Time) error {
	if f.d.vfs.Opt.ReadOnly {
		return EROFS
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pendingModTime = modTime

	if f.o != nil {
		return f.applyPendingModTime()
	}

	// queue up for later, hoping f.o becomes available
	return nil
}

// call with the mutex held
func (f *File) applyPendingModTime() error {
	defer func() { f.pendingModTime = time.Time{} }()

	if f.pendingModTime.IsZero() {
		return nil
	}

	if f.o == nil {
		return errors.New("Cannot apply ModTime, file object is not available")
	}

	err := f.o.SetModTime(f.pendingModTime)
	switch err {
	case nil:
		fs.Debugf(f.o, "File.applyPendingModTime OK")
	case fs.ErrorCantSetModTime, fs.ErrorCantSetModTimeWithoutDelete:
		// do nothing, in order to not break "touch somefile" if it exists already
	default:
		fs.Errorf(f, "File.applyPendingModTime error: %v", err)
		return err
	}

	return nil
}

// Update the size while writing
func (f *File) setSize(n int64) {
	atomic.StoreInt64(&f.size, n)
}

// Update the object when written
func (f *File) setObject(o fs.Object) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.o = o
	_ = f.applyPendingModTime()
	f.d.addObject(f)
}

// Wait for f.o to become non nil for a short time returning it or an
// error
//
// Call without the mutex held
func (f *File) waitForValidObject() (o fs.Object, err error) {
	for i := 0; i < 50; i++ {
		f.mu.Lock()
		o = f.o
		writers := f.writers
		f.mu.Unlock()
		if o != nil {
			return o, nil
		}
		if writers == 0 {
			return nil, errors.New("can't open file - writer failed")
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, ENOENT
}

// OpenRead open the file for read
func (f *File) OpenRead() (fh *ReadFileHandle, err error) {
	// if o is nil it isn't valid yet
	o, err := f.waitForValidObject()
	if err != nil {
		return nil, err
	}
	// fs.Debugf(o, "File.OpenRead")

	fh, err = newReadFileHandle(f, o)
	err = errors.Wrap(err, "open for read")

	if err != nil {
		fs.Errorf(f, "File.OpenRead failed: %v", err)
		return nil, err
	}
	return fh, nil
}

// OpenWrite open the file for write
func (f *File) OpenWrite() (fh *WriteFileHandle, err error) {
	if f.d.vfs.Opt.ReadOnly {
		return nil, EROFS
	}
	// fs.Debugf(o, "File.OpenWrite")

	fh, err = newWriteFileHandle(f.d, f, f.path())
	err = errors.Wrap(err, "open for write")

	if err != nil {
		fs.Errorf(f, "File.OpenWrite failed: %v", err)
		return nil, err
	}
	return fh, nil
}

// Fsync the file
//
// Note that we don't do anything except return OK
func (f *File) Fsync() error {
	return nil
}

// Remove the file
func (f *File) Remove() error {
	if f.d.vfs.Opt.ReadOnly {
		return EROFS
	}
	if f.o != nil {
		err := f.o.Remove()
		if err != nil {
			fs.Errorf(f, "File.Remove file error: %v", err)
			return err
		}
	}
	// Remove the item from the directory listing
	f.d.delObject(f.Name())
	return nil
}

// RemoveAll the file - same as remove for files
func (f *File) RemoveAll() error {
	return f.Remove()
}

// DirEntry returns the underlying fs.DirEntry - may be nil
func (f *File) DirEntry() (entry fs.DirEntry) {
	return f.o
}

// Dir returns the directory this file is in
func (f *File) Dir() *Dir {
	return f.d
}

// VFS returns the instance of the VFS
func (f *File) VFS() *VFS {
	return f.d.vfs
}

// Open a file according to the flags provided
func (f *File) Open(flags int) (fd Handle, err error) {
	rdwrMode := flags & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR)
	var read bool
	switch {
	case rdwrMode == os.O_RDONLY:
		read = true
	case rdwrMode == os.O_WRONLY || (rdwrMode == os.O_RDWR && (flags&os.O_TRUNC) != 0):
		read = false
	case rdwrMode == os.O_RDWR:
		fs.Errorf(f, "Can't open for Read and Write")
		return nil, EPERM
	default:
		fs.Errorf(f, "Can't figure out how to open with flags: 0x%X", flags)
		return nil, EPERM
	}
	if read {
		fd, err = f.OpenRead()
	} else {
		fd, err = f.OpenWrite()
	}
	return fd, err
}
