package chunkedreader

import (
	"io"
	"sync"

	"github.com/ncw/rclone/fs"
)

// ChunkedReader is a reader for a Object with the possibility
// of reading the source in chunks of given size
//
// A initialChunkSize of 0 will disable chunked reading.
type ChunkedReader struct {
	mu               sync.Mutex
	o                fs.Object
	rc               io.ReadCloser
	offset           int64
	chunkOffset      int64
	chunkSize        int64
	initialChunkSize int64
	chunkGrowth      bool
	doSeek           bool
}

// New returns a ChunkedReader for the Object.
//
// A initialChunkSize of 0 will disable chunked reading.
// If chunkGrowth is true, the chunk size will be doubled after each chunk read.
// A Seek or RangeSeek will reset the chunk size to it's initial value
func New(o fs.Object, initialChunkSize int64, chunkGrowth bool) *ChunkedReader {
	if initialChunkSize < 0 {
		initialChunkSize = 0
	}
	return &ChunkedReader{
		o:                o,
		offset:           -1,
		chunkSize:        initialChunkSize,
		initialChunkSize: initialChunkSize,
		chunkGrowth:      chunkGrowth,
	}
}

// Read from the file - for details see io.Reader
func (cr *ChunkedReader) Read(p []byte) (n int, err error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	for reqSize := int64(len(p)); reqSize > 0; reqSize = int64(len(p)) {
		chunkEnd := cr.chunkOffset + cr.chunkSize

		fs.Debugf(cr.o, "ChunkedReader.Read at %d length %d chunkOffset %d chunkSize %d", cr.offset, reqSize, cr.chunkOffset, cr.chunkSize)

		if atChunkEnd := cr.offset == chunkEnd; cr.offset == -1 || atChunkEnd {
			if atChunkEnd && cr.chunkSize > 0 {
				if cr.doSeek {
					cr.doSeek = false
					cr.chunkSize = cr.initialChunkSize
				} else if cr.chunkGrowth {
					cr.chunkSize *= 2
				}
				cr.chunkOffset = cr.offset
			}
			err = cr.openRange()
			if err != nil {
				return
			}
		}

		var buf []byte
		chunkRest := chunkEnd - cr.offset
		if reqSize > chunkRest && cr.chunkSize != 0 {
			buf, p = p[0:chunkRest], p[chunkRest:]
		} else {
			buf, p = p, nil
		}
		var rn int
		rn, err = io.ReadFull(cr.rc, buf)
		n += rn
		cr.offset += int64(rn)
		if err != nil {
			return
		}
	}
	return n, nil
}

// Close the file - for details see io.Closer
func (cr *ChunkedReader) Close() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	return cr.resetReader(nil, 0)
}

// Seek the file - for details see io.Seeker
func (cr *ChunkedReader) Seek(offset int64, whence int) (int64, error) {
	return cr.RangeSeek(offset, whence, -1)
}

// RangeSeek the file - for details see RangeSeeker
func (cr *ChunkedReader) RangeSeek(offset int64, whence int, length int64) (int64, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	fs.Debugf(cr.o, "ChunkedReader.RangeSeek from %d to %d", cr.offset, offset)

	size := cr.o.Size()
	switch whence {
	case 0:
		cr.offset = 0
	case 2:
		cr.offset = size
	}
	cr.chunkOffset = cr.offset + offset
	cr.offset = -1
	cr.doSeek = true
	if length > 0 {
		cr.chunkSize = length
	} else {
		cr.chunkSize = cr.initialChunkSize
	}
	return cr.offset, nil
}

// Open forces the connection to be opened
func (cr *ChunkedReader) Open() (*ChunkedReader, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	return cr, cr.openRange()
}

// openRange will open the source Object with the given range
// A length <= 0 will request till the end of the file
func (cr *ChunkedReader) openRange() error {
	offset, length := cr.chunkOffset, cr.chunkSize
	fs.Debugf(cr.o, "ChunkedReader.openRange at %d length %d", offset, length)

	var rc io.ReadCloser
	var err error
	if length <= 0 {
		if offset == 0 {
			rc, err = cr.o.Open()
		} else {
			rc, err = cr.o.Open(&fs.RangeOption{Start: offset, End: -1})
		}
	} else {
		rc, err = cr.o.Open(&fs.RangeOption{Start: offset, End: offset + length - 1})
	}
	if err != nil {
		return err
	}
	return cr.resetReader(rc, offset)
}

// resetReader switches the current reader to the given reader.
// The old reader will be Close'd before setting the new reader.
func (cr *ChunkedReader) resetReader(rc io.ReadCloser, offset int64) error {
	if cr.rc != nil {
		if err := cr.rc.Close(); err != nil {
			return err
		}
	}
	cr.rc = rc
	cr.offset = offset
	return nil
}

var (
	_ io.ReadCloser  = (*ChunkedReader)(nil)
	_ io.Seeker      = (*ChunkedReader)(nil)
	_ fs.RangeSeeker = (*ChunkedReader)(nil)
)
