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
	if !run.vfsOpt.Links {
		t.Skip("No symlinks configured")
	}

	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows")
	}

	fs.Logf(nil, "Links: %v, useVFS: %v, suffix: %v", run.vfsOpt.Links, run.useVFS, fs.LinkSuffix)

	// Create initial setup of test files and directories we will create links to
	run.mkdir(t, "dir1")
	run.mkdir(t, "dir1/sub1dir1")
	run.createFile(t, "dir1/file1", "potato")
	run.mkdir(t, "dir2")
	run.mkdir(t, "dir2/sub1dir2")
	run.createFile(t, "dir2/file1", "chicken")

	// base state all the tests will be build off
	baseState := "dir1/|dir1/sub1dir1/|dir1/file1 6|dir2/|dir2/sub1dir2/|dir2/file1 7"
	// Check the tests return to the base state
	checkBaseState := func() {
		run.checkDir(t, baseState)
	}
	checkBaseState()

	t.Run("FileLink", func(t *testing.T) {
		// Link to a file
		run.symlink(t, "dir1/file1", "dir1file1_link")
		run.checkDir(t, baseState+"|dir1file1_link 10")
		run.checkMode(t, "dir1file1_link", os.FileMode(run.vfsOpt.LinkPerms), os.FileMode(run.vfsOpt.FilePerms))
		assert.Equal(t, "dir1/file1", run.readlink(t, "dir1file1_link"))

		// Read through a symlink
		assert.Equal(t, "potato", run.readFile(t, "dir1file1_link"))

		// Write through a symlink
		err := writeFile(run.path("dir1file1_link"), []byte("carrot"), 0600)
		require.NoError(t, err)
		assert.Equal(t, "carrot", run.readFile(t, "dir1file1_link"))
		assert.Equal(t, "carrot", run.readFile(t, "dir1/file1"))

		// Rename a symlink
		err = run.os.Rename(run.path("dir1file1_link"), run.path("dir1file1_link")+"_bla")
		require.NoError(t, err)
		run.checkDir(t, baseState+"|dir1file1_link_bla 10")
		assert.Equal(t, "dir1/file1", run.readlink(t, "dir1file1_link_bla"))

		// Delete a symlink
		run.rm(t, "dir1file1_link_bla")
		checkBaseState()
	})

	t.Run("DirLink", func(t *testing.T) {
		// Link to a dir
		run.symlink(t, "dir1", "dir1_link")
		run.checkDir(t, baseState+"|dir1_link 4")
		run.checkMode(t, "dir1_link", os.FileMode(run.vfsOpt.LinkPerms), os.FileMode(run.vfsOpt.DirPerms))
		assert.Equal(t, "dir1", run.readlink(t, "dir1_link"))

		// Check you can't open a directory symlink
		_, err := run.os.OpenFile(run.path("dir1_link"), os.O_WRONLY, 0600)
		require.Error(t, err)

		// Our symlink resolution is very simple when using the VFS as when using the
		// mount the OS will resolve the symlinks, so we don't recurse here

		// Read entries directly
		dir1Entries := make(dirMap)
		run.readLocalEx(t, dir1Entries, "dir1", false)
		assert.Equal(t, newDirMap("dir1/sub1dir1/|dir1/file1 6"), dir1Entries)

		// Read entries through the directory symlink
		dir1EntriesSymlink := make(dirMap)
		run.readLocalEx(t, dir1EntriesSymlink, "dir1_link", false)
		assert.Equal(t, newDirMap("dir1_link/sub1dir1/|dir1_link/file1 6"), dir1EntriesSymlink)

		// Rename directory symlink
		err = run.os.Rename(run.path("dir1_link"), run.path("dir1_link")+"_bla")
		require.NoError(t, err)
		run.checkDir(t, baseState+"|dir1_link_bla 4")
		assert.Equal(t, "dir1", run.readlink(t, "dir1_link_bla"))

		// Remove directory symlink
		run.rm(t, "dir1_link_bla")

		checkBaseState()
	})

	// Corner case #1 - We do not allow creating regular and symlink files having the same name (ie, test.txt and test.txt.rclonelink)

	// Symlink first, then regular
	t.Run("OverwriteSymlinkWithRegular", func(t *testing.T) {
		link1Name := "link1.txt"

		run.symlink(t, "dir1/file1", link1Name)
		run.checkDir(t, baseState+"|link1.txt 10")

		fh, err := run.os.OpenFile(run.path(link1Name), os.O_WRONLY|os.O_CREATE, os.FileMode(run.vfsOpt.FilePerms))

		// On real mount with links enabled, that open the symlink target as expected, else that fails to create a new file
		assert.NoError(t, err)
		// Don't care about the result, in some cache mode the file can't be opened for writing, so closing would trigger an err
		_ = fh.Close()

		run.rm(t, link1Name)
		checkBaseState()
	})

	// Regular first, then symlink
	t.Run("OverwriteRegularWithSymlink", func(t *testing.T) {
		link1Name := "link1.txt"

		run.createFile(t, link1Name, "")
		run.checkDir(t, baseState+"|link1.txt 0")

		err := run.os.Symlink(".", run.path(link1Name))
		assert.Error(t, err)

		run.rm(t, link1Name)
		checkBaseState()
	})

	// Corner case #2 - We do not allow creating directory and symlink file having the same name (ie, test and test.rclonelink)

	// Symlink first, then directory
	t.Run("OverwriteSymlinkWithDirectory", func(t *testing.T) {
		link1Name := "link1"

		run.symlink(t, ".", link1Name)
		run.checkDir(t, baseState+"|link1 1")

		err := run.os.Mkdir(run.path(link1Name), os.FileMode(run.vfsOpt.DirPerms))
		assert.Error(t, err)

		run.rm(t, link1Name)
		checkBaseState()
	})

	// Directory first, then symlink
	t.Run("OverwriteDirectoryWithSymlink", func(t *testing.T) {
		link1Name := "link1"

		run.mkdir(t, link1Name)
		run.checkDir(t, baseState+"|link1/")

		err := run.os.Symlink(".", run.path(link1Name))
		assert.Error(t, err)

		run.rm(t, link1Name)
		checkBaseState()
	})

	// Corner case #3 - We do not allow moving directory or file having the same name in a target (ie, test and test.rclonelink)

	// Move symlink -> regular file
	t.Run("MoveSymlinkToFile", func(t *testing.T) {
		t.Skip("FIXME not implemented")
		link1Name := "link1.txt"

		run.symlink(t, ".", link1Name)
		run.createFile(t, "dir1/link1.txt", "")
		run.checkDir(t, baseState+"|link1.txt 1|dir1/link1.txt 0")

		err := run.os.Rename(run.path(link1Name), run.path("dir1/"+link1Name))
		assert.Error(t, err)

		run.rm(t, link1Name)
		run.rm(t, "dir1/link1.txt")
		checkBaseState()
	})

	// Move regular file -> symlink
	t.Run("MoveFileToSymlink", func(t *testing.T) {
		t.Skip("FIXME not implemented")
		link1Name := "link1.txt"

		run.createFile(t, link1Name, "")
		run.symlink(t, ".", "dir1/"+link1Name)
		run.checkDir(t, baseState+"|link1.txt 0|dir1/link1.txt 1")

		err := run.os.Rename(run.path(link1Name), run.path("dir1/link1.txt"))
		assert.Error(t, err)

		run.rm(t, link1Name)
		run.rm(t, "dir1/"+link1Name)
		checkBaseState()
	})

	// Move symlink -> directory
	t.Run("MoveSymlinkToDirectory", func(t *testing.T) {
		t.Skip("FIXME not implemented")
		link1Name := "link1"

		run.symlink(t, ".", link1Name)
		run.mkdir(t, "dir1/link1")
		run.checkDir(t, baseState+"|link1 1|dir1/link1/")

		err := run.os.Rename(run.path(link1Name), run.path("dir1/"+link1Name))
		assert.Error(t, err)

		run.rm(t, link1Name)
		run.rm(t, "dir1/link1")
		checkBaseState()
	})

	// Move directory -> symlink
	t.Run("MoveDirectoryToSymlink", func(t *testing.T) {
		t.Skip("FIXME not implemented")
		link1Name := "dir1/link1"

		run.mkdir(t, "link1")
		run.symlink(t, ".", link1Name)
		run.checkDir(t, baseState+"|link1/|dir1/link1 1")

		err := run.os.Rename(run.path("link1"), run.path("dir1/link1"))
		assert.Error(t, err)

		run.rm(t, "link1")
		run.rm(t, link1Name)
		checkBaseState()
	})
}
