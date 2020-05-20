package vfs

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fileCreate(t *testing.T, r *fstest.Run, mode vfscommon.CacheMode) (*VFS, *File, fstest.Item) {
	opt := vfscommon.DefaultOpt
	opt.CacheMode = mode
	vfs := New(r.Fremote, &opt)

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	node, err := vfs.Stat("dir/file1")
	require.NoError(t, err)
	require.True(t, node.Mode().IsRegular())

	return vfs, node.(*File), file1
}

func TestFileMethods(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r, vfscommon.CacheModeOff)

	// String
	assert.Equal(t, "dir/file1", file.String())
	assert.Equal(t, "<nil *File>", (*File)(nil).String())

	// IsDir
	assert.Equal(t, false, file.IsDir())

	// IsFile
	assert.Equal(t, true, file.IsFile())

	// Mode
	assert.Equal(t, vfs.Opt.FilePerms, file.Mode())

	// Name
	assert.Equal(t, "file1", file.Name())

	// Path
	assert.Equal(t, "dir/file1", file.Path())

	// Sys
	assert.Equal(t, nil, file.Sys())

	// SetSys
	file.SetSys(42)
	assert.Equal(t, 42, file.Sys())

	// Inode
	assert.NotEqual(t, uint64(0), file.Inode())

	// Node
	assert.Equal(t, file, file.Node())

	// ModTime
	assert.WithinDuration(t, t1, file.ModTime(), r.Fremote.Precision())

	// Size
	assert.Equal(t, int64(14), file.Size())

	// Sync
	assert.NoError(t, file.Sync())

	// DirEntry
	assert.Equal(t, file.o, file.DirEntry())

	// Dir
	assert.Equal(t, file.d, file.Dir())

	// VFS
	assert.Equal(t, vfs, file.VFS())
}

func TestFileSetModTime(t *testing.T) {
	r := fstest.NewRun(t)
	if !canSetModTime(t, r) {
		return
	}
	defer r.Finalise()
	vfs, file, file1 := fileCreate(t, r, vfscommon.CacheModeOff)

	err := file.SetModTime(t2)
	require.NoError(t, err)

	file1.ModTime = t2
	fstest.CheckItems(t, r.Fremote, file1)

	vfs.Opt.ReadOnly = true
	err = file.SetModTime(t2)
	assert.Equal(t, EROFS, err)
}

func fileCheckContents(t *testing.T, file *File) {
	fd, err := file.Open(os.O_RDONLY)
	require.NoError(t, err)

	contents, err := ioutil.ReadAll(fd)
	require.NoError(t, err)
	assert.Equal(t, "file1 contents", string(contents))

	require.NoError(t, fd.Close())
}

func TestFileOpenRead(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, file, _ := fileCreate(t, r, vfscommon.CacheModeOff)

	fileCheckContents(t, file)
}

func TestFileOpenReadUnknownSize(t *testing.T) {
	var (
		contents = []byte("file contents")
		remote   = "file.txt"
		ctx      = context.Background()
	)

	// create a mock object which returns size -1
	o := mockobject.New(remote).WithContent(contents, mockobject.SeekModeNone)
	o.SetUnknownSize(true)
	assert.Equal(t, int64(-1), o.Size())

	// add it to a mock fs
	f := mockfs.NewFs("test", "root")
	f.AddObject(o)
	testObj, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	assert.Equal(t, int64(-1), testObj.Size())

	// create a VFS from that mockfs
	vfs := New(f, nil)

	// find the file
	node, err := vfs.Stat(remote)
	require.NoError(t, err)
	require.True(t, node.IsFile())
	file := node.(*File)

	// open it
	fd, err := file.openRead()
	require.NoError(t, err)
	assert.Equal(t, int64(0), fd.Size())

	// check the contents are not empty even though size is empty
	gotContents, err := ioutil.ReadAll(fd)
	require.NoError(t, err)
	assert.Equal(t, contents, gotContents)
	t.Logf("gotContents = %q", gotContents)

	// check that file size has been updated
	assert.Equal(t, int64(len(contents)), fd.Size())

	require.NoError(t, fd.Close())
}

