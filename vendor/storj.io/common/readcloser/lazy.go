// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package readcloser

import "io"

// LazyReadCloser returns an ReadCloser that doesn't initialize the backing
// Reader until the first Read.
func LazyReadCloser(reader func() (io.ReadCloser, error)) io.ReadCloser {
	return &lazyReadCloser{fn: reader}
}

type lazyReadCloser struct {
	fn func() (io.ReadCloser, error)
	r  io.ReadCloser
}

func (l *lazyReadCloser) Read(p []byte) (n int, err error) {
	if l.r == nil {
		l.r, err = l.fn()
		if err != nil {
			return 0, err
		}
		l.fn = nil
	}
	return l.r.Read(p)
}

func (l *lazyReadCloser) Close() error {
	if l.r != nil {
		return l.r.Close()
	}
	return nil
}
