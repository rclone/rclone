//go:build windows

package local

import (
	"github.com/rclone/rclone/fs"
	"os"
	"syscall"
)

// https://cs.opensource.google/go/go/+/master:src/os/types_windows.go
type WindowsHLinkInfo struct {
	vol   uint32
	idxhi uint32
	idxlo uint32
}

// Load file id information (see WindowsLinkInfo)
// In theory, we *could* use reflection to retreive this information from the private
// fileStat struct in the os package. However, this is probably slow, so we'll use a duplicate
// syscall
func loadFileId(path string, info *WindowsHLinkInfo) error {
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	// Per https://learn.microsoft.com/en-us/windows/win32/fileio/reparse-points-and-file-operations,
	// “Applications that use the CreateFile function should specify the
	// FILE_FLAG_OPEN_REPARSE_POINT flag when opening the file if it is a reparse
	// point.”
	//
	// And per https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew,
	// “If the file is not a reparse point, then this flag is ignored.”
	//
	// So we set FILE_FLAG_OPEN_REPARSE_POINT unconditionally, since we want
	// information about the reparse point itself.
	//
	// If the file is a symlink, the symlink target should have already been
	// resolved when the fileStat was created, so we don't need to worry about
	// resolving symlink reparse points again here.
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS | syscall.FILE_FLAG_OPEN_REPARSE_POINT)

	h, err := syscall.CreateFile(pathp, 0, 0, nil, syscall.OPEN_EXISTING, attrs, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	var i syscall.ByHandleFileInformation
	err = syscall.GetFileInformationByHandle(h, &i)
	if err != nil {
		return err
	}
	info.vol = i.VolumeSerialNumber
	info.idxhi = i.FileIndexHigh
	info.idxlo = i.FileIndexLow
	return nil
}

func getHLinkInfo(path string, info os.FileInfo) any {
	// It would be amazing to implement this without another syscall, but for some reason the golang os api doesn't expose the volume and file id information
	// https://cs.opensource.google/go/go/+/master:src/os/types_windows.go;l=36-41;drc=dc548bb322387039a12000c04e5b9083a0511639

	var li WindowsHLinkInfo
	err := loadFileId(path, &li)
	if err != nil {
		fs.Debugf(nil, "loadFileId didn't return as expected: %v")
		return nil // for conformance with the other platform variants of this function
	}

	return li
}
