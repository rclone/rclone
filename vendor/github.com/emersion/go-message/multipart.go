package message

import (
	"io"

	"github.com/emersion/go-message/textproto"
)

// MultipartReader is an iterator over parts in a MIME multipart body.
type MultipartReader interface {
	io.Closer

	// NextPart returns the next part in the multipart or an error. When there are
	// no more parts, the error io.EOF is returned.
	//
	// Entity.Body must be read completely before the next call to NextPart,
	// otherwise it will be discarded.
	NextPart() (*Entity, error)
}

type multipartReader struct {
	r *textproto.MultipartReader
}

// NextPart implements MultipartReader.
func (r *multipartReader) NextPart() (*Entity, error) {
	p, err := r.r.NextPart()
	if err != nil {
		return nil, err
	}
	return New(Header{p.Header}, p)
}

// Close implements io.Closer.
func (r *multipartReader) Close() error {
	return nil
}

type multipartBody struct {
	header Header
	parts  []*Entity

	r *io.PipeReader
	w *Writer

	i int
}

// Read implements io.Reader.
func (m *multipartBody) Read(p []byte) (n int, err error) {
	if m.r == nil {
		r, w := io.Pipe()
		m.r = r

		var err error
		m.w, err = createWriter(w, &m.header)
		if err != nil {
			return 0, err
		}

		// Prevent calls to NextPart to succeed
		m.i = len(m.parts)

		go func() {
			if err := m.writeBodyTo(m.w); err != nil {
				w.CloseWithError(err)
				return
			}

			if err := m.w.Close(); err != nil {
				w.CloseWithError(err)
				return
			}

			w.Close()
		}()
	}

	return m.r.Read(p)
}

// Close implements io.Closer.
func (m *multipartBody) Close() error {
	if m.r != nil {
		m.r.Close()
	}
	return nil
}

// NextPart implements MultipartReader.
func (m *multipartBody) NextPart() (*Entity, error) {
	if m.i >= len(m.parts) {
		return nil, io.EOF
	}

	part := m.parts[m.i]
	m.i++
	return part, nil
}

func (m *multipartBody) writeBodyTo(w *Writer) error {
	for _, p := range m.parts {
		pw, err := w.CreatePart(p.Header)
		if err != nil {
			return err
		}

		if err := p.writeBodyTo(pw); err != nil {
			return err
		}
		if err := pw.Close(); err != nil {
			return err
		}
	}
	return nil
}
