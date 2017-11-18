package vfs

import (
	"io"
	"os"
	"sync"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// ReadFileHandle is an open for read file handle on a File
type ReadFileHandle struct {
	baseHandle
	mu         sync.Mutex
	closed     bool // set if handle has been closed
	r          *fs.Account
	o          fs.Object
	readCalled bool  // set if read has been called
	offset     int64 // offset of read of o
	roffset    int64 // offset of Read() calls
	noSeek     bool
	file       *File
	hash       *fs.MultiHasher
	opened     bool
}

// Check interfaces
var (
	_ io.Reader   = (*ReadFileHandle)(nil)
	_ io.ReaderAt = (*ReadFileHandle)(nil)
	_ io.Seeker   = (*ReadFileHandle)(nil)
	_ io.Closer   = (*ReadFileHandle)(nil)
)

func newReadFileHandle(f *File, o fs.Object) (*ReadFileHandle, error) {
	var hash *fs.MultiHasher
	var err error
	if !f.d.vfs.Opt.NoChecksum {
		hash, err = fs.NewMultiHasherTypes(o.Fs().Hashes())
		if err != nil {
			fs.Errorf(o.Fs(), "newReadFileHandle hash error: %v", err)
		}
	}

	fh := &ReadFileHandle{
		o:      o,
		noSeek: f.d.vfs.Opt.NoSeek,
		file:   f,
		hash:   hash,
	}
	return fh, nil
}

// openPending opens the file if there is a pending open
// call with the lock held
func (fh *ReadFileHandle) openPending() (err error) {
	if fh.opened {
		return nil
	}
	r, err := fh.o.Open()
	if err != nil {
		return err
	}
	fh.r = fs.NewAccount(r, fh.o).WithBuffer() // account the transfer
	fh.opened = true
	fs.Stats.Transferring(fh.o.Remote())
	return nil
}

// String converts it to printable
func (fh *ReadFileHandle) String() string {
	if fh == nil {
		return "<nil *ReadFileHandle>"
	}
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.file == nil {
		return "<nil *ReadFileHandle.file>"
	}
	return fh.file.String() + " (r)"
}

// Node returns the Node assocuated with this - satisfies Noder interface
func (fh *ReadFileHandle) Node() Node {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file
}

// seek to a new offset
//
// if reopen is true, then we won't attempt to use an io.Seeker interface
//
// Must be called with fh.mu held
func (fh *ReadFileHandle) seek(offset int64, reopen bool) (err error) {
	if fh.noSeek {
		return ESPIPE
	}
	fh.r.StopBuffering() // stop the background reading first
	fh.hash = nil
	oldReader := fh.r.GetReader()
	r := oldReader
	// Can we seek it directly?
	if do, ok := oldReader.(io.Seeker); !reopen && ok {
		fs.Debugf(fh.o, "ReadFileHandle.seek from %d to %d (io.Seeker)", fh.offset, offset)
		_, err = do.Seek(offset, 0)
		if err != nil {
			fs.Debugf(fh.o, "ReadFileHandle.Read io.Seeker failed: %v", err)
			return err
		}
	} else {
		fs.Debugf(fh.o, "ReadFileHandle.seek from %d to %d", fh.offset, offset)
		// close old one
		err = oldReader.Close()
		if err != nil {
			fs.Debugf(fh.o, "ReadFileHandle.Read seek close old failed: %v", err)
		}
		// re-open with a seek
		r, err = fh.o.Open(&fs.SeekOption{Offset: offset})
		if err != nil {
			fs.Debugf(fh.o, "ReadFileHandle.Read seek failed: %v", err)
			return err
		}
	}
	fh.r.UpdateReader(r)
	fh.offset = offset
	return nil
}

// Seek the file - returns ESPIPE if seeking isn't possible
func (fh *ReadFileHandle) Seek(offset int64, whence int) (n int64, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.noSeek {
		return 0, ESPIPE
	}
	size := fh.o.Size()
	switch whence {
	case 0:
		fh.roffset = 0
	case 2:
		fh.roffset = size
	}
	fh.roffset += offset
	// we don't check the offset - the next Read will
	return fh.roffset, nil
}

// ReadAt reads len(p) bytes into p starting at offset off in the
// underlying input source. It returns the number of bytes read (0 <=
// n <= len(p)) and any error encountered.
//
// When ReadAt returns n < len(p), it returns a non-nil error
// explaining why more bytes were not returned. In this respect,
// ReadAt is stricter than Read.
//
// Even if ReadAt returns n < len(p), it may use all of p as scratch
// space during the call. If some data is available but not len(p)
// bytes, ReadAt blocks until either all the data is available or an
// error occurs. In this respect ReadAt is different from Read.
//
// If the n = len(p) bytes returned by ReadAt are at the end of the
// input source, ReadAt may return either err == EOF or err == nil.
//
// If ReadAt is reading from an input source with a seek offset,
// ReadAt should not affect nor be affected by the underlying seek
// offset.
//
// Clients of ReadAt can execute parallel ReadAt calls on the same
// input source.
//
// Implementations must not retain p.
func (fh *ReadFileHandle) ReadAt(p []byte, off int64) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.readAt(p, off)
}

