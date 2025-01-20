package cryptomator

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	// ChunkPayloadSize is the size of the encrypted payload of each file chunk.
	ChunkPayloadSize = 32 * 1024
)

// encryptedFileSize returns the size of the file after encrypting it
func (c *cryptor) encryptedFileSize(size int64) int64 {
	overhead := int64(c.encryptionOverhead())

	fullChunksSize := (size / ChunkPayloadSize) * (ChunkPayloadSize + overhead)

	rest := size % ChunkPayloadSize
	if rest > 0 {
		rest += overhead
	}

	return headerPayloadSize + overhead + fullChunksSize + rest
}

// decryptedFileSize returns the size of the file after decrypting it
func (c *cryptor) decryptedFileSize(size int64) int64 {
	overhead := int64(c.encryptionOverhead())

	size = size - headerPayloadSize - overhead

	fullChunksSize := (size / (ChunkPayloadSize + overhead)) * ChunkPayloadSize

	rest := size % (ChunkPayloadSize + overhead)
	if rest > 0 {
		rest -= overhead
	}

	return fullChunksSize + rest
}

const (
	lastChunk    = true
	notLastChunk = false
)

// reader decrypts a Cryptomator file as it is read from.
type reader struct {
	cryptor contentCryptor
	header  fileHeader
	src     io.Reader

	unread []byte
	buf    []byte

	chunkNr uint64

	err error
}

// newContentReader creates a new Reader for the file content using the previously file header.
func (c *cryptor) newContentReader(src io.Reader, header fileHeader) (*reader, error) {
	cryptor, err := c.newContentCryptor(header.ContentKey)
	if err != nil {
		return nil, err
	}
	return &reader{
		cryptor: cryptor,
		header:  header,
		src:     src,
		buf:     make([]byte, ChunkPayloadSize+c.encryptionOverhead()),
	}, nil
}

// newReader reads the file header and returns a Reader for the content.
func (c *cryptor) newReader(src io.Reader) (r *reader, err error) {
	header, err := c.unmarshalHeader(src)
	if err != nil {
		return
	}
	return c.newContentReader(src, header)
}

func (r *reader) Read(p []byte) (int, error) {
	if len(r.unread) > 0 {
		n := copy(p, r.unread)
		r.unread = r.unread[n:]
		return n, nil
	}

	if r.err != nil {
		return 0, r.err
	}
	if len(p) == 0 {
		return 0, nil
	}

	last, err := r.readChunk()
	if err != nil {
		r.err = err
		return 0, err
	}

	n := copy(p, r.unread)
	r.unread = r.unread[n:]

	if last {
		if _, err := r.src.Read(make([]byte, 1)); err == nil {
			r.err = errors.New("trailing data after end of encrypted file")
		} else if err != io.EOF {
			r.err = fmt.Errorf("non-EOF error reading after end of encrypted file: %w", err)
		} else {
			r.err = io.EOF
		}
	}

	return n, nil
}

func (r *reader) readChunk() (last bool, err error) {
	if len(r.unread) != 0 {
		panic("stream: internal error: readChunk called with dirty buffer")
	}

	in := r.buf
	n, err := io.ReadFull(r.src, in)

	switch {
	case err == io.EOF:
		// TODO
		// return false, io.ErrUnexpectedEOF
		return true, nil
	case err == io.ErrUnexpectedEOF:
		last = true
		in = in[:n]
	case err != nil:
		return false, err
	}

	ad := r.cryptor.fileAssociatedData(r.header.Nonce, r.chunkNr)
	payload, err := r.cryptor.decryptChunk(in, ad)
	if err != nil {
		return
	}

	r.chunkNr++
	r.unread = r.buf[:copy(r.buf, payload)]
	return last, nil
}

// writer encrypts a Cryptomator file as it is written to.
type writer struct {
	cryptor contentCryptor
	header  fileHeader

	dst       io.Writer
	unwritten []byte
	buf       []byte

	err error

	chunkNr uint64
}

// newContentWriter creates a new Writer for the file content using the already written file header.
func (c *cryptor) newContentWriter(dst io.Writer, header fileHeader) (*writer, error) {
	cryptor, err := c.newContentCryptor(header.ContentKey)
	if err != nil {
		return nil, err
	}
	w := &writer{
		cryptor: cryptor,
		header:  header,
		dst:     dst,
		buf:     make([]byte, ChunkPayloadSize+c.encryptionOverhead()),
	}

	w.unwritten = w.buf[:0]
	return w, nil
}

// newWriter creates and writes a random file header and returns a writer for the file content.
func (c *cryptor) newWriter(dst io.Writer) (w *writer, err error) {
	header, err := c.NewHeader()
	if err != nil {
		return
	}
	err = c.marshalHeader(dst, header)
	if err != nil {
		return
	}
	return c.newContentWriter(dst, header)
}

func (w *writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if len(p) == 0 {
		return 0, nil
	}

	total := len(p)
	for len(p) > 0 {
		freeBuf := w.buf[len(w.unwritten):ChunkPayloadSize]
		n := copy(freeBuf, p)
		p = p[n:]
		w.unwritten = w.unwritten[:len(w.unwritten)+n]

		if len(w.unwritten) == ChunkPayloadSize && len(p) > 0 {
			if err := w.flushChunk(notLastChunk); err != nil {
				w.err = err
				return 0, err
			}
		}
	}
	return total, nil
}

// Close flushes the last chunk. It doesn't close the underlying Writer.
func (w *writer) Close() error {
	if w.err != nil {
		return w.err
	}

	w.err = w.flushChunk(lastChunk)
	if w.err != nil {
		return w.err
	}

	w.err = errors.New("stream.Writer is already closed")
	return nil
}

func (w *writer) flushChunk(last bool) error {
	if !last && len(w.unwritten) != ChunkPayloadSize {
		panic("stream: internal error: flush called with partial chunk")
	}

	if len(w.unwritten) == 0 {
		return nil
	}

	nonce := make([]byte, w.cryptor.nonceSize())
	_, err := rand.Read(nonce)
	if err != nil {
		return fmt.Errorf("stream: generating nonce failed: %w", err)
	}
	ad := w.cryptor.fileAssociatedData(w.header.Nonce, w.chunkNr)
	out := w.cryptor.encryptChunk(w.unwritten, nonce, ad)

	_, err = w.dst.Write(out)

	w.unwritten = w.buf[:0]
	w.chunkNr++
	return err
}
