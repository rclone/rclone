package fs

import (
	"io"
	"sync"

	"github.com/pkg/errors"
)

const asyncBufferSize = 128 * 1024

var asyncBufferPool = sync.Pool{
	New: func() interface{} { return newBuffer() },
}

// asyncReader will do async read-ahead from the input reader
// and make the data available as an io.Reader.
// This should be fully transparent, except that once an error
// has been returned from the Reader, it will not recover.
type asyncReader struct {
	in      io.ReadCloser // Input reader
	ready   chan *buffer  // Buffers ready to be handed to the reader
	token   chan struct{} // Tokens which allow a buffer to be taken
	exit    chan struct{} // Closes when finished
	buffers int           // Number of buffers
	err     error         // If an error has occurred it is here
	cur     *buffer       // Current buffer being served
	exited  chan struct{} // Channel is closed been the async reader shuts down
}

// newAsyncReader returns a reader that will asynchronously read from
// the supplied Reader into a number of buffers each of size asyncBufferSize
// It will start reading from the input at once, maybe even before this
// function has returned.
// The input can be read from the returned reader.
// When done use Close to release the buffers and close the supplied input.
func newAsyncReader(rd io.ReadCloser, buffers int) (io.ReadCloser, error) {
	if buffers <= 0 {
		return nil, errors.New("number of buffers too small")
	}
	if rd == nil {
		return nil, errors.New("nil reader supplied")
	}
	a := &asyncReader{}
	a.init(rd, buffers)
	return a, nil
}

func (a *asyncReader) init(rd io.ReadCloser, buffers int) {
	a.in = rd
	a.ready = make(chan *buffer, buffers)
	a.token = make(chan struct{}, buffers)
	a.exit = make(chan struct{}, 0)
	a.exited = make(chan struct{}, 0)
	a.buffers = buffers
	a.cur = nil

	// Create tokens
	for i := 0; i < buffers; i++ {
		a.token <- struct{}{}
	}

	// Start async reader
	go func() {
		// Ensure that when we exit this is signalled.
		defer close(a.exited)
		defer close(a.ready)
		for {
			select {
			case <-a.token:
				b := a.getBuffer()
				err := b.read(a.in)
				a.ready <- b
				if err != nil {
					return
				}
			case <-a.exit:
				return
			}
		}
	}()
}

// return the buffer to the pool (clearing it)
func (a *asyncReader) putBuffer(b *buffer) {
	b.clear()
	asyncBufferPool.Put(b)
}

// get a buffer from the pool
func (a *asyncReader) getBuffer() *buffer {
	return asyncBufferPool.Get().(*buffer)
}

// Read will return the next available data.
func (a *asyncReader) fill() (err error) {
	if a.cur.isEmpty() {
		if a.cur != nil {
			a.putBuffer(a.cur)
			a.token <- struct{}{}
			a.cur = nil
		}
		b, ok := <-a.ready
		if !ok {
			return a.err
		}
		a.cur = b
	}
	return nil
}

// Read will return the next available data.
func (a *asyncReader) Read(p []byte) (n int, err error) {
	// Swap buffer and maybe return error
	err = a.fill()
	if err != nil {
		return 0, err
	}

	// Copy what we can
	n = copy(p, a.cur.buffer())
	a.cur.increment(n)

	// If at end of buffer, return any error, if present
	if a.cur.isEmpty() {
		a.err = a.cur.err
		return n, a.err
	}
	return n, nil
}

// WriteTo writes data to w until there's no more data to write or when an error occurs.
// The return value n is the number of bytes written.
// Any error encountered during the write is also returned.
func (a *asyncReader) WriteTo(w io.Writer) (n int64, err error) {
	n = 0
	for {
		err = a.fill()
		if err != nil {
			return n, err
		}
		n2, err := w.Write(a.cur.buffer())
		a.cur.increment(n2)
		n += int64(n2)
		if err != nil {
			return n, err
		}
		if a.cur.err != nil {
			a.err = a.cur.err
			return n, a.cur.err
		}
	}
}

// Close will ensure that the underlying async reader is shut down.
// It will also close the input supplied on newAsyncReader.
func (a *asyncReader) Close() (err error) {
	// Return if already closed
	select {
	case <-a.exit:
		return
	default:
	}
	// Close and wait for go routine
	close(a.exit)
	<-a.exited
	// Return any outstanding buffers to the Pool
	if a.cur != nil {
		a.putBuffer(a.cur)
	}
	for b := range a.ready {
		a.putBuffer(b)
	}
	return a.in.Close()
}

// Internal buffer
// If an error is present, it must be returned
// once all buffer content has been served.
type buffer struct {
	buf    []byte
	err    error
	offset int
}

func newBuffer() *buffer {
	return &buffer{
		buf: make([]byte, asyncBufferSize),
		err: nil,
	}
}

// clear returns the buffer to its full size and clears the members
func (b *buffer) clear() {
	b.buf = b.buf[:cap(b.buf)]
	b.err = nil
	b.offset = 0
}

// isEmpty returns true is offset is at end of
// buffer, or
func (b *buffer) isEmpty() bool {
	if b == nil {
		return true
	}
	if len(b.buf)-b.offset <= 0 {
		return true
	}
	return false
}

// read into start of the buffer from the supplied reader,
// resets the offset and updates the size of the buffer.
// Any error encountered during the read is returned.
func (b *buffer) read(rd io.Reader) error {
	var n int
	n, b.err = ReadFill(rd, b.buf)
	b.buf = b.buf[0:n]
	b.offset = 0
	return b.err
}

// Return the buffer at current offset
func (b *buffer) buffer() []byte {
	return b.buf[b.offset:]
}

// increment the offset
func (b *buffer) increment(n int) {
	b.offset += n
}
