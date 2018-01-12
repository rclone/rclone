package readers

import "io"

// NewCountingReader returns a CountingReader, which will read from the given
// reader while keeping track of how many bytes were read.
func NewCountingReader(in io.Reader) *CountingReader {
	return &CountingReader{in: in}
}

// CountingReader holds a reader and a read count of how many bytes were read
// so far.
type CountingReader struct {
	in   io.Reader
	read uint64
}

// Read reads from the underlying reader.
func (cr *CountingReader) Read(b []byte) (int, error) {
	n, err := cr.in.Read(b)
	cr.read += uint64(n)
	return n, err
}

// BytesRead returns how many bytes were read from the underlying reader so far.
func (cr *CountingReader) BytesRead() uint64 {
	return cr.read
}
