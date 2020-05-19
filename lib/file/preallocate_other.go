//+build !windows,!linux

package file

import "os"

// PreallocateImplemented is a constant indicating whether the
// implementation of Preallocate actually does anything.
const PreallocateImplemented = false

// PreAllocate the file for performance reasons
func PreAllocate(size int64, out *os.File) error {
	return nil
}

// SetSparseImplemented is a constant indicating whether the
// implementation of SetSparse actually does anything.
const SetSparseImplemented = false

// SetSparse makes the file be a sparse file
func SetSparse(out *os.File) error {
	return nil
}
