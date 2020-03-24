//+build !windows,!linux

package file

import "os"

// PreAllocate the file for performance reasons
func PreAllocate(size int64, out *os.File) error {
	return nil
}

// SetSparse makes the file be a sparse file
func SetSparse(out *os.File) error {
	return nil
}
