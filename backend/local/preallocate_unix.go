//+build linux

package local

import (
	"os"

	"golang.org/x/sys/unix"
)

// preAllocate the file for performance reasons
func preAllocate(size int64, out *os.File) error {
	if size <= 0 {
		return nil
	}
	err := unix.Fallocate(int(out.Fd()), unix.FALLOC_FL_KEEP_SIZE, 0, size)
	// FIXME could be doing something here
	// if err == unix.ENOSPC {
	// 	log.Printf("No space")
	// }
	return err
}
