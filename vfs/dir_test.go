package vfs

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dirCreate(t *testing.T, r *fstest.Run) (*VFS, *Dir, fstest.Item) {
	vfs := New(r.Fremote, nil)

	file1 := r.WriteObject("dir/file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	require.True(t, node.IsDir())

	return vfs, node.(*Dir), file1
}

func TestDirMethods(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, _ := dirCreate(t, r)

	// String
	assert.Equal(t, "dir/", dir.String())
	assert.Equal(t, "<nil *Dir>", (*Dir)(nil).String())

	// IsDir
	assert.Equal(t, true, dir.IsDir())

	// IsFile
	assert.Equal(t, false, dir.IsFile())

	// Mode
	assert.Equal(t, vfs.Opt.DirPerms, dir.Mode())

	// Name
	assert.Equal(t, "dir", dir.Name())

	// Sys
	assert.Equal(t, nil, dir.Sys())

	// Inode
	assert.NotEqual(t, uint64(0), dir.Inode())

	// Node
	assert.Equal(t, dir, dir.Node())

	// ModTime
	assert.WithinDuration(t, t1, dir.ModTime(), 100*365*24*60*60*time.Second)

	// Size
	assert.Equal(t, int64(0), dir.Size())

	// Fsync
	assert.NoError(t, dir.Fsync())

	// DirEntry
	assert.Equal(t, dir.entry, dir.DirEntry())

	// VFS
	assert.Equal(t, vfs, dir.VFS())
}

func TestDirForgetAll(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, file1 := dirCreate(t, r)

	// Make sure / and dir are in cache
	_, err := vfs.Stat(file1.Path)
	require.NoError(t, err)

	root, err := vfs.Root()
	require.NoError(t, err)

	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))

	dir.ForgetAll()
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))

	root.ForgetAll()
	assert.Equal(t, 0, len(root.items))
	assert.Equal(t, 0, len(dir.items))
}

func TestDirForgetPath(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, file1 := dirCreate(t, r)

	// Make sure / and dir are in cache
	_, err := vfs.Stat(file1.Path)
	require.NoError(t, err)

	root, err := vfs.Root()
	require.NoError(t, err)

	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))

	root.ForgetPath("dir")
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))

	root.ForgetPath("not/in/cache")
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
}

func TestDirWalk(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, _, file1 := dirCreate(t, r)

	file2 := r.WriteObject("fil/a/b/c", "super long file", t1)
	fstest.CheckItems(t, r.Fremote, file1, file2)

	root, err := vfs.Root()
	require.NoError(t, err)

	// Forget the cache since we put another object in
	root.ForgetAll()

	// Read the directories in
	_, err = vfs.Stat("dir")
	require.NoError(t, err)
	_, err = vfs.Stat("fil/a/b")
	require.NoError(t, err)
	fil, err := vfs.Stat("fil")
	require.NoError(t, err)

	var result []string
	fn := func(d *Dir) {
		result = append(result, d.path)
	}

	result = nil
	root.walk("", fn)
	sort.Strings(result) // sort as there is a map traversal involved
	assert.Equal(t, []string{"", "dir", "fil", "fil/a", "fil/a/b"}, result)

	result = nil
	root.walk("dir", fn)
	assert.Equal(t, []string{"dir"}, result)

	result = nil
	root.walk("not found", fn)
	assert.Equal(t, []string(nil), result)

	result = nil
	root.walk("fil", fn)
	assert.Equal(t, []string{"fil/a/b", "fil/a", "fil"}, result)

	result = nil
	fil.(*Dir).walk("fil", fn)
	assert.Equal(t, []string{"fil/a/b", "fil/a", "fil"}, result)

	result = nil
	root.walk("fil/a", fn)
	assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)

	result = nil
	fil.(*Dir).walk("fil/a", fn)
	assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)

	result = nil
	root.walk("fil/a", fn)
	assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)

	result = nil
	root.walk("fil/a/b", fn)
	assert.Equal(t, []string{"fil/a/b"}, result)
}

func TestDirSetModTime(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, _ := dirCreate(t, r)

	err := dir.SetModTime(t1)
	require.NoError(t, err)
	assert.WithinDuration(t, t1, dir.ModTime(), time.Second)

	err = dir.SetModTime(t2)
	require.NoError(t, err)
	assert.WithinDuration(t, t2, dir.ModTime(), time.Second)

	vfs.Opt.ReadOnly = true
	err = dir.SetModTime(t2)
	assert.Equal(t, EROFS, err)
}

func TestDirStat(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, dir, _ := dirCreate(t, r)

	node, err := dir.Stat("file1")
	require.NoError(t, err)
	_, ok := node.(*File)
	assert.True(t, ok)
	assert.Equal(t, int64(14), node.Size())
	assert.Equal(t, "file1", node.Name())

	node, err = dir.Stat("not found")
	assert.Equal(t, ENOENT, err)
}

