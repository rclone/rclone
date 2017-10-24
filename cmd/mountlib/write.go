package mountlib

import (
	"io"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
)

// WriteFileHandle is an open for write handle on a File
type WriteFileHandle struct {
	mu          sync.Mutex
	closed      bool // set if handle has been closed
	remote      string
	pipeWriter  *io.PipeWriter
	o           fs.Object
	result      chan error
	file        *File
	writeCalled bool // set the first time Write() is called
	offset      int64
}

func newWriteFileHandle(d *Dir, f *File, src fs.ObjectInfo) (*WriteFileHandle, error) {
	fh := &WriteFileHandle{
		remote: src.Remote(),
		result: make(chan error, 1),
		file:   f,
	}
	var pipeReader *io.PipeReader
	pipeReader, fh.pipeWriter = io.Pipe()
	go func() {
		o, err := fs.Rcat(d.f, src.Remote(), pipeReader, time.Now())
		if err != nil {
			fs.Errorf(fh.remote, "WriteFileHandle.New Rcat failed: %v", err)
		}
		// Close the pipeReader so the pipeWriter fails with ErrClosedPipe
		_ = pipeReader.Close()
		fh.o = o
		fh.result <- err
	}()
	fh.file.addWriters(1)
	fh.file.setSize(0)
	return fh, nil
}

// String converts it to printable
func (fh *WriteFileHandle) String() string {
	if fh == nil {
		return "<nil *WriteFileHandle>"
	}
	if fh.file == nil {
		return "<nil *WriteFileHandle.file>"
	}
	return fh.file.String() + " (w)"
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (fh *WriteFileHandle) Node() Node {
	return fh.file
}

// Write data to the file handle
func (fh *WriteFileHandle) Write(data []byte, offset int64) (written int64, err error) {
	// fs.Debugf(fh.remote, "WriteFileHandle.Write len=%d", len(data))
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.offset != offset {
		fs.Errorf(fh.remote, "WriteFileHandle.Write can't seek in file")
		return 0, ESPIPE
	}
	if fh.closed {
		fs.Errorf(fh.remote, "WriteFileHandle.Write error: %v", EBADF)
		return 0, EBADF
	}
	fh.writeCalled = true
	n, err := fh.pipeWriter.Write(data)
	written = int64(n)
	fh.offset += written
	fh.file.setSize(fh.offset)
	if err != nil {
		fs.Errorf(fh.remote, "WriteFileHandle.Write error: %v", err)
		return 0, err
	}
	// fs.Debugf(fh.remote, "WriteFileHandle.Write OK (%d bytes written)", n)
	return written, nil
}

// Offset returns the offset of the file pointer
func (fh *WriteFileHandle) Offset() (offset int64) {
	return fh.offset
}

// close the file handle returning EBADF if it has been
// closed already.
//
// Must be called with fh.mu held
func (fh *WriteFileHandle) close() error {
	if fh.closed {
		return EBADF
	}
	fh.closed = true
	fh.file.addWriters(-1)
	writeCloseErr := fh.pipeWriter.Close()
	err := <-fh.result
	if err == nil {
		fh.file.setObject(fh.o)
		err = writeCloseErr
	}
	return err
}

// Flush is called on each close() of a file descriptor. So if a
// filesystem wants to return write errors in close() and the file has
// cached dirty data, this is a good place to write back data and
// return any errors. Since many applications ignore close() errors
// this is not always useful.
//
// NOTE: The flush() method may be called more than once for each
// open(). This happens if more than one file descriptor refers to an
// opened file due to dup(), dup2() or fork() calls. It is not
// possible to determine if a flush is final, so each flush should be
// treated equally. Multiple write-flush sequences are relatively
// rare, so this shouldn't be a problem.
//
// Filesystems shouldn't assume that flush will always be called after
// some writes, or that if will be called at all.
func (fh *WriteFileHandle) Flush() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	// fs.Debugf(fh.remote, "WriteFileHandle.Flush")
	// If Write hasn't been called then ignore the Flush - Release
	// will pick it up
	if !fh.writeCalled {
		fs.Debugf(fh.remote, "WriteFileHandle.Flush ignoring flush on unwritten handle")
		return nil

	}
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.remote, "WriteFileHandle.Flush error: %v", err)
	} else {
		// fs.Debugf(fh.remote, "WriteFileHandle.Flush OK")
	}
	return err
}

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *WriteFileHandle) Release() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		fs.Debugf(fh.remote, "WriteFileHandle.Release nothing to do")
		return nil
	}
	fs.Debugf(fh.remote, "WriteFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.remote, "WriteFileHandle.Release error: %v", err)
	} else {
		// fs.Debugf(fh.remote, "WriteFileHandle.Release OK")
	}
	return err
}

// Close closes the file calling Flush then Release
func (fh *WriteFileHandle) Close() error {
	err := fh.Flush()
	err2 := fh.Release()
	if err != nil {
		return err
	}
	return err2
}
