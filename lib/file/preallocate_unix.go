//go:build linux

package file

import (
	"os"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/unix"
)

var (
	fallocFlags = [...]uint32{
		0,                             // Default
	}
	fallocFlagsIndex atomic.Int32
	preAllocateMu    sync.Mutex
)

// PreallocateImplemented is a constant indicating whether the
// implementation of Preallocate actually does anything.
const PreallocateImplemented = true

// Whether or not to preallocate files as sparse
func PreAllocateAdvise(allocateSparse bool) {
	if allocateSparse {
		fallocFlagsIndex.Store(1)
	} else {
		fallocFlagsIndex.Store(0)
	}
}

// PreAllocate the file for performance reasons
func PreAllocate(size int64, out *os.File) (err error) {
	if size <= 0 {
		return nil
	}

	preAllocateMu.Lock()
	defer preAllocateMu.Unlock()

	for {

		index := fallocFlagsIndex.Load()
	again:
		if index >= int32(len(fallocFlags)+1) {
			return nil // Fallocate is disabled
		}
		if index == int32(len(fallocFlags)) {
			// use ftruncate
			err = unix.Ftruncate(int(out.Fd()), size)
			return err
		} else {
			flags := fallocFlags[index]
			err = unix.Fallocate(int(out.Fd()), flags, 0, size)
			if err == unix.ENOTSUP {
				// Try the next flags combination
				index++
				fallocFlagsIndex.Store(index)
				fs.Debugf(nil, "preAllocate: got error on fallocate, trying combination %d/%d: %v", index, len(fallocFlags), err)
				goto again

			}
		}
		// Wrap important errors
		if err == unix.ENOSPC {
			return ErrDiskFull
		}
		if err != syscall.EINTR {
			break
		}
	}
	return err
}

// SetSparseImplemented is a constant indicating whether the
// implementation of SetSparse actually does anything.
const SetSparseImplemented = false

// SetSparse makes the file be a sparse file
func SetSparse(out *os.File) error {
	return nil
}
