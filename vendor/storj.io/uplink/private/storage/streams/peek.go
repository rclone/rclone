// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"errors"
	"io"

	"github.com/zeebo/errs"
)

// PeekThresholdReader allows a check to see if the size of a given reader
// exceeds the maximum inline segment size or not.
type PeekThresholdReader struct {
	r              io.Reader
	thresholdBuf   []byte
	thresholdErr   error
	isLargerCalled bool
	readCalled     bool
}

// NewPeekThresholdReader creates a new instance of PeekThresholdReader.
func NewPeekThresholdReader(r io.Reader) (pt *PeekThresholdReader) {
	return &PeekThresholdReader{r: r}
}

// Read initially reads bytes from the internal buffer, then continues
// reading from the wrapped data reader. The number of bytes read `n`
// is returned.
func (pt *PeekThresholdReader) Read(p []byte) (n int, err error) {
	pt.readCalled = true

	if len(pt.thresholdBuf) > 0 || pt.thresholdErr != nil {
		n = copy(p, pt.thresholdBuf)
		pt.thresholdBuf = pt.thresholdBuf[n:]
		if len(pt.thresholdBuf) == 0 {
			err := pt.thresholdErr
			pt.thresholdErr = nil
			return n, err
		}
		return n, nil
	}

	return pt.r.Read(p)
}

// IsLargerThan returns a bool to determine whether a reader's size
// is larger than the given threshold or not.
func (pt *PeekThresholdReader) IsLargerThan(thresholdSize int) (bool, error) {
	if pt.isLargerCalled {
		return false, errs.New("IsLargerThan can't be called more than once")
	}
	if pt.readCalled {
		return false, errs.New("IsLargerThan can't be called after Read has been called")
	}
	pt.isLargerCalled = true
	buf := make([]byte, thresholdSize+1)
	n, err := io.ReadFull(pt.r, buf)
	pt.thresholdBuf = buf[:n]
	pt.thresholdErr = err
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			pt.thresholdErr = io.EOF
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
