package chunkedreader

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pool"
)

// parallel reads Object in chunks of a given size in parallel.
type parallel struct {
	ctx       context.Context
	o         fs.Object  // source to read from
	mu        sync.Mutex // protects following fields
	endStream int64      // offset we have started streams for
	offset    int64      // offset the read file pointer is at
	chunkSize int64      // length of the chunks to read
	nstreams  int        // number of streams to use
	streams   []*stream  // the opened streams in offset order - the current one is first
	closed    bool       // has Close been called?
}

// stream holds the info about a single download
type stream struct {
	cr        *parallel       // parent reader
	ctx       context.Context // ctx to cancel if needed
	cancel    func()          // cancel the stream
	rc        io.ReadCloser   // reader that it is reading from, may be nil
	offset    int64           // where the stream is reading from
	size      int64           // and the size it is reading
	readBytes int64           // bytes read from the stream
	rw        *pool.RW        // buffer for read
	err       chan error      // error returned from the read
	name      string          // name of this stream for debugging
}

// Start a stream reading (offset, offset+size)
func (cr *parallel) newStream(ctx context.Context, offset, size int64) (s *stream, err error) {
	ctx, cancel := context.WithCancel(ctx)

	// Create the stream
	rw := multipart.NewRW()
	s = &stream{
		cr:     cr,
		ctx:    ctx,
		cancel: cancel,
		offset: offset,
		size:   size,
		rw:     rw,
		err:    make(chan error, 1),
	}
	s.name = fmt.Sprintf("stream(%d,%d,%p)", s.offset, s.size, s)

	// Start the background read into the buffer
	go s.readFrom(ctx)

	// Return the stream to the caller
	return s, nil
}

// read the file into the buffer
func (s *stream) readFrom(ctx context.Context) {
	// Open the object at the correct range
	fs.Debugf(s.cr.o, "%s: open", s.name)
	rc, err := operations.Open(ctx, s.cr.o,
		&fs.HashesOption{Hashes: hash.Set(hash.None)},
		&fs.RangeOption{Start: s.offset, End: s.offset + s.size - 1})
	if err != nil {
		s.err <- fmt.Errorf("parallel chunked reader: failed to open stream at %d size %d: %w", s.offset, s.size, err)
		return
	}
	s.rc = rc

	fs.Debugf(s.cr.o, "%s: readfrom started", s.name)
	_, err = s.rw.ReadFrom(s.rc)
	fs.Debugf(s.cr.o, "%s: readfrom finished (%d bytes): %v", s.name, s.rw.Size(), err)
	s.err <- err
}

// eof is true when we've read all the data we are expecting
func (s *stream) eof() bool {
	return s.readBytes >= s.size
}

// read reads up to len(p) bytes into p. It returns the number of
// bytes read (0 <= n <= len(p)) and any error encountered. If some
// data is available but not len(p) bytes, read returns what is
// available instead of waiting for more.
func (s *stream) read(p []byte) (n int, err error) {
	defer log.Trace(s.cr.o, "%s: Read len(p)=%d", s.name, len(p))("n=%d, err=%v", &n, &err)
	if len(p) == 0 {
		return n, nil
	}
	for {
		var nn int
		nn, err = s.rw.Read(p[n:])
		fs.Debugf(s.cr.o, "%s: rw.Read nn=%d, err=%v", s.name, nn, err)
		s.readBytes += int64(nn)
		n += nn
		if err != nil && err != io.EOF {
			return n, err
		}
		if s.eof() {
			return n, io.EOF
		}
		// Received a faux io.EOF because we haven't read all the data yet
		if n >= len(p) {
			break
		}
		// Wait for a write to happen to read more
		s.rw.WaitWrite(s.ctx)
	}
	return n, nil
}

// Sets *perr to newErr if err is nil
func orErr(perr *error, newErr error) {
	if *perr == nil {
		*perr = newErr
	}
}

// Close a stream
func (s *stream) close() (err error) {
	defer log.Trace(s.cr.o, "%s: close", s.name)("err=%v", &err)
	s.cancel()
	err = <-s.err // wait for readFrom to stop and return error
	orErr(&err, s.rw.Close())
	if s.rc != nil {
		orErr(&err, s.rc.Close())
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("parallel chunked reader: failed to read stream at %d size %d: %w", s.offset, s.size, err)
	}
	return nil
}

// Make a new parallel chunked reader
//
// Mustn't be called for an unknown size object
func newParallel(ctx context.Context, o fs.Object, chunkSize int64, streams int) ChunkedReader {
	// Make sure chunkSize is a multiple of multipart.BufferSize
	if chunkSize < 0 {
		chunkSize = multipart.BufferSize
	}
	newChunkSize := multipart.BufferSize * (chunkSize / multipart.BufferSize)
	if newChunkSize < chunkSize {
		newChunkSize += multipart.BufferSize
	}

	fs.Debugf(o, "newParallel chunkSize=%d, streams=%d", chunkSize, streams)

	return &parallel{
		ctx:       ctx,
		o:         o,
		offset:    0,
		chunkSize: newChunkSize,
		nstreams:  streams,
	}
}

