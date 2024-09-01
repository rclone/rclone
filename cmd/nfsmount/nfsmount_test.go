//go:build unix

package nfsmount

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/rclone/rclone/cmd/serve/nfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfstest"
	"github.com/stretchr/testify/require"
)

// Return true if the command ran without error
func commandOK(name string, arg ...string) bool {
	cmd := exec.Command(name, arg...)
	_, err := cmd.CombinedOutput()
	return err == nil
}

func TestMount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		if !commandOK("sudo", "-n", "mount", "--help") {
			t.Skip("Can't run sudo mount without a password")
		}
		if !commandOK("sudo", "-n", "umount", "--help") {
			t.Skip("Can't run sudo umount without a password")
		}
		sudo = true
	}
	nfs.Opt.HandleCacheDir = t.TempDir()
	require.NoError(t, nfs.Opt.HandleCache.Set("disk"))
	vfstest.RunTests(t, false, vfscommon.CacheModeWrites, false, mount)
}
