//go:build windows

package diskusage

import (
	"golang.org/x/sys/windows"
)

// New returns the disk status for dir.
//
// May return Unsupported error if it doesn't work on this platform.
func New(dir string) (info Info, err error) {
	dir16 := windows.StringToUTF16Ptr(dir)
	err = windows.GetDiskFreeSpaceEx(dir16, &info.Available, &info.Total, &info.Free)
	return info, err
}
