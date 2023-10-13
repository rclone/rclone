// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package file

import (
	"os"
	"syscall"
)

func getInfo(info os.FileInfo) *FileInfo {
	fi := &FileInfo{}
	if s, ok := info.Sys().(*syscall.Stat_t); ok {
		fi.Nlink = uint32(s.Nlink)
		fi.UID = s.Uid
		fi.GID = s.Gid
		return fi
	}
	return nil
}
