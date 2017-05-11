package io

import (
	"github.com/tsenart/tb"
	"io"
	"time"
)

// NewThrottledWriter is an io.Writer wrapping another io.Writer with
// byte rate throttling, flushing block bytes at a time.
func NewThrottledWriter(rate, block int64, w io.Writer) io.Writer {
	return &throttledWriter{rate, block, w, tb.NewBucket(rate, -1)}
}

type throttledWriter struct {
	rate, block int64
	w           io.Writer
	b           *tb.Bucket
}

func (tw *throttledWriter) Write(p []byte) (n int, err error) {
	for wr := 0; wr < len(p); {
		var got int64
		for got < tw.block {
			if got += tw.b.Take(tw.block - got); got != tw.block {
				time.Sleep(time.Duration((1e9 / tw.rate) * (tw.block - got)))
			}
		}
		if n, err = tw.w.Write(p[wr : wr+int(got)]); err != nil {
			return wr, err
		}
		wr += n
	}
	return len(p), nil
}
