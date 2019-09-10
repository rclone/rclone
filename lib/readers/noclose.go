package readers

import "io"

// noClose is used to wrap an io.Reader to stop it being upgraded
type noClose struct {
	in io.Reader
}

// Read implements io.Closer by passing it straight on
func (nc noClose) Read(p []byte) (n int, err error) {
	return nc.in.Read(p)
}

// NoCloser makes sure that the io.Reader passed in can't upgraded to
// an io.Closer.
//
// This is for use with http.NewRequest to make sure the body doesn't
// get upgraded to an io.Closer and the body closed unexpectedly.
func NoCloser(in io.Reader) io.Reader {
	if in == nil {
		return in
	}
	// if in doesn't implement io.Closer, just return it
	if _, canClose := in.(io.Closer); !canClose {
		return in
	}
	return noClose{in: in}
}
