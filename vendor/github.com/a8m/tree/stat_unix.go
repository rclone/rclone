//+build !plan9,!windows

package tree

import (
	"os"
	"syscall"
)

func getStat(fi os.FileInfo) (ok bool, inode, device, uid, gid uint64) {
	sys := fi.Sys()
	if sys == nil {
		return false, 0, 0, 0, 0
	}
	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return false, 0, 0, 0, 0
	}
	return true, uint64(stat.Ino), uint64(stat.Dev), uint64(stat.Uid), uint64(stat.Gid)
}
