package readers

import (
	"errors"
	"io"
)

var (
	errCantSeek = errors.New("can't Seek")
)

// NoSeeker adapts an io.Reader into an io.ReadSeeker.
//
// However if Seek() is called it will return an error.
type NoSeeker struct {
	io.Reader
}

// Seek the stream - returns an error
func (r NoSeeker) Seek(offset int64, whence int) (abs int64, err error) {
	return 0, errCantSeek
}
