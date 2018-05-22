package mmap

import "os"

// PageSize is the minimum allocation size.  Allocations will use at
// least this size and are likely to be multiplied up to a multiple of
// this size.
var PageSize = os.Getpagesize()

// MustAlloc allocates size bytes and returns a slice containing them.  If
// the allocation fails it will panic.  This is best used for
// allocations which are a multiple of the PageSize.
func MustAlloc(size int) []byte {
	mem, err := Alloc(size)
	if err != nil {
		panic(err)
	}
	return mem
}

// MustFree frees buffers allocated by Alloc.  Note it should be passed
// the same slice (not a derived slice) that Alloc returned.  If the
// free fails it will panic.
func MustFree(mem []byte) {
	err := Free(mem)
	if err != nil {
		panic(err)
	}
}
