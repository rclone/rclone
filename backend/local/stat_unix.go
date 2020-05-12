package local

import (
	"os"
	"syscall"
	"time"
)

func stat(path string) (mode os.FileMode, mTime time.Time, aTime time.Time, err error) {
	stat, err := os.Stat(path)
	zeroTime := time.Unix(0, 0)
	if err != nil {
		return os.FileMode(int(0000)), zeroTime, zeroTime, err
	}
	mode = stat.Mode()
	si := stat.Sys().(*syscall.Stat_t)
	mTime = stat.ModTime()
	aTime = time.Unix(int64(si.Atimespec.Sec), int64(si.Atimespec.Nsec))
	return mode, mTime, aTime, nil
}