// _open starts the file transferring at offset
//
// Call with the lock held
func (cr *parallel) _open() (err error) {
	size := cr.o.Size()
	if size < 0 {
		return fmt.Errorf("parallel chunked reader: can't use multiple threads for unknown sized object %q", cr.o)
	}
	// Launched enough streams already
	if cr.endStream >= size {
		return nil
	}

	// Make sure cr.nstreams are running
	for i := len(cr.streams); i < cr.nstreams; i++ {
		// clip to length of file
		chunkSize := cr.chunkSize
		newEndStream := cr.endStream + chunkSize
		if newEndStream > size {
			chunkSize = size - cr.endStream
			newEndStream = cr.endStream + chunkSize
		}

		s, err := cr.newStream(cr.ctx, cr.endStream, chunkSize)
		if err != nil {
			return err
		}
		cr.streams = append(cr.streams, s)
		cr.endStream = newEndStream

		if cr.endStream >= size {
			break
		}
	}

	return nil
}

// Finished reading the current stream so pop it off and destroy it
//
// Call with lock held
func (cr *parallel) _popStream() (err error) {
	defer log.Trace(cr.o, "streams=%+v", cr.streams)("streams=%+v, err=%v", &cr.streams, &err)
	if len(cr.streams) == 0 {
		return nil
	}
	stream := cr.streams[0]
	err = stream.close()
	cr.streams[0] = nil
	cr.streams = cr.streams[1:]
	return err
}

// Get rid of all the streams
//
// Call with lock held
func (cr *parallel) _popStreams() (err error) {
	defer log.Trace(cr.o, "streams=%+v", cr.streams)("streams=%+v, err=%v", &cr.streams, &err)
	for len(cr.streams) > 0 {
		orErr(&err, cr._popStream())
	}
	cr.streams = nil
	return err
}

// Read from the file - for details see io.Reader
func (cr *parallel) Read(p []byte) (n int, err error) {
	defer log.Trace(cr.o, "Read len(p)=%d", len(p))("n=%d, err=%v", &n, &err)
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.closed {
		return 0, ErrorFileClosed
	}

	for n < len(p) {
		// Make sure we have the correct number of streams open
		err = cr._open()
		if err != nil {
			return n, err
		}

		// No streams left means EOF
		if len(cr.streams) == 0 {
			return n, io.EOF
		}

		// Read from the stream
		stream := cr.streams[0]
		nn, err := stream.read(p[n:])
		n += nn
		cr.offset += int64(nn)
		if err == io.EOF {
			err = cr._popStream()
			if err != nil {
				break
			}
		} else if err != nil {
			break
		}
	}
	return n, err
}

// Close the file - for details see io.Closer
//
// All methods on ChunkedReader will return ErrorFileClosed afterwards
func (cr *parallel) Close() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.closed {
		return ErrorFileClosed
	}
	cr.closed = true

	// Close all the streams
	return cr._popStreams()
}

// Seek the file - for details see io.Seeker
func (cr *parallel) Seek(offset int64, whence int) (int64, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	fs.Debugf(cr.o, "parallel chunked reader: seek from %d to %d whence %d", cr.offset, offset, whence)

	if cr.closed {
		return 0, ErrorFileClosed
	}

	size := cr.o.Size()
	currentOffset := cr.offset
	switch whence {
	case io.SeekStart:
		currentOffset = 0
	case io.SeekEnd:
		currentOffset = size
	}
	// set the new chunk start
	newOffset := currentOffset + offset
	if newOffset < 0 || newOffset >= size {
		return 0, ErrorInvalidSeek
	}

	// If seek pointer didn't move, return now
	if newOffset == cr.offset {
		fs.Debugf(cr.o, "parallel chunked reader: seek pointer didn't move")
		return cr.offset, nil
	}

	cr.offset = newOffset

	// Ditch out of range streams
	for len(cr.streams) > 0 {
		stream := cr.streams[0]
		if newOffset >= stream.offset+stream.size {
			_ = cr._popStream()
		} else {
			break
		}
	}

	// If no streams remain we can just restart
	if len(cr.streams) == 0 {
		fs.Debugf(cr.o, "parallel chunked reader: no streams remain")
		cr.endStream = cr.offset
		return cr.offset, nil
	}

	// Current stream
	stream := cr.streams[0]

	// If new offset is before current stream then ditch all the streams
	if newOffset < stream.offset {
		_ = cr._popStreams()
		fs.Debugf(cr.o, "parallel chunked reader: new offset is before current stream - ditch all")
		cr.endStream = cr.offset
		return cr.offset, nil
	}

	// Seek the current stream
	streamOffset := newOffset - stream.offset
	stream.readBytes = streamOffset // correct read value
	fs.Debugf(cr.o, "parallel chunked reader: seek the current stream to %d", streamOffset)
	// Wait for the read to the correct part of the data
	for stream.rw.Size() < streamOffset {
		stream.rw.WaitWrite(cr.ctx)
	}
	_, err := stream.rw.Seek(streamOffset, io.SeekStart)
	if err != nil {
		return cr.offset, fmt.Errorf("parallel chunked reader: failed to seek stream: %w", err)
	}

	return cr.offset, nil
}

// RangeSeek the file - for details see RangeSeeker
//
// In the parallel chunked reader this just acts like Seek
func (cr *parallel) RangeSeek(ctx context.Context, offset int64, whence int, length int64) (int64, error) {
	return cr.Seek(offset, whence)
}

// Open forces the connection to be opened
func (cr *parallel) Open() (ChunkedReader, error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	return cr, cr._open()
}

var (
	_ ChunkedReader = (*parallel)(nil)
)
