package readers

import (
	"compress/gzip"
	"io"
)

// gzipReader wraps a *gzip.Reader so it closes the underlying stream
// which the gzip library doesn't.
type gzipReader struct {
	*gzip.Reader
	in io.ReadCloser
}

// NewGzipReader returns an io.ReadCloser which will read the stream
// and close it when Close is called.
//
// Unfortunately gz.Reader does not close the underlying stream so we
// can't use that directly.
func NewGzipReader(in io.ReadCloser) (io.ReadCloser, error) {
	zr, err := gzip.NewReader(in)
	if err != nil {
		return nil, err
	}
	return &gzipReader{
		Reader: zr,
		in:     in,
	}, nil
}

// Close the underlying stream and the gzip reader
func (gz *gzipReader) Close() error {
	zrErr := gz.Reader.Close()
	inErr := gz.in.Close()
	if inErr != nil {
		return inErr
	}
	if zrErr != nil {
		return zrErr
	}
	return nil
}
