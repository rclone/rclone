//+build !windows,!linux

package local

import "os"

// preAllocate the file for performance reasons
func preAllocate(size int64, out *os.File) error {
	return nil
}
