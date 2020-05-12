package local

import (
	"os"
	"syscall"
	"time"
)

func stat(path string) (mode os.FileMode, mTime time.Time, aTime time.Time, err error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, nil, nil, err
	}
	mode = stat.Mode()
	mTime = time.Unix(0, stat.Sys().(*syscall.Win32FileAttributeData).LastWriteTime.Nanoseconds())
	aTime = time.Unix(0, stat.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds())
	return mode, mTime, aTime, cTime, nil
}
