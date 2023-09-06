package rfc822

import (
	"bytes"
	"fmt"
	"io"
)

type MultipartWriter struct {
	w        io.Writer
	boundary string
}

func NewMultipartWriter(w io.Writer, boundary string) *MultipartWriter {
	return &MultipartWriter{w: w, boundary: boundary}
}

func (w *MultipartWriter) AddPart(fn func(io.Writer) error) error {
	buf := new(bytes.Buffer)

	if err := fn(buf); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w.w, "--%v\r\n%v\r\n", w.boundary, buf.String()); err != nil {
		return err
	}

	return nil
}

func (w *MultipartWriter) Done() error {
	if _, err := fmt.Fprintf(w.w, "--%v--\r\n", w.boundary); err != nil {
		return err
	}

	return nil
}
