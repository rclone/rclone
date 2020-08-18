//+build linux

package file

import (
	"os"
	"sync/atomic"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

var (
	fallocFlags = [...]uint32{
		unix.FALLOC_FL_KEEP_SIZE,                             // Default
		unix.FALLOC_FL_KEEP_SIZE | unix.FALLOC_FL_PUNCH_HOLE, // for ZFS #3066
	}
	fallocFlagsIndex int32
)

// PreallocateImplemented is a constant indicating whether the
// implementation of Preallocate actually does anything.
const PreallocateImplemented = true

// PreAllocate the file for performance reasons
func PreAllocate(size int64, out *os.File) error {
	if size <= 0 {
		return nil
	}
	index := atomic.LoadInt32(&fallocFlagsIndex)
again:
	if index >= int32(len(fallocFlags)) {
		return nil // Fallocate is disabled
	}
	flags := fallocFlags[index]
	err := unix.Fallocate(int(out.Fd()), flags, 0, size)
	if err == unix.ENOTSUP {
		// Try the next flags combination
		index++
		atomic.StoreInt32(&fallocFlagsIndex, index)
		fs.Debugf(nil, "preAllocate: got error on fallocate, trying combination %d/%d: %v", index, len(fallocFlags), err)
		goto again

	}
	// FIXME could be doing something here
	// if err == unix.ENOSPC {
	// 	log.Printf("No space")
	// }
	return err
}

// SetSparseImplemented is a constant indicating whether the
// implementation of SetSparse actually does anything.
const SetSparseImplemented = false

// SetSparse makes the file be a sparse file
func SetSparse(out *os.File) error {
	return nil
}
