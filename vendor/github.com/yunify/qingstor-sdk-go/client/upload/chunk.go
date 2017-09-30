package upload

import (
	"bytes"
	"io"
	"errors"
)

const (
	// QingStor has a max upload parts limit to 10000.
	maxUploadParts = 10000
	// We read from stream for read 1024B.
	segmentSize = 1024
)

// chunk provides a struct to read file
type chunk struct {
	fd       io.Reader
	cur      int64
	size     int64
	partSize int
}

// newChunk creates a FileChunk struct
func newChunk(fd io.Reader, partSize int) *chunk {
	f := &chunk{
		fd:       fd,
		partSize: partSize,
	}
	f.initSize()

	return f
}

// nextPart reads the next part of the file
func (f *chunk) nextPart() (io.ReadSeeker, error) {
	type readerAtSeeker interface {
		io.ReaderAt
		io.ReadSeeker
	}
	switch r := f.fd.(type) {
	case readerAtSeeker:
		var sectionSize int64
		var err error
		leftSize := f.size - f.cur
		if leftSize >= int64(f.partSize) {
			sectionSize = int64(f.partSize)
		} else if leftSize > 0 {
			sectionSize = f.size - f.cur
		} else {
			err = io.EOF
		}
		seekReader := io.NewSectionReader(r, f.cur, sectionSize)
		f.cur += sectionSize
		return seekReader, err
	case io.Reader:
		buf := make([]byte, segmentSize)
		var n, lenBuf int
		var err error
		var chunk []byte
		for {
			n, _ = r.Read(buf)
			if n == 0 {
				if lenBuf == 0 {
					err = io.EOF
				}
				break
			}
			lenBuf = lenBuf + n
			chunk = append(chunk, buf...)
			if lenBuf == f.partSize {
				break
			}
		}
		partBody := bytes.NewReader(chunk[:lenBuf])
		return partBody, err
	default:
		return nil, errors.New("file does not support read")
	}
}

// initSize tries to detect the total stream size, setting u.size. If
// the size is not known, size is set to -1.
func (f *chunk) initSize() {
	f.size = -1

	switch r := f.fd.(type) {
	case io.Seeker:
		pos, _ := r.Seek(0, 1)
		defer r.Seek(pos, 0)

		n, err := r.Seek(0, 2)
		if err != nil {
			return
		}
		f.size = n

		// Try to adjust partSize if it is too small and account for
		// integer division truncation.
		if f.size/int64(f.partSize) >= int64(maxUploadParts) {
			// Add one to the part size to account for remainders
			// during the size calculation. e.g odd number of bytes.
			f.partSize = int(f.size/int64(maxUploadParts)) + 1
		}
	}
}
