//go:build windows

package local

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRmdirWindows tests that FILE_ATTRIBUTE_READONLY does not block Rmdir on windows.
// Microsoft docs indicate that "This attribute is not honored on directories."
// See https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants#file_attribute_readonly
// and https://github.com/golang/go/issues/26295
func TestRmdirWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("windows only")
	}
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := operations.Mkdir(context.Background(), r.Flocal, "testdir")
	require.NoError(t, err)

	ptr, err := syscall.UTF16PtrFromString(filepath.Join(r.Flocal.Root(), "testdir"))
	require.NoError(t, err)

	err = syscall.SetFileAttributes(ptr, uint32(syscall.FILE_ATTRIBUTE_DIRECTORY+syscall.FILE_ATTRIBUTE_READONLY))
	require.NoError(t, err)

	err = operations.Rmdir(context.Background(), r.Flocal, "testdir")
	assert.NoError(t, err)
}

func TestWrapWindowsLongPathErrorWithNoUNC(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("windows only")
	}
	ctx := context.Background()
	fInterface, err := NewFs(ctx, "local", t.TempDir(), configmap.Simple{
		"nounc": "true",
	})
	require.NoError(t, err)
	f := fInterface.(*Fs)

	remote := strings.Repeat("longdir/", 40) + "file.txt"
	localPath := f.localPath(remote)
	require.GreaterOrEqual(t, windowsPathLength(localPath), windowsMaxPath)

	err = f.wrapPathLengthError(localPath, syscall.ERROR_PATH_NOT_FOUND)
	require.Error(t, err)
	assert.ErrorIs(t, err, errWindowsLongPath)
	assert.ErrorIs(t, err, syscall.ERROR_PATH_NOT_FOUND)
	assert.Contains(t, err.Error(), "--local-nounc")
}

func TestWrapWindowsLongPathErrorIgnoresShortPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("windows only")
	}
	ctx := context.Background()
	fInterface, err := NewFs(ctx, "local", t.TempDir(), configmap.Simple{
		"nounc": "true",
	})
	require.NoError(t, err)
	f := fInterface.(*Fs)

	err = f.wrapPathLengthError(f.localPath("missing.txt"), syscall.ERROR_PATH_NOT_FOUND)
	require.Error(t, err)
	assert.NotErrorIs(t, err, errWindowsLongPath)
}
