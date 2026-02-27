//go:build plan9

package local

import (
	"github.com/rclone/rclone/fs"
	"os"
	"syscall"
)

// https://cs.opensource.google/go/go/+/master:src/os/types_plan9.go
type Plan9HLinkInfo struct {
	path  uint64
	dev   uint32
	ftype uint8
}

func getHLinkInfo(path string, info os.FileInfo) any {
	st, ok := info.Sys().(*syscall.Dir)

	if !ok {
		fs.Debugf(nil, "didn't return Stat_t as expected")
		return nil
	}

	return Plan9HLinkInfo{
		path:  st.Qid.Path,
		dev:   st.Dev,
		ftype: st.Qid.Type,
	}
}
