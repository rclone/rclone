//go:build windows

package file

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ntdll                        = windows.NewLazySystemDLL("ntdll.dll")
	ntQueryVolumeInformationFile = ntdll.NewProc("NtQueryVolumeInformationFile")
	ntSetInformationFile         = ntdll.NewProc("NtSetInformationFile")
	preAllocateMu                sync.Mutex
)

type fileAllocationInformation struct {
	AllocationSize uint64
}

type fileFsSizeInformation struct {
	TotalAllocationUnits     uint64
	AvailableAllocationUnits uint64
	SectorsPerAllocationUnit uint32
	BytesPerSector           uint32
}

type ioStatusBlock struct {
	Status, Information uintptr
}

// PreallocateImplemented is a constant indicating whether the
// implementation of Preallocate actually does anything.
const PreallocateImplemented = true

func PreAllocateAdvise(allocateSparse bool) {}

// PreAllocate the file for performance reasons
func PreAllocate(size int64, out *os.File) error {
	if size <= 0 {
		return nil
	}

	preAllocateMu.Lock()
	defer preAllocateMu.Unlock()

	var (
		iosb       ioStatusBlock
		fsSizeInfo fileFsSizeInformation
		allocInfo  fileAllocationInformation
	)

	// Query info about the block sizes on the file system
	_, _, e1 := ntQueryVolumeInformationFile.Call(
		out.Fd(),
		uintptr(unsafe.Pointer(&iosb)),
		uintptr(unsafe.Pointer(&fsSizeInfo)),
		unsafe.Sizeof(fsSizeInfo),
		uintptr(3), // FileFsSizeInformation
	)
	if e1 != nil && e1 != syscall.Errno(0) {
		return fmt.Errorf("preAllocate NtQueryVolumeInformationFile failed: %w", e1)
	}

	// Calculate the allocation size
	clusterSize := uint64(fsSizeInfo.BytesPerSector) * uint64(fsSizeInfo.SectorsPerAllocationUnit)
	if clusterSize <= 0 {
		return fmt.Errorf("preAllocate clusterSize %d <= 0", clusterSize)
	}
	allocInfo.AllocationSize = (1 + uint64(size-1)/clusterSize) * clusterSize

	// Ask for the allocation
	_, _, e1 = ntSetInformationFile.Call(
		out.Fd(),
		uintptr(unsafe.Pointer(&iosb)),
		uintptr(unsafe.Pointer(&allocInfo)),
		unsafe.Sizeof(allocInfo),
		uintptr(19), // FileAllocationInformation
	)
	if e1 != nil && e1 != syscall.Errno(0) {
		if e1 == windows.ERROR_DISK_FULL || e1 == windows.ERROR_HANDLE_DISK_FULL {
			return ErrDiskFull
		}
		return fmt.Errorf("preAllocate NtSetInformationFile failed: %w", e1)
	}

	return nil
}

// SetSparseImplemented is a constant indicating whether the
// implementation of SetSparse actually does anything.
const SetSparseImplemented = true

// SetSparse makes the file be a sparse file
func SetSparse(out *os.File) error {
	var bytesReturned uint32
	err := syscall.DeviceIoControl(syscall.Handle(out.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &bytesReturned, nil)
	if err != nil {
		return fmt.Errorf("DeviceIoControl FSCTL_SET_SPARSE: %w", err)
	}
	return nil
}
