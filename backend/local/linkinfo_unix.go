//go:build !windows && !plan9

package local

import (
	"github.com/rclone/rclone/fs"
	"os"
	"syscall"
)

// https://cs.opensource.google/go/go/+/master:src/os/types_unix.go
type UnixHLinkInfo struct {
	dev uint64
	ino uint64
}

func getHLinkInfo(path string, info os.FileInfo) any {
	st, ok := info.Sys().(*syscall.Stat_t)

	if !ok {
		fs.Debugf(nil, "didn't return Stat_t as expected")
		return nil
	}

	return UnixHLinkInfo{
		dev: st.Dev,
		ino: st.Ino,
	}
}
