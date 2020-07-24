package readers

import (
	"io"

	"github.com/pkg/errors"
)

// This is the smallest prime less than 256
//
// Using a prime here means we are less likely to hit repeating patterns
const patternReaderModulo = 251

// NewPatternReader creates a reader, that returns a deterministic byte pattern.
// After length bytes are read
func NewPatternReader(length int64) io.ReadSeeker {
	return &patternReader{
		length: length,
	}
}

type patternReader struct {
	offset int64
	length int64
	c      byte
}

func (r *patternReader) Read(p []byte) (n int, err error) {
	for i := range p {
		if r.offset >= r.length {
			return n, io.EOF
		}
		p[i] = r.c
		r.c = (r.c + 1) % patternReaderModulo
		r.offset++
		n++
	}
	return
}

// Seek implements the io.Seeker interface.
func (r *patternReader) Seek(offset int64, whence int) (abs int64, err error) {
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.offset + offset
	case io.SeekEnd:
		abs = r.length + offset
	default:
		return 0, errors.New("patternReader: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("patternReader: negative position")
	}
	r.offset = abs
	r.c = byte(abs % patternReaderModulo)
	return abs, nil
}
