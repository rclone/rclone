//go:build !linux

package local

import (
	"io"
	"os"
)

const (
	directIOFlag    = 0
	directIOEnabled = false
)

// alignedBlock allocates a regular block on unsupported platforms.
func alignedBlock(size int) []byte {
	return make([]byte, size)
}

// directIOCopy is a stub for unsupported platforms.
// It is never called because directIOEnabled is false.
func directIOCopy(dst *os.File, src io.Reader, buf []byte) (written int64, err error) {
	panic("directIOCopy called on unsupported platform")
}
