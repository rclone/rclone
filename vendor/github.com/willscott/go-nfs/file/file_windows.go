// +build windows

package file

import "os"

func getInfo(info os.FileInfo) *FileInfo {
	// https://godoc.org/golang.org/x/sys/windows#GetFileInformationByHandle
	// can be potentially used to populate Nlink

	return nil
}
