package mountlib_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/cmd/cmount"
	_ "github.com/rclone/rclone/cmd/mount"
	_ "github.com/rclone/rclone/cmd/mount2"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRc(t *testing.T) {
	ctx := context.Background()
	mount := rc.Calls.Get("mount/mount")
	assert.NotNil(t, mount)
	unmount := rc.Calls.Get("mount/unmount")
	assert.NotNil(t, unmount)
	getMountTypes := rc.Calls.Get("mount/types")
	assert.NotNil(t, getMountTypes)

	localDir, err := ioutil.TempDir("", "rclone-mountlib-localDir")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(localDir) }()
	err = ioutil.WriteFile(filepath.Join(localDir, "file.txt"), []byte("hello"), 0666)
	require.NoError(t, err)

	mountPoint, err := ioutil.TempDir("", "rclone-mountlib-mountPoint")
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		// Windows requires the mount point not to exist
		require.NoError(t, os.RemoveAll(mountPoint))
	} else {
		defer func() { _ = os.RemoveAll(mountPoint) }()
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
		_, err = mount.Fn(ctx, in)
		if err != nil {
			t.Skipf("Mount failed - skipping test: %v", err)
		}

		// check file.txt is there now
		fi, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.Equal(t, int64(5), fi.Size())
		if runtime.GOOS == "linux" {
			assert.Equal(t, os.FileMode(0400), fi.Mode())
		}

		// FIXME the OS sometimes appears to be using the mount
		// immediately after it appears so wait a moment
		time.Sleep(100 * time.Millisecond)

		t.Run("Unmount", func(t *testing.T) {
			_, err := unmount.Fn(ctx, in)
			require.NoError(t, err)
		})
	})
}
