// Package mmap implements a large block memory allocator using
// anonymous memory maps.

// +build windows

package mmap

import (
	"reflect"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

// Alloc allocates size bytes and returns a slice containing them.  If
// the allocation fails it will return with an error.  This is best
// used for allocations which are a multiple of the PageSize.
func Alloc(size int) ([]byte, error) {
	p, err := windows.VirtualAlloc(0, uintptr(size), windows.MEM_COMMIT, windows.PAGE_READWRITE)
	if err != nil {
		return nil, errors.Wrap(err, "mmap: failed to allocate memory for buffer")
	}
	var mem []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&mem))
	sh.Data = p
	sh.Len = size
	sh.Cap = size
	return mem, nil
}

// Free frees buffers allocated by Alloc.  Note it should be passed
// the same slice (not a derived slice) that Alloc returned.  If the
// free fails it will return with an error.
func Free(mem []byte) error {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&mem))
	err := windows.VirtualFree(sh.Data, 0, windows.MEM_RELEASE)
	if err != nil {
		return errors.Wrap(err, "mmap: failed to unmap memory")
	}
	return nil
}
