package vfs

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fileCreate(t *testing.T, r *fstest.Run) (*VFS, *File, fstest.Item) {
	vfs := New(r.Fremote, nil)

	file1 := r.WriteObject("dir/file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	node, err := vfs.Stat("dir/file1")
	require.NoError(t, err)
	require.True(t, node.IsFile())

	return vfs, node.(*File), file1
}

func TestFileMethods(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r)

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

	// Sys
	assert.Equal(t, nil, file.Sys())

	// Inode
	assert.NotEqual(t, uint64(0), file.Inode())

	// Node
	assert.Equal(t, file, file.Node())

	// ModTime
	assert.WithinDuration(t, t1, file.ModTime(), r.Fremote.Precision())

	// Size
	assert.Equal(t, int64(14), file.Size())

	// Fsync
	assert.NoError(t, file.Fsync())

	// DirEntry
	assert.Equal(t, file.o, file.DirEntry())

	// Dir
	assert.Equal(t, file.d, file.Dir())

	// VFS
	assert.Equal(t, vfs, file.VFS())
}

func TestFileSetModTime(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, file1 := fileCreate(t, r)

	err := file.SetModTime(t2)
	require.NoError(t, err)

	file1.ModTime = t2
	fstest.CheckItems(t, r.Fremote, file1)

	vfs.Opt.ReadOnly = true
	err = file.SetModTime(t2)
	assert.Equal(t, EROFS, err)
}

func TestFileOpenRead(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, file, _ := fileCreate(t, r)

	fd, err := file.OpenRead()
	require.NoError(t, err)

	contents, err := ioutil.ReadAll(fd)
	require.NoError(t, err)
	assert.Equal(t, "file1 contents", string(contents))

	require.NoError(t, fd.Close())
}

func TestFileOpenWrite(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r)

	fd, err := file.OpenWrite()
	require.NoError(t, err)

	newContents := []byte("this is some new contents")
	n, err := fd.Write(newContents)
	require.NoError(t, err)
	assert.Equal(t, len(newContents), n)
	require.NoError(t, fd.Close())

	assert.Equal(t, int64(25), file.Size())

	vfs.Opt.ReadOnly = true
	_, err = file.OpenWrite()
	assert.Equal(t, EROFS, err)
}

func TestFileRemove(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, file, _ := fileCreate(t, r)

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
	vfs, file, _ := fileCreate(t, r)

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
	_, file, _ := fileCreate(t, r)

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
	assert.Equal(t, EPERM, err)

	fd, err = file.Open(3)
	assert.Equal(t, EPERM, err)
}
