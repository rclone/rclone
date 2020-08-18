// Package mmap implements a large block memory allocator using
// anonymous memory maps.

// +build !plan9,!windows,!js

package mmap

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Alloc allocates size bytes and returns a slice containing them.  If
// the allocation fails it will return with an error.  This is best
// used for allocations which are a multiple of the PageSize.
func Alloc(size int) ([]byte, error) {
	mem, err := unix.Mmap(-1, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANON)
	if err != nil {
		return nil, errors.Wrap(err, "mmap: failed to allocate memory for buffer")
	}
	return mem, nil
}

// Free frees buffers allocated by Alloc.  Note it should be passed
// the same slice (not a derived slice) that Alloc returned.  If the
// free fails it will return with an error.
func Free(mem []byte) error {
	err := unix.Munmap(mem)
	if err != nil {
		return errors.Wrap(err, "mmap: failed to unmap memory")
	}
	return nil
}
