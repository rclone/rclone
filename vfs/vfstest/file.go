package vfstest

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileModTime tests mod times on files
func TestFileModTime(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.createFile(t, "file", "123")

	mtime := time.Date(2012, time.November, 18, 17, 32, 31, 0, time.UTC)
	err := run.os.Chtimes(run.path("file"), mtime, mtime)
	require.NoError(t, err)

	info, err := run.os.Stat(run.path("file"))
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())

	run.rm(t, "file")
}

// run.os.Create without opening for write too
func osCreate(name string) (vfs.OsFiler, error) {
	return run.os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
}

// run.os.Create with append
func osAppend(name string) (vfs.OsFiler, error) {
	return run.os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0666)
}

// TestFileModTimeWithOpenWriters tests mod time on open files
func TestFileModTimeWithOpenWriters(t *testing.T) {
	run.skipIfNoFUSE(t)
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	mtime := time.Date(2012, time.November, 18, 17, 32, 31, 0, time.UTC)
	filepath := run.path("cp-archive-test")

	f, err := osCreate(filepath)
	require.NoError(t, err)

	_, err = f.Write([]byte{104, 105})
	require.NoError(t, err)

	err = run.os.Chtimes(filepath, mtime, mtime)
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	run.waitForWriters()

	info, err := run.os.Stat(filepath)
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())

	run.rm(t, "cp-archive-test")
}

