// +build !windows

package sftp

import (
	"errors"
	"syscall"
)

func fakeFileInfoSys() interface{} {
	return &syscall.Stat_t{Uid: 65534, Gid: 65534}
}

func testOsSys(sys interface{}) error {
	fstat := sys.(*FileStat)
	if fstat.UID != uint32(65534) {
		return errors.New("Uid failed to match.")
	}
	if fstat.GID != uint32(65534) {
		return errors.New("Gid failed to match:")
	}
	return nil
}
