// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package buffer

import (
	"sync"
	"sync/atomic"

	"github.com/zeebo/errs"
)

// Cursor keeps track of how many bytes have been written and the furthest advanced
// reader, letting one wait until space or bytes are available.
type Cursor struct {
	writeAhead int64

	mu   sync.Mutex
	cond sync.Cond

	doneReading uint32
	doneWriting uint32

	readErr  error
	writeErr error

	maxRead int64
	written int64
}

// NewCursor constructs a new cursor that keeps track of reads and writes
// into some buffer, allowing one to wait until enough data has been read or written.
func NewCursor(writeAhead int64) *Cursor {
	c := &Cursor{writeAhead: writeAhead}
	c.cond.L = &c.mu
	return c
}

// WaitRead blocks until the writer is done or until at least n bytes have been written.
// It returns min(n, w.written) letting the caller know the largest offset that can be read.
// The ok boolean is true if there are more bytes to be read. If writing is done with an
// error, then 0 and that error are returned. If writing is done with no error and the requested
// amount is at least the amount written, it returns the written amount, false, and nil.
func (c *Cursor) WaitRead(n int64) (m int64, ok bool, err error) {
	if atomic.LoadUint32(&c.doneReading) != 0 {
		return 0, false, errs.New("WaitRead called after DoneReading")
	}
	if written := atomic.LoadInt64(&c.written); n < written {
		return n, true, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadUint32(&c.doneReading) != 0 {
		return 0, false, errs.New("WaitRead called after DoneReading")
	}

	for {
		doneWriting := atomic.LoadUint32(&c.doneWriting) != 0
		maxRead := atomic.LoadInt64(&c.maxRead)
		written := atomic.LoadInt64(&c.written)

		switch {
		// first, return any write error if there is one.
		case c.writeErr != nil:
			return 0, false, c.writeErr

		// next, return io.EOF when fully read.
		case n >= written && doneWriting:
			return written, false, nil

		// next, allow reading up to the written amount.
		case n <= written:
			return n, true, nil

		// next, if maxRead is not yet caught up to written, allow reads to proceed up to written.
		case maxRead < written:
			return written, true, nil

		// finally, if more is requested, allow at most the written amount.
		case doneWriting:
			return written, true, nil
		}

		c.cond.Wait()
	}
}

// WaitWrite blocks until the readers are done or until the furthest advanced reader is
// within the writeAhead of the writer. It returns the largest offset that can be written.
// The ok boolean is true if there are readers waiting for more bytes. If reading is done
// with an error, then 0 and that error are returned. If reading is done with no error, then
// it returns the amount written, false, and nil.
func (c *Cursor) WaitWrite(n int64) (m int64, ok bool, err error) {
	if atomic.LoadUint32(&c.doneWriting) != 0 {
		return 0, false, errs.New("WaitWrite called after DoneWriting")
	}
	if maxRead := atomic.LoadInt64(&c.maxRead); n <= maxRead+c.writeAhead {
		return n, true, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadUint32(&c.doneWriting) != 0 {
		return 0, false, errs.New("WaitWrite called after DoneWriting")
	}

	for {
		doneReading := atomic.LoadUint32(&c.doneReading) != 0
		maxRead := atomic.LoadInt64(&c.maxRead)
		written := atomic.LoadInt64(&c.written)

		switch {
		// first, return any read error if there is one.
		case c.readErr != nil:
			return 0, false, c.readErr

		// next, don't allow more writes if the reader is done.
		case doneReading:
			return written, false, nil

		// next, allow when enough behind the furthest advanced reader.
		case n <= maxRead+c.writeAhead:
			return n, true, nil

		// finally, only allow up to a maximum amount ahead of the furthest reader.
		case written < maxRead+c.writeAhead:
			return maxRead + c.writeAhead, true, nil
		}

		c.cond.Wait()
	}
}

// DoneWriting signals that no more Write calls will happen. It returns true
// the first time DoneWriting and DoneReading have both been called.
func (c *Cursor) DoneWriting(err error) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadUint32(&c.doneWriting) == 0 {
		atomic.StoreUint32(&c.doneWriting, 1)
		c.writeErr = err
		c.cond.Broadcast()

		return atomic.LoadUint32(&c.doneReading) != 0
	}

	return false
}

// DoneReading signals that no more Read calls will happen. It returns true
// the first time DoneWriting and DoneReading have both been called.
func (c *Cursor) DoneReading(err error) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadUint32(&c.doneReading) == 0 {
		atomic.StoreUint32(&c.doneReading, 1)
		c.readErr = err
		c.cond.Broadcast()

		return atomic.LoadUint32(&c.doneWriting) != 0
	}

	return false
}

// ReadTo reports to the cursor that some reader read up to byte offset n.
func (c *Cursor) ReadTo(n int64) {
	for {
		maxRead := atomic.LoadInt64(&c.maxRead)
		if n <= maxRead {
			return
		}
		if atomic.CompareAndSwapInt64(&c.maxRead, maxRead, n) {
			c.mu.Lock()
			defer c.mu.Unlock()

			c.cond.Broadcast()
			return
		}
	}
}

// WroteTo reports to the cursor that the writer wrote up to byte offset n.
func (c *Cursor) WroteTo(n int64) {
	for {
		written := atomic.LoadInt64(&c.written)
		if n <= written {
			return
		}
		if atomic.CompareAndSwapInt64(&c.written, written, n) {
			c.mu.Lock()
			defer c.mu.Unlock()

			c.cond.Broadcast()
			return
		}
	}
}
