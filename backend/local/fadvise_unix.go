//go:build linux

package local

import (
	"io"
	"os"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

// fadvise provides means to automate freeing pages in kernel page cache for
// a given file descriptor as the file is sequentially processed (read or
// written).
//
// When copying a file to a remote backend all the file content is read by
// kernel and put to page cache to make future reads faster.
// This causes memory pressure visible in both memory usage and CPU consumption
// and can even cause OOM errors in applications consuming large amounts memory.
//
// In case of an upload to a remote backend, there is no benefits from caching.
//
// fadvise would orchestrate calling POSIX_FADV_DONTNEED
//
// POSIX_FADV_DONTNEED attempts to free cached pages associated
// with the specified region.  This is useful, for example, while
// streaming large files.  A program may periodically request the
// kernel to free cached data that has already been used, so that
// more useful cached pages are not discarded instead.
//
// Requests to discard partial pages are ignored.  It is
// preferable to preserve needed data than discard unneeded data.
// If the application requires that data be considered for
// discarding, then offset and len must be page-aligned.
//
// The implementation may attempt to write back dirty pages in
// the specified region, but this is not guaranteed.  Any
// unwritten dirty pages will not be freed.  If the application
// wishes to ensure that dirty pages will be released, it should
// call fsync(2) or fdatasync(2) first.
type fadvise struct {
	o          *Object
	fd         int
	lastPos    int64
	curPos     int64
	windowSize int64

	freePagesCh chan offsetLength
	doneCh      chan struct{}
}

type offsetLength struct {
	offset int64
	length int64
}

const (
	defaultAllowPages      = 32
	defaultWorkerQueueSize = 64
)

func newFadvise(o *Object, fd int, offset int64) *fadvise {
	f := &fadvise{
		o:          o,
		fd:         fd,
		lastPos:    offset,
		curPos:     offset,
		windowSize: int64(os.Getpagesize()) * defaultAllowPages,

		freePagesCh: make(chan offsetLength, defaultWorkerQueueSize),
		doneCh:      make(chan struct{}),
	}
	go f.worker()

	return f
}

// sequential configures readahead strategy in Linux kernel.
//
// Under Linux, POSIX_FADV_NORMAL sets the readahead window to the
// default size for the backing device; POSIX_FADV_SEQUENTIAL doubles
// this size, and POSIX_FADV_RANDOM disables file readahead entirely.
func (f *fadvise) sequential(limit int64) bool {
	l := int64(0)
	if limit > 0 {
		l = limit
	}
	if err := unix.Fadvise(f.fd, f.curPos, l, unix.FADV_SEQUENTIAL); err != nil {
		fs.Debugf(f.o, "fadvise sequential failed on file descriptor %d: %s", f.fd, err)
		return false
	}

	return true
}

func (f *fadvise) next(n int) {
	f.curPos += int64(n)
	f.freePagesIfNeeded()
}

func (f *fadvise) freePagesIfNeeded() {
	if f.curPos >= f.lastPos+f.windowSize {
		f.freePages()
	}
}

func (f *fadvise) freePages() {
	f.freePagesCh <- offsetLength{f.lastPos, f.curPos - f.lastPos}
	f.lastPos = f.curPos
}

func (f *fadvise) worker() {
	for p := range f.freePagesCh {
		if err := unix.Fadvise(f.fd, p.offset, p.length, unix.FADV_DONTNEED); err != nil {
			fs.Debugf(f.o, "fadvise dontneed failed on file descriptor %d: %s", f.fd, err)
		}
	}

	close(f.doneCh)
}

func (f *fadvise) wait() {
	close(f.freePagesCh)
	<-f.doneCh
}

type fadviseReadCloser struct {
	*fadvise
	inner io.ReadCloser
}

// newFadviseReadCloser wraps os.File so that reading from that file would
// remove already consumed pages from kernel page cache.
// In addition to that it instructs kernel to double the readahead window to
// make sequential reads faster.
// See also fadvise.
func newFadviseReadCloser(o *Object, f *os.File, offset, limit int64) io.ReadCloser {
	r := fadviseReadCloser{
		fadvise: newFadvise(o, int(f.Fd()), offset),
		inner:   f,
	}

	// If syscall failed it's likely that the subsequent syscalls to that
	// file descriptor would also fail. In that case return the provided os.File
	// pointer.
	if !r.sequential(limit) {
		r.wait()
		return f
	}

	return r
}

func (f fadviseReadCloser) Read(p []byte) (n int, err error) {
	n, err = f.inner.Read(p)
	f.next(n)
	return
}

func (f fadviseReadCloser) Close() error {
	f.freePages()
	f.wait()
	return f.inner.Close()
}
