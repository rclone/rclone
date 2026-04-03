package readers

import (
	"io"

	"github.com/rclone/rclone/fs"
)

// LimitedReadCloser adds io.Closer to io.LimitedReader.  Create one with NewLimitedReadCloser
type LimitedReadCloser struct {
	*io.LimitedReader
	io.Closer
}

// Close closes the underlying io.Closer. The error, if any, will be ignored if data is read completely
func (lrc *LimitedReadCloser) Close() error {
	err := lrc.Closer.Close()
	if err != nil && lrc.N == 0 {
		fs.Debugf(nil, "ignoring close error because we already got all the data")
		err = nil
	}
	return err
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
