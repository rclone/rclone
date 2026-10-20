package vfstest

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirLs checks out listing
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

// TestDirCreateAndRemoveDir tests creating and removing a directory
func TestDirCreateAndRemoveDir(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.mkdir(t, "dir/subdir")
	run.checkDir(t, "dir/|dir/subdir/")

	// Check we can't delete a directory with stuff in
	err := run.os.Remove(run.path("dir"))
	assert.Error(t, err, "file exists")

	// Now delete subdir then dir - should produce no errors
	run.rmdir(t, "dir/subdir")
	run.checkDir(t, "dir/")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

// TestDirCreateAndRemoveFile tests creating and removing a file
func TestDirCreateAndRemoveFile(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.createFile(t, "dir/file", "potato")
	run.checkDir(t, "dir/|dir/file 6")

	// Check we can't delete a directory with stuff in
	err := run.os.Remove(run.path("dir"))
	assert.Error(t, err, "file exists")

	// Now delete file
	run.rm(t, "dir/file")

	run.checkDir(t, "dir/")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

// TestDirRenameFile tests renaming a file
func TestDirRenameFile(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.createFile(t, "file", "potato")
	run.checkDir(t, "dir/|file 6")

	err := run.os.Rename(run.path("file"), run.path("file2"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|file2 6")

	data := run.readFile(t, "file2")
	assert.Equal(t, "potato", data)

	err = run.os.Rename(run.path("file2"), run.path("dir/file3"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/file3 6")

	data = run.readFile(t, "dir/file3")
	require.NoError(t, err)
	assert.Equal(t, "potato", data)

	run.rm(t, "dir/file3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

// TestDirRenameEmptyDir tests renaming and empty directory
func TestDirRenameEmptyDir(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.mkdir(t, "dir1")
	run.checkDir(t, "dir/|dir1/")

	err := run.os.Rename(run.path("dir1"), run.path("dir/dir2"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir2/")

	err = run.os.Rename(run.path("dir/dir2"), run.path("dir/dir3"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir3/")

	run.rmdir(t, "dir/dir3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

// TestDirRenameFullDir tests renaming a full directory
func TestDirRenameFullDir(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.mkdir(t, "dir1")
	run.createFile(t, "dir1/potato.txt", "maris piper")
	run.checkDir(t, "dir/|dir1/|dir1/potato.txt 11")

	err := run.os.Rename(run.path("dir1"), run.path("dir/dir2"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir2/|dir/dir2/potato.txt 11")

	err = run.os.Rename(run.path("dir/dir2"), run.path("dir/dir3"))
	require.NoError(t, err)
	run.checkDir(t, "dir/|dir/dir3/|dir/dir3/potato.txt 11")

	run.rm(t, "dir/dir3/potato.txt")
	run.rmdir(t, "dir/dir3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

// TestDirModTime tests mod times
func TestDirModTime(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	mtime := time.Date(2012, time.November, 18, 17, 32, 31, 0, time.UTC)
	err := run.os.Chtimes(run.path("dir"), mtime, mtime)
	require.NoError(t, err)

	info, err := run.os.Stat(run.path("dir"))
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())

	run.rmdir(t, "dir")
}

// TestDirCacheFlush tests flushing the dir cache
func TestDirCacheFlush(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.checkDir(t, "")

	run.mkdir(t, "dir")
	run.mkdir(t, "otherdir")
	run.createFile(t, "dir/file", "1")
	run.createFile(t, "otherdir/file", "1")

	dm := newDirMap("otherdir/|otherdir/file 1|dir/|dir/file 1")
	localDm := make(dirMap)
	run.readLocal(t, localDm, "")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")

	err := run.fremote.Mkdir(context.Background(), "dir/subdir")
	require.NoError(t, err)

	// expect newly created "subdir" on remote to not show up
	run.forget("otherdir")
	run.readLocal(t, localDm, "")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")

	run.forget("dir")
	dm = newDirMap("otherdir/|otherdir/file 1|dir/|dir/file 1|dir/subdir/")
	run.readLocal(t, localDm, "")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")

	run.rm(t, "otherdir/file")
	run.rmdir(t, "otherdir")
	run.rm(t, "dir/file")
	run.rmdir(t, "dir/subdir")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}

// TestDirCacheFlushOnDirRename tests flushing the dir cache on rename
func TestDirCacheFlushOnDirRename(t *testing.T) {
	run.skipIfNoFUSE(t)
	run.mkdir(t, "dir")
	run.createFile(t, "dir/file", "1")

	dm := newDirMap("dir/|dir/file 1")
	localDm := make(dirMap)
	run.readLocal(t, localDm, "")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")

	// expect remotely created directory to not show up
	err := run.fremote.Mkdir(context.Background(), "dir/subdir")
	require.NoError(t, err)
	run.readLocal(t, localDm, "")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")

	err = run.os.Rename(run.path("dir"), run.path("rid"))
	require.NoError(t, err)

	dm = newDirMap("rid/|rid/subdir/|rid/file 1")
	localDm = make(dirMap)
	run.readLocal(t, localDm, "")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")

	run.rm(t, "rid/file")
	run.rmdir(t, "rid/subdir")
	run.rmdir(t, "rid")
	run.checkDir(t, "")
}

func (r *Run) renameDirOver(oldName, newName string) error {
	if r.useVFS {
		return r.os.Rename(oldName, newName)
	}
	// os.Rename rejects existing directory targets before calling rename(2).
	return syscall.Rename(oldName, newName)
}

// TestDirRenameOverEmptyDir tests replacing an empty directory.
func TestDirRenameOverEmptyDir(t *testing.T) {
	run.skipIfNoFUSE(t)
	if !run.useVFS && runtime.GOOS != "linux" {
		t.Skip("Directory replacement requires Linux mount semantics")
	}
	run.mkdir(t, "source")
	run.mkdir(t, "target")
	run.createFile(t, "source/file", "potato")
	err := run.renameDirOver(run.path("source"), run.path("target"))
	assert.NoError(t, err)
	if err != nil {
		run.rm(t, "source/file")
		run.rmdir(t, "source")
	} else {
		run.checkDir(t, "target/|target/file 6")
		run.rm(t, "target/file")
	}
	run.rmdir(t, "target")
	run.checkDir(t, "")
}

// TestDirRenameOverFullDir tests preserving a non-empty target directory.
func TestDirRenameOverFullDir(t *testing.T) {
	run.skipIfNoFUSE(t)
	if !run.useVFS && runtime.GOOS != "linux" {
		t.Skip("Directory replacement requires Linux mount semantics")
	}
	run.mkdir(t, "source")
	run.mkdir(t, "target")
	run.createFile(t, "source/file", "potato")
	run.createFile(t, "target/file", "other")
	err := run.renameDirOver(run.path("source"), run.path("target"))
	if run.isNFSMount() && errors.Is(err, syscall.EIO) {
		// The NFS server library (go-nfs) reports every rename error
		// other than ENOENT and EACCES as NFS3ERR_IO, which the
		// client returns as EIO, so ENOTEMPTY can't be passed through.
		//
		// TODO: remove when go-nfs maps ENOTEMPTY to NFS3ERR_NOTEMPTY
		t.Logf("NFS mount returned %v for rename over a non-empty directory", err)
	} else {
		assert.True(t, errors.Is(err, vfs.ENOTEMPTY) || errors.Is(err, syscall.ENOTEMPTY), "expected ENOTEMPTY, got %v", err)
	}
	run.checkDir(t, "source/|source/file 6|target/|target/file 5")
	assert.Equal(t, "potato", run.readFile(t, "source/file"))
	assert.Equal(t, "other", run.readFile(t, "target/file"))
	run.rm(t, "source/file")
	run.rm(t, "target/file")
	run.rmdir(t, "source")
	run.rmdir(t, "target")
	run.checkDir(t, "")
}
