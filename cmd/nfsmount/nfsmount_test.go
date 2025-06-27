//go:build unix

package nfsmount

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/rclone/rclone/cmd/serve/nfs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/vfs"
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
	for _, cacheType := range []string{"memory", "disk", "symlink"} {
		t.Run(cacheType, func(t *testing.T) {
			nfs.Opt.HandleCacheDir = t.TempDir()
			require.NoError(t, nfs.Opt.HandleCache.Set(cacheType))
			// Check we can create a handler
			_, err := nfs.NewHandler(context.Background(), vfs.New(object.MemoryFs, nil), &nfs.Opt)
			if errors.Is(err, nfs.ErrorSymlinkCacheNotSupported) || errors.Is(err, nfs.ErrorSymlinkCacheNoPermission) {
				t.Skip(err.Error() + ": run with: go test -c && sudo setcap cap_dac_read_search+ep ./nfsmount.test && ./nfsmount.test -test.v")
			}
			require.NoError(t, err)
			// Configure rclone via environment var since the mount gets run in a subprocess
			_ = os.Setenv("RCLONE_NFS_CACHE_DIR", nfs.Opt.HandleCacheDir)
			_ = os.Setenv("RCLONE_NFS_CACHE_TYPE", cacheType)
			t.Cleanup(func() {
				_ = os.Unsetenv("RCLONE_NFS_CACHE_DIR")
				_ = os.Unsetenv("RCLONE_NFS_CACHE_TYPE")
			})
			vfstest.RunTests(t, false, vfscommon.CacheModeWrites, false, mount)
		})
	}
}
