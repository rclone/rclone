package operations

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
)

// AccountFn is a function which will be called after every read
// from the ReOpen.
//
// It may return an error which will be passed back to the user.
type AccountFn func(n int) error

// ReOpen is a wrapper for an object reader which reopens the stream on error
type ReOpen struct {
	ctx         context.Context
	mu          sync.Mutex      // mutex to protect the below
	src         fs.Object       // object to open
	baseOptions []fs.OpenOption // options to pass to initial open and where offset == 0
	options     []fs.OpenOption // option to pass on subsequent opens where offset != 0
	rangeOption fs.RangeOption  // adjust this range option on re-opens
	rc          io.ReadCloser   // underlying stream
	size        int64           // total size of object - can be -ve
	start       int64           // absolute position to start reading from
	end         int64           // absolute position to end reading (exclusive)
	offset      int64           // offset in the file we are at, offset from start
	newOffset   int64           // if different to offset, reopen needed
	maxTries    int             // maximum number of retries
	tries       int             // number of retries we've had so far in this stream
	err         error           // if this is set then Read/Close calls will return it
	opened      bool            // if set then rc is valid and needs closing
	account     AccountFn       // account for a read
	reads       int             // count how many times the data has been read
	accountOn   int             // only account on or after this read
}

var (
	errFileClosed    = errors.New("file already closed")
	errTooManyTries  = errors.New("failed to reopen: too many retries")
	errInvalidWhence = errors.New("reopen Seek: invalid whence")
	errNegativeSeek  = errors.New("reopen Seek: negative position")
	errSeekPastEnd   = errors.New("reopen Seek: attempt to seek past end of data")
	errBadEndSeek    = errors.New("reopen Seek: can't seek from end with unknown sized object")
)

// NewReOpen makes a handle which will reopen itself and seek to where
// it was on errors up to maxTries times.
//
// If an fs.HashesOption is set this will be applied when reading from
// the start.
//
// If an fs.RangeOption is set then this will applied when reading from
// the start, and updated on retries.
func NewReOpen(ctx context.Context, src fs.Object, maxTries int, options ...fs.OpenOption) (rc *ReOpen, err error) {
	h := &ReOpen{
		ctx:         ctx,
		src:         src,
		maxTries:    maxTries,
		baseOptions: options,
		size:        src.Size(),
		start:       0,
		offset:      0,
		newOffset:   -1, // -1 means no seek required
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	// Filter the options for subsequent opens
	h.options = make([]fs.OpenOption, 0, len(options)+1)
	var limit int64 = -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.HashesOption:
			// leave hash option out when ranging
		case *fs.RangeOption:
			h.start, limit = x.Decode(h.end)
		case *fs.SeekOption:
			h.start, limit = x.Offset, -1
		default:
			h.options = append(h.options, option)
		}
	}

	// Put our RangeOption on the end
	h.rangeOption.Start = h.start
	h.options = append(h.options, &h.rangeOption)

	// If a size range is set then set the end point of the file to that
	if limit >= 0 && h.size >= 0 {
		h.end = h.start + limit
		h.rangeOption.End = h.end - 1 // remember range options are inclusive
	} else {
		h.end = h.size
		h.rangeOption.End = -1
	}

	err = h.open()
	if err != nil {
		return nil, err
	}
	return h, nil
}

// Open makes a handle which will reopen itself and seek to where it
// was on errors.
//
// If an fs.HashesOption is set this will be applied when reading from
// the start.
//
// If an fs.RangeOption is set then this will applied when reading from
// the start, and updated on retries.
//
// It will obey LowLevelRetries in the ctx as the maximum number of
// tries.
//
// Use this instead of calling the Open method on fs.Objects
func Open(ctx context.Context, src fs.Object, options ...fs.OpenOption) (rc *ReOpen, err error) {
	maxTries := fs.GetConfig(ctx).LowLevelRetries
	return NewReOpen(ctx, src, maxTries, options...)
}

// open the underlying handle - call with lock held
//
// we don't retry here as the Open() call will itself have low level retries
func (h *ReOpen) open() error {
	var opts []fs.OpenOption
	if h.offset == 0 {
		// if reading from the start using the initial options
		opts = h.baseOptions
	} else {
		// otherwise use the filtered options
		opts = h.options
		// Adjust range start to where we have got to
		h.rangeOption.Start = h.start + h.offset
	}
	// Make a copy of the options as fs.FixRangeOption modifies them :-(
	opts = append(make([]fs.OpenOption, 0, len(opts)), opts...)
	h.tries++
	if h.tries > h.maxTries {
		h.err = errTooManyTries
	} else {
		h.rc, h.err = h.src.Open(h.ctx, opts...)
	}
	if h.err != nil {
		if h.tries > 1 {
			fs.Debugf(h.src, "Reopen failed after offset %d bytes read: %v", h.offset, h.err)
		}
		return h.err
	}
	h.opened = true
	return nil
}

