// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package readcloser

import (
	"errors"
	"io"

	"github.com/zeebo/errs"
)

type eofReadCloser struct{}

func (eofReadCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (eofReadCloser) Close() error {
	return nil
}

type multiReadCloser struct {
	readers []io.ReadCloser
}

// MultiReadCloser is a MultiReader extension that returns a ReaderCloser
// that's the logical concatenation of the provided input readers.
// They're read sequentially. Once all inputs have returned EOF,
// Read will return EOF.  If any of the readers return a non-nil,
// non-EOF error, Read will return that error.
func MultiReadCloser(readers ...io.ReadCloser) io.ReadCloser {
	r := make([]io.ReadCloser, len(readers))
	copy(r, readers)
	return &multiReadCloser{r}
}

func (mr *multiReadCloser) Read(p []byte) (n int, err error) {
	for len(mr.readers) > 0 {
		// Optimization to flatten nested multiReaders.
		if len(mr.readers) == 1 {
			if r, ok := mr.readers[0].(*multiReadCloser); ok {
				mr.readers = r.readers
				continue
			}
		}
		n, err = mr.readers[0].Read(p)
		if errors.Is(err, io.EOF) {
			err = mr.readers[0].Close()
			// Use eofReader instead of nil to avoid nil panic
			// after performing flatten (Issue 18232).
			mr.readers[0] = eofReadCloser{} // permit earlier GC
			mr.readers = mr.readers[1:]
		}
		if n > 0 || !errors.Is(err, io.EOF) {
			if errors.Is(err, io.EOF) && len(mr.readers) > 0 {
				// Don't return EOF yet. More readers remain.
				err = nil
			}
			return
		}
	}
	return 0, io.EOF
}

func (mr *multiReadCloser) Close() error {
	errlist := make([]error, len(mr.readers))
	for i, r := range mr.readers {
		errlist[i] = r.Close()
	}
	return errs.Combine(errlist...)
}
