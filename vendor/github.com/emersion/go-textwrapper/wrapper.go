// A writer that wraps long text lines to a specified length.
package textwrapper

import (
	"io"
)

type writer struct {
	Len int

	sepBytes []byte
	w        io.Writer
	i        int
}

func (w *writer) Write(b []byte) (N int, err error) {
	to := w.Len - w.i

	for len(b) > to {
		var n int
		n, err = w.w.Write(b[:to])
		if err != nil {
			return
		}
		N += n
		b = b[to:]

		_, err = w.w.Write(w.sepBytes)
		if err != nil {
			return
		}

		w.i = 0
		to = w.Len
	}

	w.i += len(b)

	n, err := w.w.Write(b)
	if err != nil {
		return
	}
	N += n

	return
}

// Returns a writer that splits its input into multiple parts that have the same
// length and adds a separator between these parts.
func New(w io.Writer, sep string, l int) io.Writer {
	return &writer{
		Len:      l,
		sepBytes: []byte(sep),
		w:        w,
	}
}

// Creates a RFC822 text wrapper. It adds a CRLF (ie. \r\n) each 76 characters.
func NewRFC822(w io.Writer) io.Writer {
	return New(w, "\r\n", 76)
}
