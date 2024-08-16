package vfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/chunkedreader"
	"github.com/rclone/rclone/fs/hash"
)

// ReadFileHandle is an open for read file handle on a File
type ReadFileHandle struct {
	baseHandle
	done        func(ctx context.Context, err error)
	mu          sync.Mutex
	cond        sync.Cond // cond lock for out of sequence reads
	r           *accounting.Account
	size        int64 // size of the object (0 for unknown length)
	offset      int64 // offset of read of o
	roffset     int64 // offset of Read() calls
	file        *File
	hash        *hash.MultiHasher
	remote      string
	closed      bool // set if handle has been closed
	readCalled  bool // set if read has been called
	noSeek      bool
	sizeUnknown bool // set if size of source is not known
	opened      bool
}

// Check interfaces
var (
	_ io.Reader   = (*ReadFileHandle)(nil)
	_ io.ReaderAt = (*ReadFileHandle)(nil)
	_ io.Seeker   = (*ReadFileHandle)(nil)
	_ io.Closer   = (*ReadFileHandle)(nil)
)

func newReadFileHandle(f *File) (*ReadFileHandle, error) {
	var mhash *hash.MultiHasher
	var err error
	o := f.getObject()
	if !f.VFS().Opt.NoChecksum {
		hashes := hash.NewHashSet(o.Fs().Hashes().GetOne()) // just pick one hash
		mhash, err = hash.NewMultiHasherTypes(hashes)
		if err != nil {
			fs.Errorf(o.Fs(), "newReadFileHandle hash error: %v", err)
		}
	}

	fh := &ReadFileHandle{
		remote:      o.Remote(),
		noSeek:      f.VFS().Opt.NoSeek,
		file:        f,
		hash:        mhash,
		size:        nonNegative(o.Size()),
		sizeUnknown: o.Size() < 0,
	}
	fh.cond = sync.Cond{L: &fh.mu}
	return fh, nil
}

