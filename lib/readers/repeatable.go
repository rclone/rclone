package readers

import (
	"io"
	"sync"

	"github.com/pkg/errors"
)

// A RepeatableReader implements the io.ReadSeeker it allow to seek cached data
// back and forth within the reader but will only read data from the internal Reader as necessary
// and will play nicely with the Account and io.LimitedReader to reflect current speed
type RepeatableReader struct {
	mu sync.Mutex // protect against concurrent use
	in io.Reader  // Input reader
	i  int64      // current reading index
	b  []byte     // internal cache buffer
}

var _ io.ReadSeeker = (*RepeatableReader)(nil)

// Seek implements the io.Seeker interface.
// If seek position is passed the cache buffer length the function will return
// the maximum offset that can be used and "fs.RepeatableReader.Seek: offset is unavailable" Error
func (r *RepeatableReader) Seek(offset int64, whence int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var abs int64
	cacheLen := int64(len(r.b))
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.i + offset
	case io.SeekEnd:
		abs = cacheLen + offset
	default:
		return 0, errors.New("fs.RepeatableReader.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("fs.RepeatableReader.Seek: negative position")
	}
	if abs > cacheLen {
		return offset - (abs - cacheLen), errors.New("fs.RepeatableReader.Seek: offset is unavailable")
	}
	r.i = abs
	return abs, nil
}

// Read data from original Reader into bytes
// Data is either served from the underlying Reader or from cache if was already read
func (r *RepeatableReader) Read(b []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cacheLen := int64(len(r.b))
	if r.i == cacheLen {
		n, err = r.in.Read(b)
		if n > 0 {
			r.b = append(r.b, b[:n]...)
		}
	} else {
		n = copy(b, r.b[r.i:])
	}
	r.i += int64(n)
	return n, err
}

// NewRepeatableReader create new repeatable reader from Reader r
func NewRepeatableReader(r io.Reader) *RepeatableReader {
	return &RepeatableReader{in: r}
}

// NewRepeatableReaderSized create new repeatable reader from Reader r
// with an initial buffer of size.
func NewRepeatableReaderSized(r io.Reader, size int) *RepeatableReader {
	return &RepeatableReader{
		in: r,
		b:  make([]byte, 0, size),
	}
}

// NewRepeatableLimitReader create new repeatable reader from Reader r
// with an initial buffer of size wrapped in a io.LimitReader to read
// only size.
func NewRepeatableLimitReader(r io.Reader, size int) *RepeatableReader {
	return NewRepeatableReaderSized(io.LimitReader(r, int64(size)), size)
}

// NewRepeatableReaderBuffer create new repeatable reader from Reader r
// using the buffer passed in.
func NewRepeatableReaderBuffer(r io.Reader, buf []byte) *RepeatableReader {
	return &RepeatableReader{
		in: r,
		b:  buf[:0],
	}
}

// NewRepeatableLimitReaderBuffer create new repeatable reader from
// Reader r and buf wrapped in a io.LimitReader to read only size.
func NewRepeatableLimitReaderBuffer(r io.Reader, buf []byte, size int64) *RepeatableReader {
	return NewRepeatableReaderBuffer(io.LimitReader(r, size), buf)
}
