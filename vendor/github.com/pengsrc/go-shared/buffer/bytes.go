// Package buffer provides a thin wrapper around a byte slice. Unlike the
// standard library's bytes.BytesBuffer, it supports a portion of the strconv
// package's zero-allocation formatters.
package buffer

import (
	"strconv"
	"time"
)

// BytesBuffer is a thin wrapper around a byte slice. It's intended to be
// pooled, so the only way to construct one is via a BytesBufferPool.
type BytesBuffer struct {
	bs   []byte
	pool BytesBufferPool
}

// Free returns the BytesBuffer to its BytesBufferPool.
// Callers must not retain references to the BytesBuffer after calling Free.
func (b *BytesBuffer) Free() {
	b.pool.put(b)
}

// Len returns the length of the underlying byte slice.
func (b *BytesBuffer) Len() int {
	return len(b.bs)
}

// Cap returns the capacity of the underlying byte slice.
func (b *BytesBuffer) Cap() int {
	return cap(b.bs)
}

// Bytes returns a mutable reference to the underlying byte slice.
func (b *BytesBuffer) Bytes() []byte {
	return b.bs
}

// String returns a string copy of the underlying byte slice.
func (b *BytesBuffer) String() string {
	return string(b.bs)
}

// Reset resets the underlying byte slice. Subsequent writes re-use the slice's
// backing array.
func (b *BytesBuffer) Reset() {
	b.bs = b.bs[:0]
}

// Write implements io.Writer.
func (b *BytesBuffer) Write(bs []byte) (int, error) {
	b.bs = append(b.bs, bs...)
	return len(bs), nil
}

// AppendByte writes a single byte to the BytesBuffer.
func (b *BytesBuffer) AppendByte(v byte) {
	b.bs = append(b.bs, v)
}

// AppendBytes writes bytes to the BytesBuffer.
func (b *BytesBuffer) AppendBytes(bs []byte) {
	b.bs = append(b.bs, bs...)
}

// AppendString writes a string to the BytesBuffer.
func (b *BytesBuffer) AppendString(s string) {
	b.bs = append(b.bs, s...)
}

// AppendInt appends an integer to the underlying buffer (assuming base 10).
func (b *BytesBuffer) AppendInt(i int64) {
	b.bs = strconv.AppendInt(b.bs, i, 10)
}

// AppendUint appends an unsigned integer to the underlying buffer (assuming
// base 10).
func (b *BytesBuffer) AppendUint(i uint64) {
	b.bs = strconv.AppendUint(b.bs, i, 10)
}

// AppendFloat appends a float to the underlying buffer. It doesn't quote NaN
// or +/- Inf.
func (b *BytesBuffer) AppendFloat(f float64, bitSize int) {
	b.bs = strconv.AppendFloat(b.bs, f, 'f', -1, bitSize)
}

// AppendBool appends a bool to the underlying buffer.
func (b *BytesBuffer) AppendBool(v bool) {
	b.bs = strconv.AppendBool(b.bs, v)
}

// AppendTime appends a time to the underlying buffer.
func (b *BytesBuffer) AppendTime(t time.Time, format string) {
	if format == "" {
		b.bs = strconv.AppendInt(b.bs, t.Unix(), 10)
	} else {
		b.bs = t.AppendFormat(b.bs, format)
	}
}

const defaultSize = 1024 // Create 1 KiB buffers by default
