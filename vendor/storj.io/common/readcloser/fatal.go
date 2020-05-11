// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package readcloser

import "io"

// FatalReadCloser returns a ReadCloser that always fails with err.
func FatalReadCloser(err error) io.ReadCloser {
	return &fatalReadCloser{Err: err}
}

type fatalReadCloser struct {
	Err error
}

func (f *fatalReadCloser) Read(p []byte) (n int, err error) {
	return 0, f.Err
}

func (f *fatalReadCloser) Close() error {
	return nil
}
