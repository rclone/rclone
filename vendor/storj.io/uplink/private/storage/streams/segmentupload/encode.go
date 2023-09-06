// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package segmentupload

import (
	"errors"
	"io"

	"github.com/zeebo/errs"

	"storj.io/uplink/private/eestream"
)

// EncodedReader provides a redundant piece for given reader.
type EncodedReader struct {
	r         io.Reader
	rs        eestream.RedundancyStrategy
	num       int
	stripeBuf []byte
	shareBuf  []byte
	available int
	err       error
}

// NewEncodedReader provides a reader that returns a redundant piece for the
// reader using the given redundancy strategy and piece number.
func NewEncodedReader(r io.Reader, rs eestream.RedundancyStrategy, num int) *EncodedReader {
	return &EncodedReader{
		r:         r,
		rs:        rs,
		num:       num,
		stripeBuf: make([]byte, rs.StripeSize()),
		shareBuf:  make([]byte, rs.ErasureShareSize()),
	}
}

// Read reads the redundant piece data.
func (er *EncodedReader) Read(p []byte) (n int, err error) {
	// No need to trace this function because it's very fast and called many times.
	if er.err != nil {
		return 0, er.err
	}

	for len(p) > 0 {
		if er.available == 0 {
			// take the next stripe from the segment buffer
			_, err := io.ReadFull(er.r, er.stripeBuf)
			if errors.Is(err, io.EOF) {
				er.err = io.EOF
				break
			} else if err != nil {
				er.err = errs.Wrap(err)
				return 0, er.err
			}

			// encode the num-th erasure share
			err = er.rs.EncodeSingle(er.stripeBuf, er.shareBuf, er.num)
			if err != nil {
				er.err = err
				return 0, err
			}

			er.available = len(er.shareBuf)
		}

		off := len(er.shareBuf) - er.available
		nc := copy(p, er.shareBuf[off:])
		p = p[nc:]
		er.available -= nc
		n += nc
	}

	return n, er.err
}
