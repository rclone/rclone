package chunkedreader

import (
	"context"
	"io"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// sequential is a reader for an Object with the possibility
// of reading the source in chunks of given size
//
// An initialChunkSize of <= 0 will disable chunked reading.
type sequential struct {
	ctx              context.Context
	mu               sync.Mutex    // protects following fields
	o                fs.Object     // source to read from
	rc               io.ReadCloser // reader for the current open chunk
	offset           int64         // offset the next Read will start. -1 forces a reopen of o
	chunkOffset      int64         // beginning of the current or next chunk
	chunkSize        int64         // length of the current or next chunk. -1 will open o from chunkOffset to the end
	initialChunkSize int64         // default chunkSize after the chunk specified by RangeSeek is complete
	maxChunkSize     int64         // consecutive read chunks will double in size until reached. -1 means no limit
	customChunkSize  bool          // is the current chunkSize set by RangeSeek?
	closed           bool          // has Close been called?
}

// Make a new sequential chunked reader
func newSequential(ctx context.Context, o fs.Object, initialChunkSize int64, maxChunkSize int64) ChunkedReader {
	return &sequential{
		ctx:              ctx,
		o:                o,
		offset:           -1,
		chunkSize:        initialChunkSize,
		initialChunkSize: initialChunkSize,
		maxChunkSize:     maxChunkSize,
	}
}

// Read from the file - for details see io.Reader
func (cr *sequential) Read(p []byte) (n int, err error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.closed {
		return 0, ErrorFileClosed
	}

	for reqSize := int64(len(p)); reqSize > 0; reqSize = int64(len(p)) {
		// the current chunk boundary. valid only when chunkSize > 0
		chunkEnd := cr.chunkOffset + cr.chunkSize

		fs.Debugf(cr.o, "ChunkedReader.Read at %d length %d chunkOffset %d chunkSize %d", cr.offset, reqSize, cr.chunkOffset, cr.chunkSize)

		switch {
		case cr.chunkSize > 0 && cr.offset == chunkEnd: // last chunk read completely
			cr.chunkOffset = cr.offset
			if cr.customChunkSize { // last chunkSize was set by RangeSeek
				cr.customChunkSize = false
				cr.chunkSize = cr.initialChunkSize
			} else {
				cr.chunkSize *= 2
				if cr.chunkSize > cr.maxChunkSize && cr.maxChunkSize != -1 {
					cr.chunkSize = cr.maxChunkSize
				}
			}
			// recalculate the chunk boundary. valid only when chunkSize > 0
			chunkEnd = cr.chunkOffset + cr.chunkSize
			fallthrough
		case cr.offset == -1: // first Read or Read after RangeSeek
			err = cr.openRange()
			if err != nil {
				return
			}
		}

		var buf []byte
		chunkRest := chunkEnd - cr.offset
		// limit read to chunk boundaries if chunkSize > 0
		if reqSize > chunkRest && cr.chunkSize > 0 {
			buf, p = p[0:chunkRest], p[chunkRest:]
		} else {
			buf, p = p, nil
		}
		var rn int
		rn, err = io.ReadFull(cr.rc, buf)
		n += rn
		cr.offset += int64(rn)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				err = io.EOF
			}
			return
		}
	}
	return n, nil
}

// Close the file - for details see io.Closer
//
// All methods on ChunkedReader will return ErrorFileClosed afterwards
func (cr *sequential) Close() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.closed {
		return ErrorFileClosed
	}
	cr.closed = true

	return cr.resetReader(nil, 0)
}

// Seek the file - for details see io.Seeker
func (cr *sequential) Seek(offset int64, whence int) (int64, error) {
	return cr.RangeSeek(context.TODO(), offset, whence, -1)
}

// RangeSeek the file - for details see RangeSeeker
//
// The specified length will only apply to the next chunk opened.
// RangeSeek will not reopen the source until Read is called.
func (cr *sequential) RangeSeek(ctx context.Context, offset int64, whence int, length int64) (int64, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	fs.Debugf(cr.o, "ChunkedReader.RangeSeek from %d to %d length %d", cr.offset, offset, length)

	if cr.closed {
		return 0, ErrorFileClosed
	}

	size := cr.o.Size()
	switch whence {
	case io.SeekStart:
		cr.offset = 0
	case io.SeekEnd:
		if size < 0 {
			return 0, ErrorInvalidSeek // Can't seek from end for unknown size
		}
		cr.offset = size
	}
	// set the new chunk start
	cr.chunkOffset = cr.offset + offset
	// force reopen on next Read
	cr.offset = -1
	if length > 0 {
		cr.customChunkSize = true
		cr.chunkSize = length
	} else {
		cr.chunkSize = cr.initialChunkSize
	}
	if cr.chunkOffset < 0 || cr.chunkOffset >= size {
		cr.chunkOffset = 0
		return 0, ErrorInvalidSeek
	}
	return cr.chunkOffset, nil
}

// Open forces the connection to be opened
func (cr *sequential) Open() (ChunkedReader, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.rc != nil && cr.offset != -1 {
		return cr, nil
	}
	return cr, cr.openRange()
}

// openRange will open the source Object with the current chunk range
//
// If the current open reader implements RangeSeeker, it is tried first.
// When RangeSeek fails, o.Open with a RangeOption is used.
//
// A length <= 0 will request till the end of the file
func (cr *sequential) openRange() error {
	offset, length := cr.chunkOffset, cr.chunkSize
	fs.Debugf(cr.o, "ChunkedReader.openRange at %d length %d", offset, length)

	if cr.closed {
		return ErrorFileClosed
	}

	if rs, ok := cr.rc.(fs.RangeSeeker); ok {
		n, err := rs.RangeSeek(cr.ctx, offset, io.SeekStart, length)
		if err == nil && n == offset {
			cr.offset = offset
			return nil
		}
		if err != nil {
			fs.Debugf(cr.o, "ChunkedReader.openRange seek failed (%s). Trying Open", err)
		} else {
			fs.Debugf(cr.o, "ChunkedReader.openRange seeked to wrong offset. Wanted %d, got %d. Trying Open", offset, n)
		}
	}

	var rc io.ReadCloser
	var err error
	if length <= 0 {
		if offset == 0 {
			rc, err = cr.o.Open(cr.ctx, &fs.HashesOption{Hashes: hash.Set(hash.None)})
		} else {
			rc, err = cr.o.Open(cr.ctx, &fs.HashesOption{Hashes: hash.Set(hash.None)}, &fs.RangeOption{Start: offset, End: -1})
		}
	} else {
		rc, err = cr.o.Open(cr.ctx, &fs.HashesOption{Hashes: hash.Set(hash.None)}, &fs.RangeOption{Start: offset, End: offset + length - 1})
	}
	if err != nil {
		return err
	}
	return cr.resetReader(rc, offset)
}

// resetReader switches the current reader to the given reader.
// The old reader will be Close'd before setting the new reader.
func (cr *sequential) resetReader(rc io.ReadCloser, offset int64) error {
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
	_ ChunkedReader = (*sequential)(nil)
)
