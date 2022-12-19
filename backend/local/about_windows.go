//go:build windows
// +build windows

package local

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/rclone/rclone/fs"
)

var getFreeDiskSpace = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var available, total, free int64
	root, e := syscall.UTF16PtrFromString(f.root)
	if e != nil {
		return nil, fmt.Errorf("failed to read disk usage: %w", e)
	}
	_, _, e1 := getFreeDiskSpace.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&available)), // lpFreeBytesAvailable - for this user
		uintptr(unsafe.Pointer(&total)),     // lpTotalNumberOfBytes
		uintptr(unsafe.Pointer(&free)),      // lpTotalNumberOfFreeBytes
	)
	if e1 != syscall.Errno(0) {
		return nil, fmt.Errorf("failed to read disk usage: %w", e1)
	}
	usage := &fs.Usage{
		Total: fs.NewUsageValue(total),        // quota of bytes that can be used
		Used:  fs.NewUsageValue(total - free), // bytes in use
		Free:  fs.NewUsageValue(available),    // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// check interface
var _ fs.Abouter = &Fs{}
