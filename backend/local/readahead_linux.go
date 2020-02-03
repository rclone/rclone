// +build linux

package local

import (
	"syscall"

	"github.com/rclone/rclone/fs"
)

/*
const (
	readaheadAmount = 50 * 1024 * 1024
	reread    = 25 * 1024 * 1024
	doReahead = true
)
*/

func readahead(file *localOpenFile) {
	readaheadAmount := file.o.fs.opt.ReadaheadAmount
	if readaheadAmount != 0 {
		readaheadMinBuf := file.o.fs.opt.ReadaheadMinBuf
		if file.readAheadTill == 0 || (file.readAheadTill-file.readTill) < readaheadMinBuf {
			r0, _, errno := syscall.Syscall(syscall.SYS_READAHEAD, file.fd.Fd(), file.readAheadTill, readaheadAmount)
			if r0 != 0 {
				fs.Logf(file.o, "failed to execute sys_readahead: %v", errno)
			} else {
				file.readAheadTill += readaheadAmount
			}
		}
	}
}
