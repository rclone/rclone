// TestFunctional runs more functional tests all the tests against all the
// VFS cache modes
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
	// Run the tests with all the VFS cache modes
	vfstest.RunTests(t, true, vfscommon.CacheModeOff, true, mountlib.MountFn(func(vfsInst *vfs.VFS, mountpoint string, opt *mountlib.Options) (unmountResult <-chan error, unmount func() error, err error) {
		// This function is called by RunTests to mount the filesystem.  It is
		// called for each cache mode.
		//
		// It needs to return a channel which will receive the unmount result,
		// a function to unmount the filesystem and an error.
		//
		// We create a channel to receive the unmount result to satisfy the
		// interface.
		unmountResultChan := make(chan error, 1)
		// Create a function to unmount the filesystem.
		//
		// This function is called when the test is finished. We don't actually
		// want to unmount the filesystem as we are using the in-process vfs
		// mounter, so we just send a nil error to signal unmount completion.
		unmount = func() error {
			// Send a nil error to signal unmount complete
			unmountResultChan <- nil
			// Return no error
			return nil
		}
		// Return the channel, the unmount function, and no error
		return unmountResultChan, unmount, nil
	}))
}
