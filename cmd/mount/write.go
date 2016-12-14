// +build linux darwin freebsd

package mount

import (
	"errors"
	"io"
	"sync"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"golang.org/x/net/context"
)

var errClosedFileHandle = errors.New("Attempt to use closed file handle")

// WriteFileHandle is an open for write handle on a File
type WriteFileHandle struct {
	mu          sync.Mutex
	closed      bool // set if handle has been closed
	remote      string
	pipeReader  *io.PipeReader
	pipeWriter  *io.PipeWriter
	o           fs.Object
	result      chan error
	file        *File
	writeCalled bool // set the first time Write() is called
}

// Check interface satisfied
var _ fusefs.Handle = (*WriteFileHandle)(nil)

func newWriteFileHandle(d *Dir, f *File, src fs.ObjectInfo) (*WriteFileHandle, error) {
	fh := &WriteFileHandle{
		remote: src.Remote(),
		result: make(chan error, 1),
		file:   f,
	}
	fh.pipeReader, fh.pipeWriter = io.Pipe()
	r := fs.NewAccountSizeName(fh.pipeReader, 0, src.Remote()) // account the transfer
	go func() {
		o, err := d.f.Put(r, src)
		fh.o = o
		fh.result <- err
	}()
	fh.file.addWriters(1)
	fs.Stats.Transferring(fh.remote)
	return fh, nil
}

// Check interface satisfied
var _ fusefs.HandleWriter = (*WriteFileHandle)(nil)

// Write data to the file handle
func (fh *WriteFileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fs.Debug(fh.remote, "WriteFileHandle.Write len=%d", len(req.Data))
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		fs.ErrorLog(fh.remote, "WriteFileHandle.Write error: %v", errClosedFileHandle)
		return errClosedFileHandle
	}
	fh.writeCalled = true
	// FIXME should probably check the file isn't being seeked?
	n, err := fh.pipeWriter.Write(req.Data)
	resp.Size = n
	fh.file.written(int64(n))
	if err != nil {
		fs.ErrorLog(fh.remote, "WriteFileHandle.Write error: %v", err)
		return err
	}
	fs.Debug(fh.remote, "WriteFileHandle.Write OK (%d bytes written)", n)
	return nil
}

// close the file handle returning errClosedFileHandle if it has been
// closed already.
//
// Must be called with fh.mu held
func (fh *WriteFileHandle) close() error {
	if fh.closed {
		return errClosedFileHandle
	}
	fh.closed = true
	fs.Stats.DoneTransferring(fh.remote, true)
	fh.file.addWriters(-1)
	writeCloseErr := fh.pipeWriter.Close()
	err := <-fh.result
	readCloseErr := fh.pipeReader.Close()
	if err == nil {
		fh.file.setObject(fh.o)
		err = writeCloseErr
	}
	if err == nil {
		err = readCloseErr
	}
	return err
}

// Check interface satisfied
var _ fusefs.HandleFlusher = (*WriteFileHandle)(nil)

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
func (fh *WriteFileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	fs.Debug(fh.remote, "WriteFileHandle.Flush")
	// If Write hasn't been called then ignore the Flush - Release
	// will pick it up
	if !fh.writeCalled {
		fs.Debug(fh.remote, "WriteFileHandle.Flush ignoring flush on unwritten handle")
		return nil

	}
	err := fh.close()
	if err != nil {
		fs.ErrorLog(fh.remote, "WriteFileHandle.Flush error: %v", err)
	} else {
		fs.Debug(fh.remote, "WriteFileHandle.Flush OK")
	}
	return err
}

var _ fusefs.HandleReleaser = (*WriteFileHandle)(nil)

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *WriteFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.closed {
		fs.Debug(fh.remote, "WriteFileHandle.Release nothing to do")
		return nil
	}
	fs.Debug(fh.remote, "WriteFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.ErrorLog(fh.remote, "WriteFileHandle.Release error: %v", err)
	} else {
		fs.Debug(fh.remote, "WriteFileHandle.Release OK")
	}
	return err
}