// This lists dir and checks the listing is as expected
func checkListing(t *testing.T, dir *Dir, want []string) {
	var got []string
	nodes, err := dir.ReadDirAll()
	require.NoError(t, err)
	for _, node := range nodes {
		got = append(got, fmt.Sprintf("%s,%d,%v", node.Name(), node.Size(), node.IsDir()))
	}
	assert.Equal(t, want, got)
}

func TestDirReadDirAll(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs := New(r.Fremote, nil)

	file1 := r.WriteObject("dir/file1", "file1 contents", t1)
	file2 := r.WriteObject("dir/file2", "file2- contents", t2)
	file3 := r.WriteObject("dir/subdir/file3", "file3-- contents", t3)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	dir := node.(*Dir)

	checkListing(t, dir, []string{"file1,14,false", "file2,15,false", "subdir,0,true"})

	node, err = vfs.Stat("")
	require.NoError(t, err)
	dir = node.(*Dir)

	checkListing(t, dir, []string{"dir,0,true"})

	node, err = vfs.Stat("dir/subdir")
	require.NoError(t, err)
	dir = node.(*Dir)

	checkListing(t, dir, []string{"file3,16,false"})
}

func TestDirOpen(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, dir, _ := dirCreate(t, r)

	fd, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)
	_, ok := fd.(*DirHandle)
	assert.True(t, ok)
	require.NoError(t, fd.Close())

	fd, err = dir.Open(os.O_WRONLY)
	assert.Equal(t, EPERM, err)
}

func TestDirCreate(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, _ := dirCreate(t, r)

	file, err := dir.Create("potato")
	require.NoError(t, err)
	assert.Equal(t, int64(0), file.Size())

	fd, err := file.Open(os.O_WRONLY | os.O_CREATE)
	require.NoError(t, err)

	// FIXME Note that this fails with the current implementation
	// until the file has been opened.

	// file2, err := vfs.Stat("dir/potato")
	// require.NoError(t, err)
	// assert.Equal(t, file, file2)

	n, err := fd.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	require.NoError(t, fd.Close())

	file2, err := vfs.Stat("dir/potato")
	require.NoError(t, err)
	assert.Equal(t, int64(5), file2.Size())

	vfs.Opt.ReadOnly = true
	_, err = dir.Create("sausage")
	assert.Equal(t, EROFS, err)
}

func TestDirMkdir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, file1 := dirCreate(t, r)

	_, err := dir.Mkdir("file1")
	assert.Error(t, err)

	sub, err := dir.Mkdir("sub")
	assert.NoError(t, err)

	// check the vfs
	checkListing(t, dir, []string{"file1,14,false", "sub,0,true"})
	checkListing(t, sub, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir", "dir/sub"}, r.Fremote.Precision())

	vfs.Opt.ReadOnly = true
	_, err = dir.Mkdir("sausage")
	assert.Equal(t, EROFS, err)
}

func TestDirRemove(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, _ := dirCreate(t, r)

	// check directory is there
	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	assert.True(t, node.IsDir())

	err = dir.Remove()
	assert.Equal(t, ENOTEMPTY, err)

	// Delete the sub file
	node, err = vfs.Stat("dir/file1")
	require.NoError(t, err)
	err = node.Remove()
	require.NoError(t, err)

	// Remove the now empty directory
	err = dir.Remove()
	require.NoError(t, err)

	// check directory is not there
	node, err = vfs.Stat("dir")
	assert.Equal(t, ENOENT, err)

	// check the vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.Remove()
	assert.Equal(t, EROFS, err)
}

func TestDirRemoveAll(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, _ := dirCreate(t, r)

	// Remove the directory and contents
	err := dir.RemoveAll()
	require.NoError(t, err)

	// check the vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.RemoveAll()
	assert.Equal(t, EROFS, err)
}

func TestDirRemoveName(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, _ := dirCreate(t, r)

	err := dir.RemoveName("file1")
	require.NoError(t, err)
	checkListing(t, dir, []string(nil))
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"dir,0,true"})

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{}, []string{"dir"}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.RemoveName("potato")
	assert.Equal(t, EROFS, err)
}

func TestDirRename(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, dir, file1 := dirCreate(t, r)

	root, err := vfs.Root()
	require.NoError(t, err)

	err = dir.Rename("not found", "tuba", dir)
	assert.Equal(t, ENOENT, err)

	// Rename a directory
	err = root.Rename("dir", "dir2", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true"})
	checkListing(t, dir, []string{"file1,14,false"})

	// check the underlying r.Fremote
	file1.Path = "dir2/file1"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir2"}, r.Fremote.Precision())

	// refetch dir
	node, err := vfs.Stat("dir2")
	assert.NoError(t, err)
	dir = node.(*Dir)

	// Rename a file
	err = dir.Rename("file1", "file2", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true", "file2,14,false"})
	checkListing(t, dir, []string(nil))

	// check the underlying r.Fremote
	file1.Path = "file2"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir2"}, r.Fremote.Precision())

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.Rename("potato", "tuba", dir)
	assert.Equal(t, EROFS, err)
}
