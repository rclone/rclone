// Run the more functional vfstest package on the vfs

package vfs_test

import (
	"testing"

	_ "github.com/rclone/rclone/backend/all" // import all the backends
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfstest"
)

// TestExt runs more functional tests all the tests against all the
// VFS cache modes
func TestFunctional(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skip on non local")
	}
	vfstest.RunTests(t, true, func(f fs.Fs, mountpoint string) (VFS *vfs.VFS, unmountResult <-chan error, unmount func() error, err error) {
		unmountResultChan := make(chan (error), 1)
		unmount = func() error {
			unmountResultChan <- nil
			return nil
		}
		VFS = vfs.New(f, nil)
		return VFS, unmountResultChan, unmount, nil
	})
}
