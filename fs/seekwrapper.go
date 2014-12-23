package fs

import (
	"io"
	"os"
)

// SeekWrapper wraps an io.Reader with a basic Seek method which
// returns the Size attribute.
//
// This is used for google.golang.org/api/googleapi/googleapi.go
// to detect the length (see getReaderSize function)
//
// Without this the getReaderSize function reads the entire file into
// memory to find its length.
type SeekWrapper struct {
	In   io.Reader
	Size int64
}

// Read bytes from the object - see io.Reader
func (file *SeekWrapper) Read(p []byte) (n int, err error) {
	return file.In.Read(p)
}

// Seek - minimal implementation for Google API length detection
func (file *SeekWrapper) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case os.SEEK_CUR:
		return 0, nil
	case os.SEEK_END:
		return file.Size, nil
	}
	return 0, nil
}

// Interfaces that SeekWrapper implements
var _ io.Reader = (*SeekWrapper)(nil)
var _ io.Seeker = (*SeekWrapper)(nil)
