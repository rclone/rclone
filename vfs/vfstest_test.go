package vfs_test

import (
	"testing"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfstest"
)

// If the remote name is set, then skip this test as it is not local
func TestFunctional(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skip on non local")
	}
	vfstest.RunTests(t, true, vfscommon.CacheModeOff, true, mountlib.MountFn(func(vfsInst *vfs.VFS, mountpoint string, opt *mountlib.Options) (unmountResult <-chan error, unmount func() error, err error) {
		unmountResultChan := make(chan error, 1)
		unmount = func() error {
			unmountResultChan <- nil
			return nil
		}
		return unmountResultChan, unmount, nil
	}))
}