// Fallback Alloc and Free for unsupported OSes

//go:build plan9 || js

package mmap

// Alloc allocates size bytes and returns a slice containing them.  If
// the allocation fails it will return with an error.  This is best
// used for allocations which are a multiple of the Pagesize.
func Alloc(size int) ([]byte, error) {
	return make([]byte, size), nil
}

// Free frees buffers allocated by Alloc.  Note it should be passed
// the same slice (not a derived slice) that Alloc returned.  If the
// free fails it will return with an error.
func Free(mem []byte) error {
	return nil
}
