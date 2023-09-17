// Copyright 2020 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ratelimit

import "io"

type reader struct {
	r io.Reader
	l *Limiter
}

// Read Read
func (r *reader) Read(buf []byte) (int, error) {
	n, err := r.r.Read(buf)
	r.l.Wait(n)
	return n, err
}

// Reader returns a reader with limiter
func Reader(r io.Reader, l *Limiter) io.Reader {
	return &reader{
		r: r,
		l: l,
	}
}
