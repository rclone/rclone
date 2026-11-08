//go:build darwin

package file

import "os"

// PreallocateImplemented is a constant indicating whether the
// implementation of PreAllocate actually does anything.
const PreallocateImplemented = true

// PreAllocate truncates the file to size, creating a sparse file.
//
// The file size is set to the final size immediately, while the
// allocated blocks grow as data is written, so the progress of a
// transfer can be observed cheaply with stat.blocks.
func PreAllocate(size int64, out *os.File) error {
	if size <= 0 {
		return nil
	}
	return out.Truncate(size)
}

// SetSparseImplemented is a constant indicating whether the
// implementation of SetSparse actually does anything.
const SetSparseImplemented = false

// SetSparse makes the file be a sparse file
func SetSparse(out *os.File) error {
	return nil
}
