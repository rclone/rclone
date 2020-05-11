// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import "io"

// SizeReader holds reader and size read so far
type SizeReader struct {
	reader io.Reader
	size   int64
}

// NewSizeReader keeps track of how much bytes are read from the reader
func NewSizeReader(r io.Reader) *SizeReader {
	return &SizeReader{reader: r}
}

func (r *SizeReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.size += int64(n)
	return n, err
}

// Size returns the number of bytes read so far
func (r *SizeReader) Size() int64 {
	return r.size
}
