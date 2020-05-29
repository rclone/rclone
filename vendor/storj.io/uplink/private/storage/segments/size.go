// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import (
	"io"
)

// SizedReader allows to check the total number of bytes read so far.
type SizedReader struct {
	r    io.Reader
	size int64
}

// SizeReader create a new instance of SizedReader.
func SizeReader(r io.Reader) *SizedReader {
	return &SizedReader{r: r}
}

// Read implements io.Reader.Read.
func (r *SizedReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.size += int64(n)
	return n, err
}

// Size returns the total number of bytes read so far.
func (r *SizedReader) Size() int64 { return r.size }
