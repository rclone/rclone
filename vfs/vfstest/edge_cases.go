package vfstest

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTouchAndDelete checks that writing a zero byte file and immediately
// deleting it is not racy. See https://github.com/rclone/rclone/issues/1181
func TestTouchAndDelete(t *testing.T) {
	run.skipIfNoFUSE(t)
	run.checkDir(t, "")

	run.createFile(t, "touched", "")
	run.rm(t, "touched")

	run.checkDir(t, "")
}

// TestRenameOpenHandle checks that a file with open writers is successfully
// renamed after all writers close. See https://github.com/rclone/rclone/issues/2130
func TestRenameOpenHandle(t *testing.T) {
	run.skipIfNoFUSE(t)
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	run.checkDir(t, "")

	// create file
	example := []byte("Some Data")
	path := run.path("rename")
	file, err := osCreate(path)
	require.NoError(t, err)

	// write some data
	_, err = file.Write(example)
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	// attempt to rename open file
	err = run.os.Rename(path, path+"bla")
	require.NoError(t, err)

	// close open writers to allow rename on remote to go through
	err = file.Close()
	require.NoError(t, err)

	run.waitForWriters()

	// verify file was renamed properly
	run.checkDir(t, "renamebla 9")

	// cleanup
	run.rm(t, "renamebla")
	run.checkDir(t, "")
}
