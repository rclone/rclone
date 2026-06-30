package mountlib_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/cmd/cmount"
	_ "github.com/rclone/rclone/cmd/mount"
	_ "github.com/rclone/rclone/cmd/mount2"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest/testy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRc(t *testing.T) {
	// Disable tests under macOS and the CI since they are locking up
	if runtime.GOOS == "darwin" {
		testy.SkipUnreliable(t)
	}
	ctx := context.Background()
	configfile.Install()
	mount := rc.Calls.Get("mount/mount")
	assert.NotNil(t, mount)
	unmount := rc.Calls.Get("mount/unmount")
	assert.NotNil(t, unmount)
	getMountTypes := rc.Calls.Get("mount/types")
	assert.NotNil(t, getMountTypes)

	localDir := t.TempDir()
	err := os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("hello"), 0666)
	require.NoError(t, err)

	mountPoint := t.TempDir()
	if runtime.GOOS == "windows" {
		// Windows requires the mount point not to exist
		require.NoError(t, os.RemoveAll(mountPoint))
	}

	out, err := getMountTypes.Fn(ctx, nil)
	require.NoError(t, err)
	var mountTypes []string

	err = out.GetStruct("mountTypes", &mountTypes)
	require.NoError(t, err)
	t.Logf("Mount types %v", mountTypes)

	t.Run("Errors", func(t *testing.T) {
		_, err := mount.Fn(ctx, rc.Params{})
		assert.Error(t, err)

		_, err = mount.Fn(ctx, rc.Params{"fs": "/tmp"})
		assert.Error(t, err)

		_, err = mount.Fn(ctx, rc.Params{"mountPoint": "/tmp"})
		assert.Error(t, err)
	})

	t.Run("Mount", func(t *testing.T) {
		if len(mountTypes) == 0 {
			t.Skip("Can't mount")
		}
		in := rc.Params{
			"fs":         localDir,
			"mountPoint": mountPoint,
			"vfsOpt": rc.Params{
				"FilePerms": 0400,
			},
		}

		// check file.txt is not there
		filePath := filepath.Join(mountPoint, "file.txt")
		_, err := os.Stat(filePath)
		require.Error(t, err)
		require.True(t, os.IsNotExist(err))

		// mount
		out, err := mount.Fn(ctx, in)
		if err != nil {
			t.Skipf("Mount failed - skipping test: %v", err)
		}

		// check the returned mount point matches what we asked for
		returnedMountPoint, err := out.GetString("mountPoint")
		require.NoError(t, err)
		assert.Equal(t, mountPoint, returnedMountPoint)

		// check file.txt is there now
		fi, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.Equal(t, int64(5), fi.Size())
		if runtime.GOOS == "linux" {
			assert.Equal(t, os.FileMode(0400), fi.Mode())
		}

		// check mount point list
		checkMountList := func() []mountlib.MountInfo {
			listCall := rc.Calls.Get("mount/listmounts")
			require.NotNil(t, listCall)
			listReply, err := listCall.Fn(ctx, rc.Params{})
			require.NoError(t, err)
			mountPointsReply, err := listReply.Get("mountPoints")
			require.NoError(t, err)
			mountPoints, ok := mountPointsReply.([]mountlib.MountInfo)
			require.True(t, ok)
			return mountPoints
		}
		mountPoints := checkMountList()
		require.Equal(t, 1, len(mountPoints))
		require.Equal(t, mountPoint, mountPoints[0].MountPoint)

		// FIXME the OS sometimes appears to be using the mount
		// immediately after it appears so wait a moment
		time.Sleep(100 * time.Millisecond)

		t.Run("Unmount", func(t *testing.T) {
			_, err := unmount.Fn(ctx, in)
			require.NoError(t, err)
			assert.Equal(t, 0, len(checkMountList()))
		})
	})
}

func TestRcFlatOptions(t *testing.T) {
	// Disable tests under macOS and the CI since they are locking up
	if runtime.GOOS == "darwin" {
		testy.SkipUnreliable(t)
	}
	ctx := context.Background()
	configfile.Install()
	mount := rc.Calls.Get("mount/mount")
	assert.NotNil(t, mount)
	unmount := rc.Calls.Get("mount/unmount")
	assert.NotNil(t, unmount)
	getMountTypes := rc.Calls.Get("mount/types")
	assert.NotNil(t, getMountTypes)

	localDir := t.TempDir()
	err := os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("hello"), 0666)
	require.NoError(t, err)

	out, err := getMountTypes.Fn(ctx, nil)
	require.NoError(t, err)
	var mountTypes []string
	err = out.GetStruct("mountTypes", &mountTypes)
	require.NoError(t, err)
	if len(mountTypes) == 0 {
		t.Skip("Can't mount")
	}

	mountPointFlat := t.TempDir()
	if runtime.GOOS == "windows" {
		require.NoError(t, os.RemoveAll(mountPointFlat))
	}

	in := rc.Params{
		"fs":         localDir,
		"mountPoint": mountPointFlat,
		"file_perms": 0400,           // flat VFS option
		"volname":    "MyTestVolume", // flat Mount option
	}

	// mount
	out, err = mount.Fn(ctx, in)
	if err != nil {
		t.Skipf("Mount failed - skipping test: %v", err)
	}

	// check the returned mount point matches what we asked for
	returnedMountPoint, err := out.GetString("mountPoint")
	require.NoError(t, err)
	assert.Equal(t, mountPointFlat, returnedMountPoint)

	// check that the flat options were consumed and removed from parameter map
	_, ok := in["file_perms"]
	assert.False(t, ok, "file_perms flat option should have been deleted")
	_, ok = in["volname"]
	assert.False(t, ok, "volname flat option should have been deleted")

	// unmount
	_, err = unmount.Fn(ctx, rc.Params{
		"mountPoint": mountPointFlat,
	})
	require.NoError(t, err)

	// FIXME wait a moment for the OS to release the mount point
	time.Sleep(100 * time.Millisecond)
}

func TestRcFlatOptionsNull(t *testing.T) {
	ctx := context.Background()
	configfile.Install()
	mount := rc.Calls.Get("mount/mount")
	assert.NotNil(t, mount)

	in := rc.Params{
		"fs":             "some_fs",
		"mountPoint":     "some_mount_point",
		"vfs_cache_mode": nil, // flat VFS option set to null
	}

	_, err := mount.Fn(ctx, in)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "interpreting <nil> as string failed")
}
