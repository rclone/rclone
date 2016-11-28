// +build linux darwin freebsd

package mount

import (
	"sync"
	"sync/atomic"
	"time"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// File represents a file
type File struct {
	size    int64        // size of file - read and written with atomic int64 - must be 64 bit aligned
	d       *Dir         // parent directory - read only
	mu      sync.RWMutex // protects the following
	o       fs.Object    // NB o may be nil if file is being written
	writers int          // number of writers for this file
}

// newFile creates a new File
func newFile(d *Dir, o fs.Object) *File {
	return &File{
		d: d,
		o: o,
	}
}

// addWriters increments or decrements the writers
func (f *File) addWriters(n int) {
	f.mu.Lock()
	f.writers += n
	f.mu.Unlock()
}

// Check interface satisfied
var _ fusefs.Node = (*File)(nil)

// Attr fills out the attributes for the file
func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fs.Debug(f.o, "File.Attr")
	a.Gid = gid
	a.Uid = uid
	a.Mode = filePerms
	// if o is nil it isn't valid yet, so return the size so far
	if f.o == nil {
		a.Size = uint64(atomic.LoadInt64(&f.size))
	} else {
		a.Size = uint64(f.o.Size())
		if !noModTime {
			modTime := f.o.ModTime()
			a.Atime = modTime
			a.Mtime = modTime
			a.Ctime = modTime
			a.Crtime = modTime
		}
	}
	a.Blocks = (a.Size + 511) / 512
	return nil
}

// Update the size while writing
func (f *File) written(n int64) {
	atomic.AddInt64(&f.size, n)
}

// Update the object when written
func (f *File) setObject(o fs.Object) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.o = o
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
	return nil, fuse.ENOENT
}

// Check interface satisfied
var _ fusefs.NodeOpener = (*File)(nil)

// Open the file for read or write
func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fusefs.Handle, error) {
	// if o is nil it isn't valid yet
	o, err := f.waitForValidObject()
	if err != nil {
		return nil, err
	}

	fs.Debug(o, "File.Open")

	switch {
	case req.Flags.IsReadOnly():
		if noSeek {
			resp.Flags |= fuse.OpenNonSeekable
		}
		return newReadFileHandle(o)
	case req.Flags.IsWriteOnly() || (req.Flags.IsReadWrite() && (req.Flags&fuse.OpenTruncate) != 0):
		resp.Flags |= fuse.OpenNonSeekable
		src := newCreateInfo(f.d.f, o.Remote())
		fh, err := newWriteFileHandle(f.d, f, src)
		if err != nil {
			return nil, err
		}
		return fh, nil
	case req.Flags.IsReadWrite():
		return nil, errors.New("can't open read and write")
	}

	/*
	   // File was opened in append-only mode, all writes will go to end
	   // of file. OS X does not provide this information.
	   OpenAppend    OpenFlags = syscall.O_APPEND
	   OpenCreate    OpenFlags = syscall.O_CREAT
	   OpenDirectory OpenFlags = syscall.O_DIRECTORY
	   OpenExclusive OpenFlags = syscall.O_EXCL
	   OpenNonblock  OpenFlags = syscall.O_NONBLOCK
	   OpenSync      OpenFlags = syscall.O_SYNC
	   OpenTruncate  OpenFlags = syscall.O_TRUNC
	*/
	return nil, errors.New("can't figure out how to open")
}
