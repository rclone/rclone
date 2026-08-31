package readers

import (
	"io"
	"sync"
)

// noClose is used to wrap an io.Reader to stop it being upgraded
type noClose struct {
	in io.Reader
}

// Read implements io.Reader by passing it straight on
func (nc noClose) Read(p []byte) (n int, err error) {
	return nc.in.Read(p)
}

// noCloseWriterTo is a noClose which also forwards io.WriterTo
type noCloseWriterTo struct {
	noClose
}

// WriteTo implements io.WriterTo by passing it straight on
func (nc noCloseWriterTo) WriteTo(w io.Writer) (n int64, err error) {
	return nc.in.(io.WriterTo).WriteTo(w)
}

// NoCloser makes sure that the io.Reader passed in can't upgraded to
// an io.Closer.
//
// This is for use with http.NewRequest to make sure the body doesn't
// get upgraded to an io.Closer and the body closed unexpectedly.
//
// If in implements io.WriterTo then the returned reader does too so
// that io.Copy can still use the more efficient path.
func NoCloser(in io.Reader) io.Reader {
	if in == nil {
		return in
	}
	// if in doesn't implement io.Closer, just return it
	if _, canClose := in.(io.Closer); !canClose {
		return in
	}
	if _, canWriteTo := in.(io.WriterTo); canWriteTo {
		return noCloseWriterTo{noClose{in: in}}
	}
	return noClose{in: in}
}

// noCloseNotify is used to wrap an io.Reader replacing its Close method
type noCloseNotify struct {
	in     io.Reader
	once   sync.Once
	notify func()
}

// Read implements io.Reader by passing it straight on
func (nc *noCloseNotify) Read(p []byte) (n int, err error) {
	return nc.in.Read(p)
}

// Close calls notify the first time it is called. It never closes the
// underlying reader.
func (nc *noCloseNotify) Close() error {
	nc.once.Do(nc.notify)
	return nil
}

// noCloseNotifyWriterTo is a noCloseNotify which also forwards io.WriterTo
type noCloseNotifyWriterTo struct {
	noCloseNotify
}

// WriteTo implements io.WriterTo by passing it straight on
func (nc *noCloseNotifyWriterTo) WriteTo(w io.Writer) (n int64, err error) {
	return nc.in.(io.WriterTo).WriteTo(w)
}

// NoCloserNotify makes sure the io.Reader passed in can't be upgraded
// to an io.Closer, like NoCloser, but returns an io.ReadCloser whose
// Close calls notify (once only) instead of closing in.
//
// This is for use with http.NewRequest to find out when the transport
// has finished with the request body. The transport always closes the
// body, but is documented to possibly do so in a different goroutine
// after the request has finished, so notify signals when the
// transport can no longer be reading the body.
//
// If in implements io.WriterTo then the returned reader does too so
// that io.Copy can still use the more efficient path.
func NoCloserNotify(in io.Reader, notify func()) io.ReadCloser {
	if in == nil {
		return nil
	}
	if _, canWriteTo := in.(io.WriterTo); canWriteTo {
		return &noCloseNotifyWriterTo{noCloseNotify{in: in, notify: notify}}
	}
	return &noCloseNotify{in: in, notify: notify}
}