// TestSymlinks tests all the api of the VFS / Mount symlinks support
func TestSymlinks(t *testing.T) {
	run.skipIfNoFUSE(t)

	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	{
		// VFS only implements os.Stat, which return information to target for symlinks, getting symlink information would require os.Lstat implementation.
		// We will not bother to add Lstat implemented, but in the test we can just call os.Lstat which return the information needed when !useVFS

		// this is a link to a directory
		// ldl, _ := os.Lstat("/tmp/kkk/link_dir")
		// ld, _ := os.Stat("/tmp/kkk/link_dir")

		// LINK_DIR: Lrwxrwxrwx, false <-> drwxr-xr-x, true
		// fs.Logf(nil, "LINK_DIR: %v, %v <-> %v, %v", ldl.Mode(), ldl.IsDir(), ld.Mode(), ld.IsDir())

		// This is a link to a regular file
		// lfl, _ := os.Lstat("/tmp/kkk/link_file")
		// lf, _ := os.Stat("/tmp/kkk/link_file")

		// LINK_FILE: Lrwxrwxrwx, false <-> -rw-r--r--, false
		// fs.Logf(nil, "LINK_FILE: %v, %v <-> %v, %v", lfl.Mode(), lfl.IsDir(), lf.Mode(), lf.IsDir())
	}

	if !run.useVFS {
		t.Skip("Requires useVFS")
	}

	suffix := ""

	if run.useVFS || !run.vfsOpt.Links {
		suffix = fs.LinkSuffix
	}

	fs.Logf(nil, "Links: %v, useVFS: %v, suffix: %v", run.vfsOpt.Links, run.useVFS, suffix)

	run.mkdir(t, "dir1")
	run.mkdir(t, "dir1/sub1dir1")
	run.createFile(t, "dir1/file1", "potato")

	run.mkdir(t, "dir2")
	run.mkdir(t, "dir2/sub1dir2")
	run.createFile(t, "dir2/file1", "chicken")

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")

	// Link to a file
	run.relativeSymlink(t, "dir1/file1", "dir1file1_link"+suffix)

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|dir1file1_link"+suffix+" 10")

	if run.vfsOpt.Links {
		if run.useVFS {
			run.checkMode(t, "dir1file1_link"+suffix, run.vfsOpt.LinkPerms, run.vfsOpt.LinkPerms)
		} else {
			run.checkMode(t, "dir1file1_link"+suffix, run.vfsOpt.LinkPerms, run.vfsOpt.FilePerms)
		}
	} else {
		run.checkMode(t, "dir1file1_link"+suffix, run.vfsOpt.FilePerms, run.vfsOpt.FilePerms)
	}

	assert.Equal(t, "dir1/file1", run.readlink(t, "dir1file1_link"+suffix))

	if !run.useVFS && run.vfsOpt.Links {
		assert.Equal(t, "potato", run.readFile(t, "dir1file1_link"+suffix))

		err := writeFile(run.path("dir1file1_link"+suffix), []byte("carrot"), 0600)
		require.NoError(t, err)

		assert.Equal(t, "carrot", run.readFile(t, "dir1file1_link"+suffix))
		assert.Equal(t, "carrot", run.readFile(t, "dir1/file1"))
	} else {
		assert.Equal(t, "dir1/file1", run.readFile(t, "dir1file1_link"+suffix))
	}

	err := run.os.Rename(run.path("dir1file1_link"+suffix), run.path("dir1file1_link")+"_bla"+suffix)
	require.NoError(t, err)

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|dir1file1_link_bla"+suffix+" 10")

	assert.Equal(t, "dir1/file1", run.readlink(t, "dir1file1_link_bla"+suffix))

	run.rm(t, "dir1file1_link_bla"+suffix)

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")

	// Link to a dir
	run.relativeSymlink(t, "dir1", "dir1_link"+suffix)

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|dir1_link"+suffix+" 4")

	if run.vfsOpt.Links {
		if run.useVFS {
			run.checkMode(t, "dir1_link"+suffix, run.vfsOpt.LinkPerms, run.vfsOpt.LinkPerms)
		} else {
			run.checkMode(t, "dir1_link"+suffix, run.vfsOpt.LinkPerms, run.vfsOpt.DirPerms)
		}
	} else {
		run.checkMode(t, "dir1_link"+suffix, run.vfsOpt.FilePerms, run.vfsOpt.FilePerms)
	}

	assert.Equal(t, "dir1", run.readlink(t, "dir1_link"+suffix))

	fh, err := run.os.OpenFile(run.path("dir1_link"+suffix), os.O_WRONLY, 0600)

	if !run.useVFS && run.vfsOpt.Links {
		require.Error(t, err)

		dirLinksEntries := make(dirMap)
		run.readLocal(t, dirLinksEntries, "dir1_link"+suffix)

		assert.Equal(t, 2, len(dirLinksEntries))

		dir1Entries := make(dirMap)
		run.readLocal(t, dir1Entries, "dir1")

		assert.Equal(t, 2, len(dir1Entries))
	} else {
		require.NoError(t, err)
		// Don't care about the result, in some cache mode the file can't be opened for writing, so closing would trigger an err
		_ = fh.Close()

		assert.Equal(t, "dir1", run.readFile(t, "dir1_link"+suffix))
	}

	err = run.os.Rename(run.path("dir1_link"+suffix), run.path("dir1_link")+"_bla"+suffix)
	require.NoError(t, err)

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|dir1_link_bla"+suffix+" 4")

	assert.Equal(t, "dir1", run.readlink(t, "dir1_link_bla"+suffix))

	run.rm(t, "dir1_link_bla"+suffix) // run.rmdir works fine as well

	run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")

	// Corner case #1 - We do not allow creating regular and symlink files having the same name (ie, test.txt and test.txt.rclonelink)

	// Symlink first, then regular
	{
		link1Name := "link1.txt" + suffix

		run.relativeSymlink(t, "dir1/file1", link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1.txt"+suffix+" 10")

		fh, err = run.os.OpenFile(run.path("link1.txt"), os.O_WRONLY|os.O_CREATE, run.vfsOpt.FilePerms)

		// On real mount with links enabled, that open the symlink target as expected, else that fails to create a new file
		if !run.useVFS && run.vfsOpt.Links {
			assert.Equal(t, true, err == nil)
			// Don't care about the result, in some cache mode the file can't be opened for writing, so closing would trigger an err
			_ = fh.Close()
		} else {
			assert.Equal(t, true, err != nil)
		}

		run.rm(t, link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Regular first, then symlink
	{
		link1Name := "link1.txt" + suffix

		run.createFile(t, "link1.txt", "")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1.txt 0")

		err = run.os.Symlink(".", run.path(link1Name))
		assert.Equal(t, true, err != nil)

		run.rm(t, "link1.txt")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Corner case #2 - We do not allow creating directory and symlink file having the same name (ie, test and test.rclonelink)

	// Symlink first, then directory
	{
		link1Name := "link1" + suffix

		run.relativeSymlink(t, ".", link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1"+suffix+" 1")

		err = run.os.Mkdir(run.path("link1"), run.vfsOpt.DirPerms)
		assert.Equal(t, true, err != nil)

		run.rm(t, link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Directory first, then symlink
	{
		link1Name := "link1" + suffix

		run.mkdir(t, "link1")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1/")

		err = run.os.Symlink(".", run.path(link1Name))
		assert.Equal(t, true, err != nil)

		run.rm(t, "link1")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Corner case #3 - We do not allow moving directory or file having the same name in a target (ie, test and test.rclonelink)

	// Move symlink -> regular file
	{
		link1Name := "link1.txt" + suffix

		run.relativeSymlink(t, ".", link1Name)
		run.createFile(t, "dir1/link1.txt", "")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1.txt"+suffix+" 1|dir1/link1.txt 0")

		err = run.os.Rename(run.path(link1Name), run.path("dir1/"+link1Name))
		assert.Equal(t, true, err != nil)

		run.rm(t, link1Name)
		run.rm(t, "dir1/link1.txt")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Move regular file -> symlink
	{
		link1Name := "link1.txt" + suffix

		run.createFile(t, "link1.txt", "")
		run.relativeSymlink(t, ".", "dir1/"+link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1.txt 0|dir1/link1.txt"+suffix+" 1")

		err = run.os.Rename(run.path("link1.txt"), run.path("dir1/link1.txt"))
		assert.Equal(t, true, err != nil)

		run.rm(t, "link1.txt")
		run.rm(t, "dir1/"+link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Move symlink -> directory
	{
		link1Name := "link1" + suffix

		run.relativeSymlink(t, ".", link1Name)
		run.mkdir(t, "dir1/link1")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1"+suffix+" 1|dir1/link1/")

		err = run.os.Rename(run.path(link1Name), run.path("dir1/"+link1Name))
		assert.Equal(t, true, err != nil)

		run.rm(t, link1Name)
		run.rm(t, "dir1/link1")
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}

	// Move directory -> symlink
	{
		link1Name := "dir1/link1" + suffix

		run.mkdir(t, "link1")
		run.relativeSymlink(t, ".", link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7|link1/|dir1/link1"+suffix+" 1")

		err = run.os.Rename(run.path("link1"), run.path("dir1/link1"))
		assert.Equal(t, true, err != nil)

		run.rm(t, "link1")
		run.rm(t, link1Name)
		run.checkDir(t, "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7")
	}
}
