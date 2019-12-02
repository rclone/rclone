package operations

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
)

// reOpen is a wrapper for an object reader which reopens the stream on error
type reOpen struct {
	ctx         context.Context
	mu          sync.Mutex       // mutex to protect the below
	src         fs.Object        // object to open
	hashOption  *fs.HashesOption // option to pass to initial open
	rangeOption *fs.RangeOption  // option to pass to initial open
	rc          io.ReadCloser    // underlying stream
	read        int64            // number of bytes read from this stream
	maxTries    int              // maximum number of retries
	tries       int              // number of retries we've had so far in this stream
	err         error            // if this is set then Read/Close calls will return it
	opened      bool             // if set then rc is valid and needs closing
}

var (
	errorFileClosed   = errors.New("file already closed")
	errorTooManyTries = errors.New("failed to reopen: too many retries")
)

// newReOpen makes a handle which will reopen itself and seek to where it was on errors
//
// If hashOption is set this will be applied when reading from the start
//
// If rangeOption is set then this will applied when reading from the
// start, and updated on retries.
func newReOpen(ctx context.Context, src fs.Object, hashOption *fs.HashesOption, rangeOption *fs.RangeOption, maxTries int) (rc io.ReadCloser, err error) {
	h := &reOpen{
		ctx:         ctx,
		src:         src,
		hashOption:  hashOption,
		rangeOption: rangeOption,
		maxTries:    maxTries,
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	err = h.open()
	if err != nil {
		return nil, err
	}
	return h, nil
}

// open the underlying handle - call with lock held
//
// we don't retry here as the Open() call will itself have low level retries
func (h *reOpen) open() error {
	var optsArray [2]fs.OpenOption
	var opts = optsArray[:0]
	if h.read == 0 {
		if h.rangeOption != nil {
			opts = append(opts, h.rangeOption)
		}
		if h.hashOption != nil {
			// put hashOption on if reading from the start, ditch otherwise
			opts = append(opts, h.hashOption)
		}
	} else {
		if h.rangeOption != nil {
			// range to the read point
			opts = append(opts, &fs.RangeOption{Start: h.rangeOption.Start + h.read, End: h.rangeOption.End})
		} else {
			// seek to the read point
			opts = append(opts, &fs.SeekOption{Offset: h.read})
		}
	}
	h.tries++
	if h.tries > h.maxTries {
		h.err = errorTooManyTries
	} else {
		h.rc, h.err = h.src.Open(h.ctx, opts...)
	}
	if h.err != nil {
		if h.tries > 1 {
			fs.Debugf(h.src, "Reopen failed after %d bytes read: %v", h.read, h.err)
		}
		return h.err
	}
	h.opened = true
	return nil
}

// Read bytes retrying as necessary
func (h *reOpen) Read(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.err != nil {
		// return a previous error if there is one
		return n, h.err
	}
	n, err = h.rc.Read(p)
	if err != nil {
		h.err = err
	}
	h.read += int64(n)
	if err != nil && err != io.EOF && !fserrors.IsNoLowLevelRetryError(err) {
		// close underlying stream
		h.opened = false
		_ = h.rc.Close()
		// reopen stream, clearing error if successful
		fs.Debugf(h.src, "Reopening on read failure after %d bytes: retry %d/%d: %v", h.read, h.tries, h.maxTries, err)
		if h.open() == nil {
			err = nil
		}
	}
	return n, err
}

// Close the stream
func (h *reOpen) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.opened {
		return errorFileClosed
	}
	h.opened = false
	h.err = errorFileClosed
	return h.rc.Close()
}
