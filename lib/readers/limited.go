package readers

import "io"

// LimitedReadCloser adds io.Closer to io.LimitedReader.  Create one with NewLimitedReadCloser
type LimitedReadCloser struct {
	*io.LimitedReader
	io.Closer
}

// NewLimitedReadCloser returns a LimitedReadCloser wrapping rc to
// limit it to reading limit bytes. If limit < 0 then it does not
// wrap rc, it just returns it.
func NewLimitedReadCloser(rc io.ReadCloser, limit int64) (lrc io.ReadCloser) {
	if limit < 0 {
		return rc
	}
	return &LimitedReadCloser{
		LimitedReader: &io.LimitedReader{R: rc, N: limit},
		Closer:        rc,
	}
}
