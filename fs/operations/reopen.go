package operations

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
)

// ReOpen is a wrapper for an object reader which reopens the stream on error
type ReOpen struct {
	ctx      context.Context
	mu       sync.Mutex      // mutex to protect the below
	src      fs.Object       // object to open
	options  []fs.OpenOption // option to pass to initial open
	rc       io.ReadCloser   // underlying stream
	read     int64           // number of bytes read from this stream
	maxTries int             // maximum number of retries
	tries    int             // number of retries we've had so far in this stream
	err      error           // if this is set then Read/Close calls will return it
	opened   bool            // if set then rc is valid and needs closing
}

var (
	errorFileClosed   = errors.New("file already closed")
	errorTooManyTries = errors.New("failed to reopen: too many retries")
)

// NewReOpen makes a handle which will reopen itself and seek to where it was on errors
//
// If hashOption is set this will be applied when reading from the start
//
// If rangeOption is set then this will applied when reading from the
// start, and updated on retries.
func NewReOpen(ctx context.Context, src fs.Object, maxTries int, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	h := &ReOpen{
		ctx:      ctx,
		src:      src,
		maxTries: maxTries,
		options:  options,
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
func (h *ReOpen) open() error {
	opts := []fs.OpenOption{}
	var hashOption *fs.HashesOption
	var rangeOption *fs.RangeOption
	for _, option := range h.options {
		switch option.(type) {
		case *fs.HashesOption:
			hashOption = option.(*fs.HashesOption)
		case *fs.RangeOption:
			rangeOption = option.(*fs.RangeOption)
		case *fs.HTTPOption:
			opts = append(opts, option)
		default:
			if option.Mandatory() {
				fs.Logf(h.src, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if h.read == 0 {
		if rangeOption != nil {
			opts = append(opts, rangeOption)
		}
		if hashOption != nil {
			// put hashOption on if reading from the start, ditch otherwise
			opts = append(opts, hashOption)
		}
	} else {
		if rangeOption != nil {
			// range to the read point
			opts = append(opts, &fs.RangeOption{Start: rangeOption.Start + h.read, End: rangeOption.End})
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
func (h *ReOpen) Read(p []byte) (n int, err error) {
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
func (h *ReOpen) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.opened {
		return errorFileClosed
	}
	h.opened = false
	h.err = errorFileClosed
	return h.rc.Close()
}
