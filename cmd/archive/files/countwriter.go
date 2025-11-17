package files

import (
	"io"
	"sync/atomic"
)

// CountWriter counts bytes written through it.
// It is safe for concurrent Count/Reset; Write is as safe as the wrapped Writer.
type CountWriter struct {
	w     io.Writer
	count atomic.Uint64
}

// NewCountWriter wraps w (use nil if you want to drop data).
func NewCountWriter(w io.Writer) *CountWriter {
	if w == nil {
		w = io.Discard
	}
	return &CountWriter{w: w}
}

func (cw *CountWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if n > 0 {
		cw.count.Add(uint64(n))
	}
	return n, err
}

// Count returns the total bytes written.
func (cw *CountWriter) Count() uint64 {
	return cw.count.Load()
}
