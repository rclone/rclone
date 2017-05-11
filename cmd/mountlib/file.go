package mountlib

import (
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
		inode: NewInode(),
	}
}

// String converts it to printable
func (f *File) String() string {
	if f == nil {
		return "<nil *File>"
	}
	return path.Join(f.d.path, f.leaf)
}

// IsFile returns true for File - satisfies Node interface
func (f *File) IsFile() bool {
	return true
}

// Inode returns the inode number - satisfies Node interface
func (f *File) Inode() uint64 {
	return f.inode
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (f *File) Node() Node {
	return f
}

// rename should be called to update f.o and f.d after a rename
func (f *File) rename(d *Dir, o fs.Object) {
	f.mu.Lock()
	f.o = o
	f.d = d
	f.mu.Unlock()
}

// addWriters increments or decrements the writers
func (f *File) addWriters(n int) {
	f.mu.Lock()
	f.writers += n
	f.mu.Unlock()
}

// Attr fills out the attributes for the file
func (f *File) Attr(noModTime bool) (modTime time.Time, Size, Blocks uint64, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// if o is nil it isn't valid yet or there are writers, so return the size so far
	if f.o == nil || f.writers != 0 {
		Size = uint64(atomic.LoadInt64(&f.size))
		if !noModTime && !f.pendingModTime.IsZero() {
			modTime = f.pendingModTime
		}
	} else {
		Size = uint64(f.o.Size())
		if !noModTime {
			modTime = f.o.ModTime()
		}
	}
	Blocks = (Size + 511) / 512
	// fs.Debugf(f.o, "File.Attr modTime=%v, Size=%d, Blocks=%v", modTime, Size, Blocks)
	return
}

// SetModTime sets the modtime for the file
func (f *File) SetModTime(modTime time.Time) error {
	if f.d.fsys.readOnly {
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
	case fs.ErrorCantSetModTime:
		// do nothing, in order to not break "touch somefile" if it exists already
	default:
		fs.Errorf(f.o, "File.applyPendingModTime error: %v", err)
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
	f.d.addObject(o, f)
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
		fs.Errorf(o, "File.OpenRead failed: %v", err)
		return nil, err
	}
	return fh, nil
}

// OpenWrite open the file for write
func (f *File) OpenWrite() (fh *WriteFileHandle, err error) {
	if f.d.fsys.readOnly {
		return nil, EROFS
	}
	// if o is nil it isn't valid yet
	o, err := f.waitForValidObject()
	if err != nil {
		return nil, err
	}
	// fs.Debugf(o, "File.OpenWrite")

	src := newCreateInfo(f.d.f, o.Remote())
	fh, err = newWriteFileHandle(f.d, f, src)
	err = errors.Wrap(err, "open for write")

	if err != nil {
		fs.Errorf(o, "File.OpenWrite failed: %v", err)
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