// reopen the underlying handle by closing it and reopening it.
func (h *ReOpen) reopen() (err error) {
	// close underlying stream if needed
	if h.opened {
		h.opened = false
		_ = h.rc.Close()
	}
	return h.open()
}

// account for n bytes being read
func (h *ReOpen) accountRead(n int) error {
	if h.account == nil {
		return nil
	}
	// Don't start accounting until we've reached this many reads
	//
	// rw.reads will be 1 the first time this is called
	// rw.accountOn 2 means start accounting on the 2nd read through
	if h.reads >= h.accountOn {
		return h.account(n)
	}
	return nil
}

// Read bytes retrying as necessary
func (h *ReOpen) Read(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		// return a previous error if there is one
		return n, h.err
	}

	// re-open if seek needed
	if h.newOffset >= 0 {
		if h.offset != h.newOffset {
			fs.Debugf(h.src, "Seek from %d to %d", h.offset, h.newOffset)
			h.offset = h.newOffset
			err = h.reopen()
			if err != nil {
				return 0, err
			}
		}
		h.newOffset = -1
	}

	// Read a full buffer
	startOffset := h.offset
	var nn int
	for n < len(p) && err == nil {
		nn, err = h.rc.Read(p[n:])
		n += nn
		h.offset += int64(nn)
		if err != nil && err != io.EOF {
			h.err = err
			if !fserrors.IsNoLowLevelRetryError(err) {
				fs.Debugf(h.src, "Reopening on read failure after offset %d bytes: retry %d/%d: %v", h.offset, h.tries, h.maxTries, err)
				if h.reopen() == nil {
					err = nil
				}
			}
		}
	}
	// Count a read of the data if we read from the start successfully
	if startOffset == 0 && n != 0 {
		h.reads++
	}
	// Account the read
	accErr := h.accountRead(n)
	if err == nil {
		err = accErr
	}
	return n, err
}

// Seek sets the offset for the next Read or Write to offset, interpreted
// according to whence: SeekStart means relative to the start of the file,
// SeekCurrent means relative to the current offset, and SeekEnd means relative
// to the end (for example, offset = -2 specifies the penultimate byte of the
// file). Seek returns the new offset relative to the start of the file or an
// error, if any.
//
// Seeking to an offset before the start of the file is an error. Seeking
// to any positive offset may be allowed, but if the new offset exceeds the
// size of the underlying object the behavior of subsequent I/O operations is
// implementation-dependent.
func (h *ReOpen) Seek(offset int64, whence int) (int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		// return a previous error if there is one
		return 0, h.err
	}
	var abs int64
	var size = h.end - h.start
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		if h.newOffset >= 0 {
			abs = h.newOffset + offset
		} else {
			abs = h.offset + offset
		}
	case io.SeekEnd:
		if h.size < 0 {
			return 0, errBadEndSeek
		}
		abs = size + offset
	default:
		return 0, errInvalidWhence
	}
	if abs < 0 {
		return 0, errNegativeSeek
	}
	if h.size >= 0 && abs > size {
		return size, errSeekPastEnd
	}

	h.tries = 0       // Reset open count on seek
	h.newOffset = abs // New offset - applied in Read
	return abs, nil
}

// Close the stream
func (h *ReOpen) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.opened {
		return errFileClosed
	}
	h.opened = false
	h.err = errFileClosed
	return h.rc.Close()
}

// SetAccounting should be provided with a function which will be
// called after every read from the RW.
//
// It may return an error which will be passed back to the user.
func (h *ReOpen) SetAccounting(account AccountFn) *ReOpen {
	h.account = account
	return h
}

// DelayAccounting makes sure the accounting function only gets called
// on the i-th or later read of the data from this point (counting
// from 1).
//
// This is useful so that we don't account initial reads of the data
// e.g. when calculating hashes.
//
// Set this to 0 to account everything.
func (h *ReOpen) DelayAccounting(i int) {
	h.accountOn = i
	h.reads = 0
}
