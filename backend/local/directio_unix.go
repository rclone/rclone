//go:build linux

package local

import (
	"syscall"
	"unsafe"
)

const (
	directIOFlag    = syscall.O_DIRECT
	directIOAlign   = 4096
	directIOEnabled = true
)

// alignedBlock allocates a block of size bytes aligned to directIOAlign.
func alignedBlock(size int) []byte {
	block := make([]byte, size+directIOAlign)
	a := alignment(block)
	offset := 0
	if a != 0 {
		offset = directIOAlign - a
	}
	return block[offset : offset+size]
}

// alignment returns the alignment of the block start
func alignment(block []byte) int {
	return int(uintptr(unsafe.Pointer(&block[0])) & uintptr(directIOAlign-1))
}
