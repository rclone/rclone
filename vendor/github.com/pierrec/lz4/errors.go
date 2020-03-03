package lz4

import (
	"errors"
	"fmt"
	"os"
	rdebug "runtime/debug"
)

var (
	// ErrInvalidSourceShortBuffer is returned by UncompressBlock or CompressBLock when a compressed
	// block is corrupted or the destination buffer is not large enough for the uncompressed data.
	ErrInvalidSourceShortBuffer = errors.New("lz4: invalid source or destination buffer too short")
	// ErrInvalid is returned when reading an invalid LZ4 archive.
	ErrInvalid = errors.New("lz4: bad magic number")
	// ErrBlockDependency is returned when attempting to decompress an archive created with block dependency.
	ErrBlockDependency = errors.New("lz4: block dependency not supported")
	// ErrUnsupportedSeek is returned when attempting to Seek any way but forward from the current position.
	ErrUnsupportedSeek = errors.New("lz4: can only seek forward from io.SeekCurrent")
)

func recoverBlock(e *error) {
	if r := recover(); r != nil && *e == nil {
		if debugFlag {
			fmt.Fprintln(os.Stderr, r)
			rdebug.PrintStack()
		}
		*e = ErrInvalidSourceShortBuffer
	}
}
