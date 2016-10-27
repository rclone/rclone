// +build linux darwin freebsd

package mount

import (
	"io"
	"sync"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"golang.org/x/net/context"
)

// ReadFileHandle is an open for read file handle on a File
type ReadFileHandle struct {
	mu         sync.Mutex
	closed     bool // set if handle has been closed
	r          io.ReadCloser
	o          fs.Object
	readCalled bool // set if read has been called
	offset     int64
}

func newReadFileHandle(o fs.Object) (*ReadFileHandle, error) {
	r, err := o.Open()
	if err != nil {
		return nil, err
	}
	return &ReadFileHandle{
		r: r,
		o: o,
	}, nil
}

// Check interface satisfied
var _ fusefs.Handle = (*ReadFileHandle)(nil)

// Check interface satisfied
var _ fusefs.HandleReader = (*ReadFileHandle)(nil)

// seek to a new offset
func (fh *ReadFileHandle) seek(offset int64) error {
	// Can we seek it directly?
	if do, ok := fh.r.(io.Seeker); ok {
		fs.Debug(fh.o, "ReadFileHandle.seek from %d to %d (io.Seeker)", fh.offset, offset)
		_, err := do.Seek(offset, 0)
		if err != nil {
			fs.Debug(fh.o, "ReadFileHandle.Read io.Seeker failed: %v", err)
			return err
		}
	} else {
		fs.Debug(fh.o, "ReadFileHandle.seek from %d to %d", fh.offset, offset)
		// if not re-open with a seek
		r, err := fh.o.Open(&fs.SeekOption{Offset: offset})
		if err != nil {
			fs.Debug(fh.o, "ReadFileHandle.Read seek failed: %v", err)
			return err
		}
		err = fh.r.Close()
		if err != nil {
			fs.Debug(fh.o, "ReadFileHandle.Read seek close old failed: %v", err)
		}
		fh.r = r
	}
	fh.offset = offset
	return nil
}

// Read from the file handle
func (fh *ReadFileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	fs.Debug(fh.o, "ReadFileHandle.Read size %d offset %d", req.Size, req.Offset)
	if fh.closed {
		fs.ErrorLog(fh.o, "ReadFileHandle.Read error: %v", errClosedFileHandle)
		return errClosedFileHandle
	}
	if req.Offset != fh.offset {
		err := fh.seek(req.Offset)
		if err != nil {
			return err
		}
	}
	if req.Size > 0 {
		fh.readCalled = true
	}
	// We don't actually enforce Offset to match where previous read
	// ended. Maybe we should, but that would mean'd we need to track
	// it. The kernel *should* do it for us, based on the
	// fuse.OpenNonSeekable flag.
	//
	// One exception to the above is if we fail to fully populate a
	// page cache page; a read into page cache is always page aligned.
	// Make sure we never serve a partial read, to avoid that.
	buf := make([]byte, req.Size)
	n, err := io.ReadFull(fh.r, buf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]
	fh.offset += int64(n)
	if err != nil {
		fs.ErrorLog(fh.o, "ReadFileHandle.Read error: %v", err)
	} else {
		fs.Debug(fh.o, "ReadFileHandle.Read OK")
	}
	return err
}

// close the file handle returning errClosedFileHandle if it has been
// closed already.
//
// Must be called with fh.mu held
func (fh *ReadFileHandle) close() error {
	if fh.closed {
		return errClosedFileHandle
	}
	fh.closed = true
	return fh.r.Close()
}

// Check interface satisfied
var _ fusefs.HandleFlusher = (*ReadFileHandle)(nil)

// Flush is called each time the file or directory is closed.
// Because there can be multiple file descriptors referring to a
// single opened file, Flush can be called multiple times.
func (fh *ReadFileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	fs.Debug(fh.o, "ReadFileHandle.Flush")

	// Ignore the Flush as there is nothing we can sensibly do and
	// it seems quite common for Flush to be called from
	// different threads each of which have read some data.
	if false {
		// If Read hasn't been called then ignore the Flush - Release
		// will pick it up
		if !fh.readCalled {
			fs.Debug(fh.o, "ReadFileHandle.Flush ignoring flush on unread handle")
			return nil

		}
		err := fh.close()
		if err != nil {
			fs.ErrorLog(fh.o, "ReadFileHandle.Flush error: %v", err)
			return err
		}
	}
	fs.Debug(fh.o, "ReadFileHandle.Flush OK")
	return nil
}

var _ fusefs.HandleReleaser = (*ReadFileHandle)(nil)

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *ReadFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		fs.Debug(fh.o, "ReadFileHandle.Release nothing to do")
		return nil
	}
	fs.Debug(fh.o, "ReadFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.ErrorLog(fh.o, "ReadFileHandle.Release error: %v", err)
	} else {
		fs.Debug(fh.o, "ReadFileHandle.Release OK")
	}
	return err
}
