// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"bufio"
	"io"
	"sync"

	"storj.io/drpc"
)

//
// Writer
//

// Writer is a helper to buffer and write packets and frames to an io.Writer.
type Writer struct {
	w    io.Writer
	size int
	mu   sync.Mutex
	buf  []byte
}

// NewWriter returns a Writer that will attempt to buffer size data before
// sending it to the io.Writer.
func NewWriter(w io.Writer, size int) *Writer {
	if size == 0 {
		size = 1024
	}

	return &Writer{
		w:    w,
		size: size,
		buf:  make([]byte, 0, size),
	}
}

// WritePacket writes the packet as a single frame, ignoring any size
// constraints.
func (b *Writer) WritePacket(pkt Packet) (err error) {
	return b.WriteFrame(Frame{
		Data: pkt.Data,
		ID:   pkt.ID,
		Kind: pkt.Kind,
		Done: true,
	})
}

// WriteFrame appends the frame into the buffer, and if the buffer is larger
// than the configured size, flushes it.
func (b *Writer) WriteFrame(fr Frame) (err error) {
	b.mu.Lock()
	b.buf = AppendFrame(b.buf, fr)
	if len(b.buf) >= b.size {
		_, err = b.w.Write(b.buf)
		b.buf = b.buf[:0]
	}
	b.mu.Unlock()
	return err
}

// Flush forces a flush of any buffered data to the io.Writer. It is a no-op if
// there is no data in the buffer.
func (b *Writer) Flush() (err error) {
	defer mon.Task()(nil)(&err)

	b.mu.Lock()
	if len(b.buf) > 0 {
		_, err = b.w.Write(b.buf)
		b.buf = b.buf[:0]
	}
	b.mu.Unlock()
	return err
}

//
// Reader
//

// SplitFrame is used by bufio.Scanner to split frames out of a stream of bytes.
func SplitFrame(data []byte, atEOF bool) (int, []byte, error) {
	rem, _, ok, err := ParseFrame(data)
	switch advance := len(data) - len(rem); {
	case err != nil:
		return 0, nil, err
	case len(data) > 0 && !ok && atEOF:
		return 0, nil, drpc.ProtocolError.New("truncated frame")
	case !ok:
		return 0, nil, nil
	case advance < 0, len(data) < advance:
		return 0, nil, drpc.InternalError.New("scanner issue with advance value")
	default:
		return advance, data[:advance], nil
	}
}

// Reader reconstructs packets from frames read from an io.Reader.
type Reader struct {
	buf *bufio.Scanner
	id  ID
}

// NewReader constructs a Reader to read Packets from the io.Reader.
func NewReader(r io.Reader) *Reader {
	buf := bufio.NewScanner(r)
	buf.Buffer(make([]byte, 4<<10), 1<<20)
	buf.Split(SplitFrame)
	return &Reader{buf: buf}
}

// ReadPacket reads a packet from the io.Reader. IDs read from frames
// must be monotonically increasing. When a new ID is read, the old
// data is discarded. This allows for easier asynchronous interrupts.
// If the amount of data in the Packet becomes too large, an error is
// returned.
func (s *Reader) ReadPacket() (pkt Packet, err error) {
	defer mon.Task()(nil)(&err)

	for s.buf.Scan() {
		rem, fr, ok, err := ParseFrame(s.buf.Bytes())
		switch {
		case err != nil:
			return Packet{}, drpc.ProtocolError.Wrap(err)
		case !ok, len(rem) > 0:
			return Packet{}, drpc.InternalError.New("problem with scanner")
		case fr.Control:
			// Ignore any frames with the control bit set so that we can
			// use it in the future to mean things to people who understand
			// it.
			continue
		case fr.ID.Less(s.id):
			return Packet{}, drpc.ProtocolError.New("id monotonicity violation")
		case s.id.Less(fr.ID):
			s.id = fr.ID
			pkt = Packet{
				Data: pkt.Data[:0],
				ID:   fr.ID,
				Kind: fr.Kind,
			}
		case fr.Kind != pkt.Kind:
			return Packet{}, drpc.ProtocolError.New("packet kind change")
		}

		pkt.Data = append(pkt.Data, fr.Data...)
		switch {
		case len(pkt.Data) > 4<<20:
			return Packet{}, drpc.ProtocolError.New("data overflow")
		case fr.Done:
			return pkt, nil
		}
	}
	if err := s.buf.Err(); err != nil {
		return Packet{}, err
	}
	return Packet{}, io.EOF
}
