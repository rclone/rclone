package api

// BIN protocol helpers

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/lib/readers"
)

// protocol errors
var (
	ErrorPrematureEOF  = errors.New("Premature EOF")
	ErrorInvalidLength = errors.New("Invalid length")
	ErrorZeroTerminate = errors.New("String must end with zero")
)

// BinWriter is a binary protocol writer
type BinWriter struct {
	b *bytes.Buffer // growing byte buffer
	a []byte        // temporary buffer for next varint
}

// NewBinWriter creates a binary protocol helper
func NewBinWriter() *BinWriter {
	return &BinWriter{
		b: new(bytes.Buffer),
		a: make([]byte, binary.MaxVarintLen64),
	}
}

// Bytes returns binary data
func (w *BinWriter) Bytes() []byte {
	return w.b.Bytes()
}

// Reader returns io.Reader with binary data
func (w *BinWriter) Reader() io.Reader {
	return bytes.NewReader(w.b.Bytes())
}

// WritePu16 writes a short as unsigned varint
func (w *BinWriter) WritePu16(val int) {
	if val < 0 || val > 65535 {
		panic(fmt.Sprintf("Invalid UInt16 %v", val))
	}
	w.WritePu64(int64(val))
}

// WritePu32 writes a signed long as unsigned varint
func (w *BinWriter) WritePu32(val int64) {
	if val < 0 || val > 4294967295 {
		panic(fmt.Sprintf("Invalid UInt32 %v", val))
	}
	w.WritePu64(val)
}

// WritePu64 writes an unsigned (actually, signed) long as unsigned varint
func (w *BinWriter) WritePu64(val int64) {
	if val < 0 {
		panic(fmt.Sprintf("Invalid UInt64 %v", val))
	}
	w.b.Write(w.a[:binary.PutUvarint(w.a, uint64(val))])
}

// WriteString writes a zero-terminated string
func (w *BinWriter) WriteString(str string) {
	buf := []byte(str)
	w.WritePu64(int64(len(buf) + 1))
	w.b.Write(buf)
	w.b.WriteByte(0)
}

// Write writes a byte buffer
func (w *BinWriter) Write(buf []byte) {
	w.b.Write(buf)
}

// WriteWithLength writes a byte buffer prepended with its length as varint
func (w *BinWriter) WriteWithLength(buf []byte) {
	w.WritePu64(int64(len(buf)))
	w.b.Write(buf)
}

// BinReader is a binary protocol reader helper
type BinReader struct {
	b     *bufio.Reader
	count *readers.CountingReader
	err   error // keeps the first error encountered
}

// NewBinReader creates a binary protocol reader helper
func NewBinReader(reader io.Reader) *BinReader {
	r := &BinReader{}
	r.count = readers.NewCountingReader(reader)
	r.b = bufio.NewReader(r.count)
	return r
}

// Count returns number of bytes read
func (r *BinReader) Count() uint64 {
	return r.count.BytesRead()
}

// Error returns first encountered error or nil
func (r *BinReader) Error() error {
	return r.err
}

// check() keeps the first error encountered in a stream
func (r *BinReader) check(err error) bool {
	if err == nil {
		return true
	}
	if r.err == nil {
		// keep the first error
		r.err = err
	}
	if err != io.EOF {
		panic(fmt.Sprintf("Error parsing response: %v", err))
	}
	return false
}

// ReadByteAsInt reads a single byte as uint32, returns -1 for EOF or errors
func (r *BinReader) ReadByteAsInt() int {
	if octet, err := r.b.ReadByte(); r.check(err) {
		return int(octet)
	}
	return -1
}

// ReadByteAsShort reads a single byte as uint16, returns -1 for EOF or errors
func (r *BinReader) ReadByteAsShort() int16 {
	if octet, err := r.b.ReadByte(); r.check(err) {
		return int16(octet)
	}
	return -1
}

// ReadIntSpl reads two bytes as little-endian uint16, returns -1 for EOF or errors
func (r *BinReader) ReadIntSpl() int {
	var val uint16
	if r.check(binary.Read(r.b, binary.LittleEndian, &val)) {
		return int(val)
	}
	return -1
}

// ReadULong returns uint64 equivalent of -1 for EOF or errors
func (r *BinReader) ReadULong() uint64 {
	if val, err := binary.ReadUvarint(r.b); r.check(err) {
		return val
	}
	return 0xffffffffffffffff
}

// ReadPu32 returns -1 for EOF or errors
func (r *BinReader) ReadPu32() int64 {
	if val, err := binary.ReadUvarint(r.b); r.check(err) {
		return int64(val)
	}
	return -1
}

// ReadNBytes reads given number of bytes, returns invalid data for EOF or errors
func (r *BinReader) ReadNBytes(len int) []byte {
	buf := make([]byte, len)
	n, err := r.b.Read(buf)
	if r.check(err) {
		return buf
	}
	if n != len {
		r.check(ErrorPrematureEOF)
	}
	return buf
}

// ReadBytesByLength reads buffer length and its bytes
func (r *BinReader) ReadBytesByLength() []byte {
	len := r.ReadPu32()
	if len < 0 {
		r.check(ErrorInvalidLength)
		return []byte{}
	}
	return r.ReadNBytes(int(len))
}

// ReadString reads a zero-terminated string with length
func (r *BinReader) ReadString() string {
	len := int(r.ReadPu32())
	if len < 1 {
		r.check(ErrorInvalidLength)
		return ""
	}
	buf := make([]byte, len-1)
	n, err := r.b.Read(buf)
	if !r.check(err) {
		return ""
	}
	if n != len-1 {
		r.check(ErrorPrematureEOF)
		return ""
	}
	zeroByte, err := r.b.ReadByte()
	if !r.check(err) {
		return ""
	}
	if zeroByte != 0 {
		r.check(ErrorZeroTerminate)
		return ""
	}
	return string(buf)
}

// ReadDate reads a Unix encoded time
func (r *BinReader) ReadDate() time.Time {
	return time.Unix(r.ReadPu32(), 0)
}
