package file

import "os"

type FileInfo struct {
	Nlink uint32
	UID   uint32
	GID   uint32
}

// GetInfo extracts some non-standardized items from the result of a Stat call.
func GetInfo(fi os.FileInfo) *FileInfo {
	return getInfo(fi)
}
