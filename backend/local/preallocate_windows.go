//+build windows

package local

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
)

var (
	ntdll                        = windows.NewLazySystemDLL("ntdll.dll")
	ntQueryVolumeInformationFile = ntdll.NewProc("NtQueryVolumeInformationFile")
	ntSetInformationFile         = ntdll.NewProc("NtSetInformationFile")
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

// preAllocate the file for performance reasons
func preAllocate(size int64, out *os.File) error {
	if size <= 0 {
		return nil
	}

	var (
		iosb       ioStatusBlock
		fsSizeInfo fileFsSizeInformation
		allocInfo  fileAllocationInformation
	)

	// Query info about the block sizes on the file system
	_, _, e1 := ntQueryVolumeInformationFile.Call(
		uintptr(out.Fd()),
		uintptr(unsafe.Pointer(&iosb)),
		uintptr(unsafe.Pointer(&fsSizeInfo)),
		uintptr(unsafe.Sizeof(fsSizeInfo)),
		uintptr(3), // FileFsSizeInformation
	)
	if e1 != nil && e1 != syscall.Errno(0) {
		return errors.Wrap(e1, "preAllocate NtQueryVolumeInformationFile failed")
	}

	// Calculate the allocation size
	clusterSize := uint64(fsSizeInfo.BytesPerSector) * uint64(fsSizeInfo.SectorsPerAllocationUnit)
	if clusterSize <= 0 {
		return errors.Errorf("preAllocate clusterSize %d <= 0", clusterSize)
	}
	allocInfo.AllocationSize = (1 + uint64(size-1)/clusterSize) * clusterSize

	// Ask for the allocation
	_, _, e1 = ntSetInformationFile.Call(
		uintptr(out.Fd()),
		uintptr(unsafe.Pointer(&iosb)),
		uintptr(unsafe.Pointer(&allocInfo)),
		uintptr(unsafe.Sizeof(allocInfo)),
		uintptr(19), // FileAllocationInformation
	)
	if e1 != nil && e1 != syscall.Errno(0) {
		return errors.Wrap(e1, "preAllocate NtSetInformationFile failed")
	}

	return nil
}
