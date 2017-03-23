// +build linux darwin freebsd

package mount

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirLs(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.checkDir(t, "")

	run.mkdir(t, "a directory")
	run.createFile(t, "a file", "hello")

	run.checkDir(t, "a directory/|a file 5")

	run.rmdir(t, "a directory")
	run.rm(t, "a file")

	run.checkDir(t, "")
}

func TestDirCreateAndRemoveDir(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.mkdir(t, "dir/subdir")
	run.checkDir(t, "dir/|dir/subdir/")

	// Check we can't delete a directory with stuff in
	err := os.Remove(run.path("dir"))
	assert.Error(t, err, "file exists")

	// Now delete subdir then dir - should produce no errors
	run.rmdir(t, "dir/subdir")
	run.checkDir(t, "dir/")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

func TestDirCreateAndRemoveFile(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.createFile(t, "dir/file", "potato")
	run.checkDir(t, "dir/|dir/file 6")

	// Check we can't delete a directory with stuff in
	err := os.Remove(run.path("dir"))
	assert.Error(t, err, "file exists")

	// Now delete file
	run.rm(t, "dir/file")

	run.checkDir(t, "dir/")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

func TestDirRenameFile(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.createFile(t, "file", "potato")
	run.checkDir(t, "dir/|file 6")

	err := os.Rename(run.path("file"), run.path("file2"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|file2 6")

	data, err := ioutil.ReadFile(run.path("file2"))
	require.NoError(t, err)
	assert.Equal(t, "potato", string(data))

	err = os.Rename(run.path("file2"), run.path("dir/file3"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/file3 6")

	data, err = ioutil.ReadFile(run.path("dir/file3"))
	require.NoError(t, err)
	assert.Equal(t, "potato", string(data))

	run.rm(t, "dir/file3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

func TestDirRenameEmptyDir(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.mkdir(t, "dir1")
	run.checkDir(t, "dir/|dir1/")

	err := os.Rename(run.path("dir1"), run.path("dir/dir2"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir2/")

	err = os.Rename(run.path("dir/dir2"), run.path("dir/dir3"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir3/")

	run.rmdir(t, "dir/dir3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

func TestDirRenameFullDir(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.mkdir(t, "dir1")
	run.createFile(t, "dir1/potato.txt", "maris piper")
	run.checkDir(t, "dir/|dir1/|dir1/potato.txt 11")

	err := os.Rename(run.path("dir1"), run.path("dir/dir2"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir2/|dir/dir2/potato.txt 11")

	err = os.Rename(run.path("dir/dir2"), run.path("dir/dir3"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir3/|dir/dir3/potato.txt 11")

	run.rm(t, "dir/dir3/potato.txt")
	run.rmdir(t, "dir/dir3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

func TestDirModTime(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	mtime := time.Date(2012, 11, 18, 17, 32, 31, 0, time.UTC)
	err := os.Chtimes(run.path("dir"), mtime, mtime)
	require.NoError(t, err)

	info, err := os.Stat(run.path("dir"))
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())

	run.rmdir(t, "dir")
}
