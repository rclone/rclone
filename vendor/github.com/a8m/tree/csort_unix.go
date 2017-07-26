//+build linux openbsd dragonfly android solaris

package tree

import (
	"os"
	"syscall"
)

func CTimeSort(f1, f2 os.FileInfo) bool {
	s1, s2 := f1.Sys().(*syscall.Stat_t), f2.Sys().(*syscall.Stat_t)
	return s1.Ctim.Sec < s2.Ctim.Sec
}
