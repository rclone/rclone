//go:build unix

package nfsmount

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/cmd/serve/nfs"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfstest"
	"github.com/stretchr/testify/assert"
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
			_, err := nfs.NewHandler(context.Background(), vfs.New(context.Background(), object.MemoryFs, nil), &nfs.Opt)
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

// TestSubpathMount exercises --nfs-mount-path end-to-end: a local-backed
// source has a /sub subdirectory pre-populated, the NFS server exports
// the source, and the client mounts /sub. A file written into the source
// at /sub/hello.txt must be readable through the mountpoint as ./hello.txt.
func TestSubpathMount(t *testing.T) {
	if runtime.GOOS != "darwin" {
		if !commandOK("sudo", "-n", "mount", "--help") {
			t.Skip("Can't run sudo mount without a password")
		}
		if !commandOK("sudo", "-n", "umount", "--help") {
			t.Skip("Can't run sudo umount without a password")
		}
		sudo = true
	}

	// Source filesystem on disk with the expected layout.
	srcDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "sub", "hello.txt"), []byte("world"), 0644))

	ctx := context.Background()
	f, err := fs.NewFs(ctx, srcDir)
	require.NoError(t, err)

	nfs.Opt.HandleCacheDir = t.TempDir()
	require.NoError(t, nfs.Opt.HandleCache.Set("memory"))

	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeOff
	V := vfs.New(ctx, f, &vfsOpt)
	defer V.Shutdown()

	prevPath := mountPath
	mountPath = "/sub"
	defer func() { mountPath = prevPath }()

	mountpoint := t.TempDir()
	opt := mountlib.Options{}
	opt.SetVolumeName("nfs-subpath-test")
	_, unmount, _, err := mount(V, mountpoint, &opt)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, unmount())
	}()

	data, err := os.ReadFile(filepath.Join(mountpoint, "hello.txt"))
	require.NoError(t, err, "hello.txt must be visible at the mount root when mounted on /sub")
	assert.Equal(t, "world", string(data))
}
