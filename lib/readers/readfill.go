package readers

import "io"

// ReadFill reads as much data from r into buf as it can
//
// It reads until the buffer is full or r.Read returned an error.
//
// This is io.ReadFull but when you just want as much data as
// possible, not an exact size of block.
func ReadFill(r io.Reader, buf []byte) (n int, err error) {
	var nn int
	for n < len(buf) && err == nil {
		nn, err = r.Read(buf[n:])
		n += nn
	}
	return n, err
}
