// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"errors"
	"io"
)

// EOFReader holds reader and status of EOF.
type EOFReader struct {
	reader io.Reader
	eof    bool
	err    error
}

// NewEOFReader keeps track of the state, has the internal reader reached EOF.
func NewEOFReader(r io.Reader) *EOFReader {
	return &EOFReader{reader: r}
}

func (r *EOFReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if errors.Is(err, io.EOF) {
		r.eof = true
	} else if err != nil && r.err == nil {
		r.err = err
	}
	return n, err
}

// IsEOF returns true if EOF was returned during reading.
func (r *EOFReader) IsEOF() bool {
	return r.eof
}

// HasError returns true if error was returned during reading.
func (r *EOFReader) HasError() bool {
	return r.err != nil
}
