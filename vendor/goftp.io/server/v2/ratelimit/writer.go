// Copyright 2020 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ratelimit

import "io"

type writer struct {
	w io.Writer
	l *Limiter
}

// Write Write
func (w *writer) Write(buf []byte) (int, error) {
	w.l.Wait(len(buf))
	return w.w.Write(buf)
}

// Writer returns a writer with limiter
func Writer(w io.Writer, l *Limiter) io.Writer {
	return &writer{
		w: w,
		l: l,
	}
}
