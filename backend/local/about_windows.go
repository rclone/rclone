// +build windows

package local

import (
	"context"
	"syscall"
	"unsafe"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

var getFreeDiskSpace = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var available, total, free int64
	_, _, e1 := getFreeDiskSpace.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(f.root))),
		uintptr(unsafe.Pointer(&available)), // lpFreeBytesAvailable - for this user
		uintptr(unsafe.Pointer(&total)),     // lpTotalNumberOfBytes
		uintptr(unsafe.Pointer(&free)),      // lpTotalNumberOfFreeBytes
	)
	if e1 != syscall.Errno(0) {
		return nil, errors.Wrap(e1, "failed to read disk usage")
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