// openPending opens the file if there is a pending open
// call with the lock held
func (fh *ReadFileHandle) openPending() (err error) {
	if fh.opened {
		return nil
	}
	o := fh.file.getObject()
	opt := &fh.file.VFS().Opt
	r, err := chunkedreader.New(context.TODO(), o, int64(opt.ChunkSize), int64(opt.ChunkSizeLimit), opt.ChunkStreams).Open()
	if err != nil {
		return err
	}
	tr := accounting.GlobalStats().NewTransfer(o, nil)
	fh.done = tr.Done
	fh.r = tr.Account(context.TODO(), r).WithBuffer() // account the transfer
	fh.opened = true

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

// Node returns the Node associated with this - satisfies Noder interface
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
	fh.hash = nil
	if !reopen {
		ar := fh.r.GetAsyncReader()
		// try to fulfill the seek with buffer discard
		if ar != nil && ar.SkipBytes(int(offset-fh.offset)) {
			fh.offset = offset
			return nil
		}
	}
	fh.r.StopBuffering() // stop the background reading first
	oldReader := fh.r.GetReader()
	r, ok := oldReader.(chunkedreader.ChunkedReader)
	if !ok {
		fs.Logf(fh.remote, "ReadFileHandle.Read expected reader to be a ChunkedReader, got %T", oldReader)
		reopen = true
	}
	if !reopen {
		fs.Debugf(fh.remote, "ReadFileHandle.seek from %d to %d (fs.RangeSeeker)", fh.offset, offset)
		_, err = r.RangeSeek(context.TODO(), offset, io.SeekStart, -1)
		if err != nil {
			fs.Debugf(fh.remote, "ReadFileHandle.Read fs.RangeSeeker failed: %v", err)
			return err
		}
	} else {
		fs.Debugf(fh.remote, "ReadFileHandle.seek from %d to %d", fh.offset, offset)
		// close old one
		err = oldReader.Close()
		if err != nil {
			fs.Debugf(fh.remote, "ReadFileHandle.Read seek close old failed: %v", err)
		}
		// re-open with a seek
		o := fh.file.getObject()
		opt := &fh.file.VFS().Opt
		r = chunkedreader.New(context.TODO(), o, int64(opt.ChunkSize), int64(opt.ChunkSizeLimit), opt.ChunkStreams)
		_, err := r.Seek(offset, 0)
		if err != nil {
			fs.Debugf(fh.remote, "ReadFileHandle.Read seek failed: %v", err)
			return err
		}
		r, err = r.Open()
		if err != nil {
			fs.Debugf(fh.remote, "ReadFileHandle.Read seek failed: %v", err)
			return err
		}
	}
	fh.r.UpdateReader(context.TODO(), r)
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
	size := fh.size
	switch whence {
	case io.SeekStart:
		fh.roffset = 0
	case io.SeekEnd:
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

// This waits for *poff to equal off or aborts after the timeout.
//
// Waits here potentially affect all seeks so need to keep them short.
//
// Call with fh.mu Locked
func waitSequential(what string, remote string, cond *sync.Cond, maxWait time.Duration, poff *int64, off int64) {
	var (
		timeout = time.NewTimer(maxWait)
		done    = make(chan struct{})
		abort   atomic.Int32
	)
	go func() {
		select {
		case <-timeout.C:
			// take the lock to make sure that cond.Wait() is called before
			// cond.Broadcast. NB cond.L == mu
			cond.L.Lock()
			// set abort flag and give all the waiting goroutines a kick on timeout
			abort.Store(1)
			fs.Debugf(remote, "aborting in-sequence %s wait, off=%d", what, off)
			cond.Broadcast()
			cond.L.Unlock()
		case <-done:
		}
	}()
	for *poff != off && abort.Load() == 0 {
		fs.Debugf(remote, "waiting for in-sequence %s to %d for %v", what, off, maxWait)
		cond.Wait()
	}
	// tidy up end timer
	close(done)
	timeout.Stop()
	if *poff != off {
		fs.Debugf(remote, "failed to wait for in-sequence %s to %d", what, off)
	}
}

// Implementation of ReadAt - call with lock held
func (fh *ReadFileHandle) readAt(p []byte, off int64) (n int, err error) {
	// defer log.Trace(fh.remote, "p[%d], off=%d", len(p), off)("n=%d, err=%v", &n, &err)
	err = fh.openPending() // FIXME pending open could be more efficient in the presence of seek (and retries)
	if err != nil {
		return 0, err
	}
	// fs.Debugf(fh.remote, "ReadFileHandle.Read size %d offset %d", reqSize, off)
	if fh.closed {
		fs.Errorf(fh.remote, "ReadFileHandle.Read error: %v", EBADF)
		return 0, ECLOSED
	}
	maxBuf := 1024 * 1024
	if len(p) < maxBuf {
		maxBuf = len(p)
	}
	if gap := off - fh.offset; gap > 0 && gap < int64(8*maxBuf) {
		waitSequential("read", fh.remote, &fh.cond, time.Duration(fh.file.VFS().Opt.ReadWait), &fh.offset, off)
	}
	doSeek := off != fh.offset
	if doSeek && fh.noSeek {
		return 0, ESPIPE
	}
	var newOffset int64
	retries := 0
	reqSize := len(p)
	doReopen := false
	lowLevelRetries := fs.GetConfig(context.TODO()).LowLevelRetries
	for {
		if doSeek {
			// Are we attempting to seek beyond the end of the
			// file - if so just return EOF leaving the underlying
			// file in an unchanged state.
			if off >= fh.size {
				fs.Debugf(fh.remote, "ReadFileHandle.Read attempt to read beyond end of file: %d > %d", off, fh.size)
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
			} else if (err == io.ErrUnexpectedEOF || err == io.EOF) && (newOffset == fh.size || fh.sizeUnknown) {
				if fh.sizeUnknown {
					// size is now known since we have read to the end
					fh.sizeUnknown = false
					fh.size = newOffset
				}
				// Have read to end of file - reset error
				err = nil
				break
			}
		}
		if retries >= lowLevelRetries {
			break
		}
		retries++
		fs.Errorf(fh.remote, "ReadFileHandle.Read error: low level retry %d/%d: %v", retries, lowLevelRetries, err)
		doSeek = true
		doReopen = true
	}
	if err != nil {
		fs.Errorf(fh.remote, "ReadFileHandle.Read error: %v", err)
	} else {
		fh.offset = newOffset
		// fs.Debugf(fh.remote, "ReadFileHandle.Read OK")

		if fh.hash != nil {
			_, err = fh.hash.Write(p[:n])
			if err != nil {
				fs.Errorf(fh.remote, "ReadFileHandle.Read HashError: %v", err)
				return 0, err
			}
		}

		// If we have no error and we didn't fill the buffer, must be EOF
		if n != len(p) {
			err = io.EOF
		}
	}
	fh.cond.Broadcast() // wake everyone up waiting for an in-sequence read
	return n, err
}

func (fh *ReadFileHandle) checkHash() error {
	if fh.hash == nil || !fh.readCalled || fh.offset < fh.size {
		return nil
	}

	o := fh.file.getObject()
	for hashType, dstSum := range fh.hash.Sums() {
		srcSum, err := o.Hash(context.TODO(), hashType)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// if it was file not found then at
				// this point we don't care any more
				continue
			}
			return err
		}
		if !hash.Equals(dstSum, srcSum) {
			return fmt.Errorf("corrupted on transfer: %v hashes differ src %q vs dst %q", hashType, srcSum, dstSum)
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
	if fh.roffset >= fh.size && !fh.sizeUnknown {
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
		var err error
		defer func() {
			fh.done(context.TODO(), err)
		}()
		// Close first so that we have hashes
		err = fh.r.Close()
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
	// fs.Debugf(fh.remote, "ReadFileHandle.Flush")

	if err := fh.checkHash(); err != nil {
		fs.Errorf(fh.remote, "ReadFileHandle.Flush error: %v", err)
		return err
	}

	// fs.Debugf(fh.remote, "ReadFileHandle.Flush OK")
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
		fs.Debugf(fh.remote, "ReadFileHandle.Release nothing to do")
		return nil
	}
	fs.Debugf(fh.remote, "ReadFileHandle.Release closing")
	err := fh.close()
	if err != nil {
		fs.Errorf(fh.remote, "ReadFileHandle.Release error: %v", err)
		//} else {
		// fs.Debugf(fh.remote, "ReadFileHandle.Release OK")
	}
	return err
}

// Name returns the name of the file from the underlying Object.
func (fh *ReadFileHandle) Name() string {
	return fh.file.String()
}

// Size returns the size of the underlying file
func (fh *ReadFileHandle) Size() int64 {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.size
}

// Stat returns info about the file
func (fh *ReadFileHandle) Stat() (os.FileInfo, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	return fh.file, nil
}
