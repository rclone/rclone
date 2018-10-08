package readers

import "io"

// NewPatternReader creates a reader, that returns a deterministic byte pattern.
// After length bytes are read
func NewPatternReader(length int64) io.Reader {
	return &patternReader{
		length: length,
	}
}

type patternReader struct {
	length int64
	c      byte
}

func (r *patternReader) Read(p []byte) (n int, err error) {
	for i := range p {
		if r.length <= 0 {
			return n, io.EOF
		}
		p[i] = r.c
		r.c = (r.c + 1) % 253
		r.length--
		n++
	}
	return
}
