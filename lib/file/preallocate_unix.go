//go:build linux
// +build linux

package file

import (
	"os"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/rclone/rclone/fs"
)

var (
	fallocFlags = [...]uint32{
		unix.FALLOC_FL_KEEP_SIZE,                             // Default
		unix.FALLOC_FL_KEEP_SIZE | unix.FALLOC_FL_PUNCH_HOLE, // for ZFS #3066
	}

	isDisabled = int32(0)
)

// PreallocateImplemented is a constant indicating whether the
// implementation of Preallocate actually does anything.
const PreallocateImplemented = true

// PreAllocate the file for performance reasons
func PreAllocate(size int64, out *os.File) (err error) {
	if size <= 0 {
		return nil
	}

	index := int32(0)

	if atomic.LoadInt32(&isDisabled) == 1 {
		return nil
	}

	isNotSupported := false

	defer func() {
		if isNotSupported {
			atomic.CompareAndSwapInt32(&isDisabled, 0, 1)
		}
	}()

	for {
	again:
		if index >= int32(len(fallocFlags)) {
			isNotSupported = true
			return nil // Fallocate is disabled
		}
		flags := fallocFlags[index]
		err = unix.Fallocate(int(out.Fd()), flags, 0, size)
		if err == unix.ENOTSUP {
			// Try the next flags combination
			index++
			fs.Debugf(nil, "preAllocate: got error on fallocate, trying combination %d/%d: %v", index, len(fallocFlags), err)
			goto again

		}
		// Wrap important errors
		if err == unix.ENOSPC {
			isNotSupported = true
			return ErrDiskFull
		}
		if err != syscall.EINTR {
			isNotSupported = true
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