// Implementation of ReadAt - call with lock held
func (fh *ReadFileHandle) readAt(p []byte, off int64) (n int, err error) {
	err = fh.openPending() // FIXME pending open could be more efficient in the presense of seek (and retries)
	if err != nil {
		return 0, err
	}
	// fs.Debugf(fh.o, "ReadFileHandle.Read size %d offset %d", reqSize, off)
	if fh.closed {
		fs.Errorf(fh.o, "ReadFileHandle.Read error: %v", EBADF)
		return 0, ECLOSED
	}
	doSeek := off != fh.offset
	if doSeek && fh.noSeek {
		return 0, ESPIPE
	}
	var newOffset int64
	retries := 0
	reqSize := len(p)
	doReopen := false
	for {
		if doSeek {
			// Are we attempting to seek beyond the end of the
			// file - if so just return EOF leaving the underlying
			// file in an unchanged state.
			if off >= fh.o.Size() {
				fs.Debugf(fh.o, "ReadFileHandle.Read attempt to read beyond end of file: %d > %d", off, fh.o.Size())
				return 0, io.EOF
			}
			// Otherwise do the seek
			err = fh.seek(off, doReopen)
		} else {
			err = nil
		}
		if err == nil {
			if reqSize > 0 {
				fh.readCalled = true
			}
			n, err = io.ReadFull(fh.r, p)
			newOffset = fh.offset + int64(n)
			// if err == nil && rand.Intn(10) == 0 {
			// 	err = errors.New("random error")
			// }
			if err == nil {
				break
			} else if (err == io.ErrUnexpectedEOF || err == io.EOF) && newOffset == fh.o.Size() {
				// Have read to end of file - reset error
				err = nil
				break
			}
		}
		if retries >= fs.Config.LowLevelRetries {
			break
		}
		retries++
		fs.Errorf(fh.o, "ReadFileHandle.Read error: low level retry %d/%d: %v", retries, fs.Config.LowLevelRetries, err)
		doSeek = true
		doReopen = true
	}
	if err != nil {
		fs.Errorf(fh.o, "ReadFileHandle.Read error: %v", err)
	} else {
		fh.offset = newOffset
		// fs.Debugf(fh.o, "ReadFileHandle.Read OK")

		if fh.hash != nil {
			_, err = fh.hash.Write(p[:n])
			if err != nil {
				fs.Errorf(fh.o, "ReadFileHandle.Read HashError: %v", err)
				return 0, err
			}
		}

		// If we have no error and we didn't fill the buffer, must be EOF
		if n != len(p) {
			err = io.EOF
		}
	}
	return n, err
}

