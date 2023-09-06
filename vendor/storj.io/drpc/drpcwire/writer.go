// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcwire

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"storj.io/drpc/drpcdebug"
)

//
// Writer
//

// Writer is a helper to buffer and write packets and frames to an io.Writer.
type Writer struct {
	empty uint32
	w     io.Writer
	size  int
	mu    sync.Mutex
	buf   []byte
}

// NewWriter returns a Writer that will attempt to buffer size data before
// sending it to the io.Writer.
func NewWriter(w io.Writer, size int) *Writer {
	if size == 0 {
		size = 4 * 1024
	}

	return &Writer{
		w:    w,
		size: size,
		buf:  make([]byte, 0, size),
	}
}

func (b *Writer) log(what string, cb func() string) {
	if drpcdebug.Enabled {
		drpcdebug.Log(func() (_, _, _ string) { return fmt.Sprintf("<wri %p>", b), what, cb() })
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

// Empty returns true if there are no bytes buffered in the writer.
func (b *Writer) Empty() bool {
	return atomic.LoadUint32(&b.empty) == 0
}

// Reset clears any pending data in the buffer.
func (b *Writer) Reset() *Writer {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf = b.buf[:0]
	atomic.StoreUint32(&b.empty, 0)
	return b
}

// WriteFrame appends the frame into the buffer, and if the buffer is larger
// than the configured size, flushes it.
func (b *Writer) WriteFrame(fr Frame) (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.buf) == 0 {
		atomic.StoreUint32(&b.empty, 1)
	}
	b.buf = AppendFrame(b.buf, fr)
	if len(b.buf) >= b.size {
		b.log("FLUSH", func() string { return fmt.Sprintf("buffer: %d > %d", len(b.buf), b.size) })
		_, err = b.w.Write(b.buf)
		b.buf = b.buf[:0]
		atomic.StoreUint32(&b.empty, 0)
	}
	return err
}

// Flush forces a flush of any buffered data to the io.Writer. It is a no-op if
// there is no data in the buffer.
func (b *Writer) Flush() (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.buf) > 0 {
		_, err = b.w.Write(b.buf)
		b.log("FLUSH", func() string { return fmt.Sprintf("explicit: %d", len(b.buf)) })
		b.buf = b.buf[:0]
		atomic.StoreUint32(&b.empty, 0)
	}
	return err
}
