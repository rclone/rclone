package vfstest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileForgetGrown checks that a file grown out of band is seen at its new
// size once forgotten. See https://github.com/rclone/rclone/issues/9617
func TestFileForgetGrown(t *testing.T) {
	run.skipIfVFS(t)
	run.skipIfNoFUSE(t)
	if !run.hasInvalidateKernelCacheHooks() {
		t.Skip("mount backend doesn't invalidate the kernel cache on forget")
	}
	if run.vfsOpt.CacheMode != vfscommon.CacheModeOff {
		t.Skip("only meaningful with --vfs-cache-mode off - reads go straight to the remote")
	}

	const (
		short = "small"
		long  = "much longer contents than the original file had"
	)

	// Create through the mount and stat it so the kernel caches the short size.
	run.createFile(t, "file", short)
	fi, err := run.os.Stat(run.path("file"))
	require.NoError(t, err)
	require.Equal(t, int64(len(short)), fi.Size())

	// Grow the object out of band, directly on the remote.
	src := object.NewStaticObjectInfo("file", time.Now(), int64(len(long)), true, nil, nil)
	_, err = run.fremote.Put(context.Background(), strings.NewReader(long), src)
	require.NoError(t, err)

	run.forgetFile("file")

	// The kernel must now report the new size before the attr-timeout
	deadline := time.Now().Add(time.Duration(mountlib.Opt.AttrTimeout) * 3 / 4)
	var size int64
	for {
		fi, err = run.os.Stat(run.path("file"))
		require.NoError(t, err)
		if size = fi.Size(); size == int64(len(long)) || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, int64(len(long)), size)

	run.rm(t, "file")
}
