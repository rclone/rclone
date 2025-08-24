// Package mmap implements a large block memory allocator using
// anonymous memory maps.

//go:build windows

package mmap

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Alloc allocates size bytes and returns a slice containing them.  If
// the allocation fails it will return with an error.  This is best
// used for allocations which are a multiple of the PageSize.
func Alloc(size int) ([]byte, error) {
	p, err := windows.VirtualAlloc(0, uintptr(size), windows.MEM_COMMIT, windows.PAGE_READWRITE)
	if err != nil {
		return nil, fmt.Errorf("mmap: failed to allocate memory for buffer: %w", err)
	}

	pp := unsafe.Pointer(&p)
	up := *(*unsafe.Pointer)(pp)
	return unsafe.Slice((*byte)(up), size), nil
}

// Free frees buffers allocated by Alloc.  Note it should be passed
// the same slice (not a derived slice) that Alloc returned.  If the
// free fails it will return with an error.
func Free(mem []byte) error {
	p := unsafe.SliceData(mem)
	err := windows.VirtualFree(uintptr(unsafe.Pointer(p)), 0, windows.MEM_RELEASE)
	if err != nil {
		return fmt.Errorf("mmap: failed to unmap memory: %w", err)
	}
	return nil
}
