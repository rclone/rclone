package readers

import "io"

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