func (fh *ReadFileHandle) checkHash() error {
	if fh.hash == nil || !fh.readCalled || fh.offset < fh.o.Size() {
		return nil
	}

	for hashType, dstSum := range fh.hash.Sums() {
		srcSum, err := fh.o.Hash(hashType)
		if err != nil {
			return err
		}
		if !fs.HashEquals(dstSum, srcSum) {
			return errors.Errorf("corrupted on transfer: %v hash differ %q vs %q", hashType, dstSum, srcSum)
		}
	}

	return nil
}

// Read reads up to len(p) bytes into p. It returns the number of bytes read (0
// <= n <= len(p)) and any error encountered. Even if Read returns n < len(p),
// it may use all of p as scratch space during the call. If some data is
// available but not len(p) bytes, Read conventionally returns what is
// available instead of waiting for more.
//
// When Read encounters an error or end-of-file condition after successfully
// reading n > 0 bytes, it returns the number of bytes read. It may return the
// (non-nil) error from the same call or return the error (and n == 0) from a
// subsequent call. An instance of this general case is that a Reader returning
// a non-zero number of bytes at the end of the input stream may return either
// err == EOF or err == nil. The next Read should return 0, EOF.
//
// Callers should always process the n > 0 bytes returned before considering
// the error err. Doing so correctly handles I/O errors that happen after
// reading some bytes and also both of the allowed EOF behaviors.
//
// Implementations of Read are discouraged from returning a zero byte count
// with a nil error, except when len(p) == 0. Callers should treat a return of
// 0 and nil as indicating that nothing happened; in particular it does not
// indicate EOF.
//
// Implementations must not retain p.
func (fh *ReadFileHandle) Read(p []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.roffset >= fh.o.Size() {
		return 0, io.EOF
	}
	n, err = fh.readAt(p, fh.roffset)
	fh.roffset += int64(n)
	return n, err
}

// close the file handle returning EBADF if it has been
// closed already.
//
// Must be called with fh.mu held
func (fh *ReadFileHandle) close() error {
	if fh.closed {
		return ECLOSED
	}
	fh.closed = true

	if fh.opened {
		fs.Stats.DoneTransferring(fh.o.Remote(), true)
		// Close first so that we have hashes
		err := fh.r.Close()
		if err != nil {
			return err
		}
		// Now check the hash
		err = fh.checkHash()
		if err != nil {
			return err
		}
	}
	return nil
}

// Close closes the file
func (fh *ReadFileHandle) Close() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.close()
}

// Flush is called each time the file or directory is closed.
// Because there can be multiple file descriptors referring to a
// single opened file, Flush can be called multiple times.
func (fh *ReadFileHandle) Flush() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if !fh.opened {
		return nil
	}
	// fs.Debugf(fh.o, "ReadFileHandle.Flush")

	if err := fh.checkHash(); err != nil {
		fs.Errorf(fh.o, "ReadFileHandle.Flush error: %v", err)
		return err
	}

	// fs.Debugf(fh.o, "ReadFileHandle.Flush OK")
	return nil
}

// Release is called when we are finished with the file handle
//
// It isn't called directly from userspace so the error is ignored by
// the kernel
func (fh *ReadFileHandle) Release() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if !fh.opened {
		return nil
	}
	if fh.closed {
		fs.Debugf(fh.o, "ReadFileHandle.Release nothing to do")
		return nil
	}
	fs.Debugf(fh.o, "ReadFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.o, "ReadFileHandle.Release error: %v", err)
	} else {
		// fs.Debugf(fh.o, "ReadFileHandle.Release OK")
	}
	return err
}

// Size returns the size of the underlying file
func (fh *ReadFileHandle) Size() int64 {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.o.Size()
}

// Stat returns info about the file
func (fh *ReadFileHandle) Stat() (os.FileInfo, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file, nil
}
