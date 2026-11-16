//go:build windows

package local

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

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

// TestListDirModTimeWindows tests that the modification time of a
// directory is up to date in a listing made straight after something
// is created in it. The time Windows returns when listing the
// directory it is in isn't updated until the directory is next opened.
func TestListDirModTimeWindows(t *testing.T) {
	ctx := context.Background()
	r := fstest.NewRun(t)
	defer r.Finalise()

	require.NoError(t, operations.Mkdir(ctx, r.Flocal, "dir"))
	dirPath := filepath.Join(r.LocalName, "dir")
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	require.NoError(t, os.Chtimes(dirPath, old, old))
	require.NoError(t, os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("hello"), 0o666))

	entries, err := r.Flocal.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	listed := entries[0].ModTime(ctx)

	// This opens the directory, so has to come after the listing
	fi, err := os.Stat(dirPath)
	require.NoError(t, err)
	assert.False(t, fi.ModTime().Equal(old), "creating a file didn't change the modification time of the directory")
	assert.True(t, listed.Equal(fi.ModTime()), "listed %v but the directory has %v", listed, fi.ModTime())
}