func TestFileOpenWrite(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r, vfscommon.CacheModeOff)

	fd, err := file.openWrite(os.O_WRONLY | os.O_TRUNC)
	require.NoError(t, err)

	newContents := []byte("this is some new contents")
	n, err := fd.Write(newContents)
	require.NoError(t, err)
	assert.Equal(t, len(newContents), n)
	require.NoError(t, fd.Close())

	assert.Equal(t, int64(25), file.Size())

	vfs.Opt.ReadOnly = true
	_, err = file.openWrite(os.O_WRONLY | os.O_TRUNC)
	assert.Equal(t, EROFS, err)
}

func TestFileRemove(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r, vfscommon.CacheModeOff)

	err := file.Remove()
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote)

	vfs.Opt.ReadOnly = true
	err = file.Remove()
	assert.Equal(t, EROFS, err)
}

func TestFileRemoveAll(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r, vfscommon.CacheModeOff)

	err := file.RemoveAll()
	require.NoError(t, err)

	fstest.CheckItems(t, r.Fremote)

	vfs.Opt.ReadOnly = true
	err = file.RemoveAll()
	assert.Equal(t, EROFS, err)
}

func TestFileOpen(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, file, _ := fileCreate(t, r, vfscommon.CacheModeOff)

	fd, err := file.Open(os.O_RDONLY)
	require.NoError(t, err)
	_, ok := fd.(*ReadFileHandle)
	assert.True(t, ok)
	require.NoError(t, fd.Close())

	fd, err = file.Open(os.O_WRONLY)
	assert.NoError(t, err)
	_, ok = fd.(*WriteFileHandle)
	assert.True(t, ok)
	require.NoError(t, fd.Close())

	fd, err = file.Open(os.O_RDWR)
	assert.NoError(t, err)
	_, ok = fd.(*WriteFileHandle)
	assert.True(t, ok)

	_, err = file.Open(3)
	assert.Equal(t, EPERM, err)
}

func testFileRename(t *testing.T, mode vfscommon.CacheMode) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, item := fileCreate(t, r, mode)

	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("skip as can't rename files")
	}

	rootDir, err := vfs.Root()
	require.NoError(t, err)

	// check file in cache
	if mode != vfscommon.CacheModeOff {
		// read contents to get file in cache
		fileCheckContents(t, file)
		assert.True(t, vfs.cache.Exists(item.Path))
	}

	dir := file.Dir()

	// start with "dir/file1"
	fstest.CheckItems(t, r.Fremote, item)

	// rename file to "newLeaf"
	err = dir.Rename("file1", "newLeaf", rootDir)
	require.NoError(t, err)

	item.Path = "newLeaf"
	fstest.CheckItems(t, r.Fremote, item)

	// check file in cache
	if mode != vfscommon.CacheModeOff {
		assert.True(t, vfs.cache.Exists(item.Path))
	}

	// check file exists in the vfs layer at its new name
	_, err = vfs.Stat("newLeaf")
	require.NoError(t, err)

	// rename it back to "dir/file1"
	err = rootDir.Rename("newLeaf", "file1", dir)
	require.NoError(t, err)

	item.Path = "dir/file1"
	fstest.CheckItems(t, r.Fremote, item)

	// check file in cache
	if mode != vfscommon.CacheModeOff {
		assert.True(t, vfs.cache.Exists(item.Path))
	}

	// now try renaming it with the file open
	// first open it and write to it but don't close it
	fd, err := file.Open(os.O_WRONLY | os.O_TRUNC)
	require.NoError(t, err)
	newContents := []byte("this is some new contents")
	_, err = fd.Write(newContents)
	require.NoError(t, err)

	// rename file to "newLeaf"
	err = dir.Rename("file1", "newLeaf", rootDir)
	require.NoError(t, err)
	newItem := fstest.NewItem("newLeaf", string(newContents), item.ModTime)

	// check file has been renamed immediately in the cache
	if mode != vfscommon.CacheModeOff {
		assert.True(t, vfs.cache.Exists("newLeaf"))
	}

	// check file exists in the vfs layer at its new name
	_, err = vfs.Stat("newLeaf")
	require.NoError(t, err)

	// Close the file
	require.NoError(t, fd.Close())

	// Check file has now been renamed on the remote
	item.Path = "newLeaf"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{newItem}, nil, fs.ModTimeNotSupported)
}

func TestFileRename(t *testing.T) {
	t.Run("CacheModeOff", func(t *testing.T) {
		testFileRename(t, vfscommon.CacheModeOff)
	})
	t.Run("CacheModeFull", func(t *testing.T) {
		testFileRename(t, vfscommon.CacheModeFull)
	})
}
