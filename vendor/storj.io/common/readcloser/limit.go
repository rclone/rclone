// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package readcloser

import "io"

// LimitReadCloser is a LimitReader extension that returns a ReadCloser
// that reads from r but stops with EOF after n bytes.
func LimitReadCloser(r io.ReadCloser, n int64) io.ReadCloser {
	return &limitedReadCloser{io.LimitReader(r, n), r}
}

type limitedReadCloser struct {
	R io.Reader
	C io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (n int, err error) {
	return l.R.Read(p)
}

func (l *limitedReadCloser) Close() error {
	return l.C.Close()
}
