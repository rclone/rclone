package vfs

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dirCreate(t *testing.T) (r *fstest.Run, vfs *VFS, dir *Dir, item fstest.Item, cleanup func()) {
	r, vfs, cleanup = newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	require.True(t, node.IsDir())

	return r, vfs, node.(*Dir), file1, cleanup
}

func TestDirMethods(t *testing.T) {
	_, vfs, dir, _, cleanup := dirCreate(t)
	defer cleanup()

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

	// Path
	assert.Equal(t, "dir", dir.Path())

	// Sys
	assert.Equal(t, nil, dir.Sys())

	// SetSys
	dir.SetSys(42)
	assert.Equal(t, 42, dir.Sys())

	// Inode
	assert.NotEqual(t, uint64(0), dir.Inode())

	// Node
	assert.Equal(t, dir, dir.Node())

	// ModTime
	assert.WithinDuration(t, t1, dir.ModTime(), 100*365*24*60*60*time.Second)

	// Size
	assert.Equal(t, int64(0), dir.Size())

	// Sync
	assert.NoError(t, dir.Sync())

	// DirEntry
	assert.Equal(t, dir.entry, dir.DirEntry())

	// VFS
	assert.Equal(t, vfs, dir.VFS())
}

func TestDirForgetAll(t *testing.T) {
	_, vfs, dir, file1, cleanup := dirCreate(t)
	defer cleanup()

	// Make sure / and dir are in cache
	_, err := vfs.Stat(file1.Path)
	require.NoError(t, err)

	root, err := vfs.Root()
	require.NoError(t, err)

	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.False(t, dir.read.IsZero())

	dir.ForgetAll()
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.True(t, dir.read.IsZero())

	root.ForgetAll()
	assert.Equal(t, 0, len(root.items))
	assert.Equal(t, 0, len(dir.items))
	assert.True(t, root.read.IsZero())
}

func TestDirForgetPath(t *testing.T) {
	_, vfs, dir, file1, cleanup := dirCreate(t)
	defer cleanup()

	// Make sure / and dir are in cache
	_, err := vfs.Stat(file1.Path)
	require.NoError(t, err)

	root, err := vfs.Root()
	require.NoError(t, err)

	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.False(t, dir.read.IsZero())

	root.ForgetPath("dir/notfound", fs.EntryObject)
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 1, len(dir.items))
	assert.False(t, root.read.IsZero())
	assert.True(t, dir.read.IsZero())

	root.ForgetPath("dir", fs.EntryDirectory)
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
	assert.True(t, root.read.IsZero())

	root.ForgetPath("not/in/cache", fs.EntryDirectory)
	assert.Equal(t, 1, len(root.items))
	assert.Equal(t, 0, len(dir.items))
}

func TestDirWalk(t *testing.T) {
	r, vfs, _, file1, cleanup := dirCreate(t)
	defer cleanup()

	file2 := r.WriteObject(context.Background(), "fil/a/b/c", "super long file", t1)
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
	root.walk(fn)
	sort.Strings(result) // sort as there is a map traversal involved
	assert.Equal(t, []string{"", "dir", "fil", "fil/a", "fil/a/b"}, result)

	assert.Nil(t, root.cachedDir("not found"))
	if dir := root.cachedDir("dir"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"dir"}, result)
	}
	if dir := root.cachedDir("fil"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a", "fil"}, result)
	}
	if dir := fil.(*Dir); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a", "fil"}, result)
	}
	if dir := root.cachedDir("fil/a"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)
	}
	if dir := fil.(*Dir).cachedDir("a"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)
	}
	if dir := root.cachedDir("fil/a"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b", "fil/a"}, result)
	}
	if dir := root.cachedDir("fil/a/b"); assert.NotNil(t, dir) {
		result = nil
		dir.walk(fn)
		assert.Equal(t, []string{"fil/a/b"}, result)
	}
}

func TestDirSetModTime(t *testing.T) {
	_, vfs, dir, _, cleanup := dirCreate(t)
	defer cleanup()

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
	_, _, dir, _, cleanup := dirCreate(t)
	defer cleanup()

	node, err := dir.Stat("file1")
	require.NoError(t, err)
	_, ok := node.(*File)
	assert.True(t, ok)
	assert.Equal(t, int64(14), node.Size())
	assert.Equal(t, "file1", node.Name())

	_, err = dir.Stat("not found")
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
	r, vfs, cleanup := newTestVFS(t)
	defer cleanup()

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2- contents", t2)
	file3 := r.WriteObject(context.Background(), "dir/subdir/file3", "file3-- contents", t3)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	dir := node.(*Dir)

	checkListing(t, dir, []string{"file1,14,false", "file2,15,false", "subdir,0,true"})

	node, err = vfs.Stat("")
	require.NoError(t, err)
	root := node.(*Dir)

	checkListing(t, root, []string{"dir,0,true"})

	node, err = vfs.Stat("dir/subdir")
	require.NoError(t, err)
	subdir := node.(*Dir)

	checkListing(t, subdir, []string{"file3,16,false"})

	t.Run("Virtual", func(t *testing.T) {
		// Add some virtual entries and check what happens
		dir.AddVirtual("virtualFile", 17, false)
		dir.AddVirtual("virtualDir", 0, true)
		// Remove some existing entries
		dir.DelVirtual("file2")
		dir.DelVirtual("subdir")

		checkListing(t, dir, []string{"file1,14,false", "virtualDir,0,true", "virtualFile,17,false"})

		// Now action the deletes and uploads
		_ = r.WriteObject(context.Background(), "dir/virtualFile", "virtualFile contents", t1)
		_ = r.WriteObject(context.Background(), "dir/virtualDir/testFile", "testFile contents", t1)
		o, err := r.Fremote.NewObject(context.Background(), "dir/file2")
		require.NoError(t, err)
		require.NoError(t, o.Remove(context.Background()))
		require.NoError(t, operations.Purge(context.Background(), r.Fremote, "dir/subdir"))

		// Force a directory reload...
		dir.invalidateDir("dir")

		checkListing(t, dir, []string{"file1,14,false", "virtualDir,0,true", "virtualFile,20,false"})

		// check no virtuals left
		dir.mu.Lock()
		assert.Nil(t, dir.virtual)
		dir.mu.Unlock()

		// Add some virtual entries and check what happens
		dir.AddVirtual("virtualFile2", 100, false)
		dir.AddVirtual("virtualDir2", 0, true)
		// Remove some existing entries
		dir.DelVirtual("file1")

		checkListing(t, dir, []string{"virtualDir,0,true", "virtualDir2,0,true", "virtualFile,20,false", "virtualFile2,100,false"})

		// Force a directory reload...
		dir.invalidateDir("dir")

		want := []string{"file1,14,false", "virtualDir,0,true", "virtualDir2,0,true", "virtualFile,20,false", "virtualFile2,100,false"}
		features := r.Fremote.Features()
		if features.CanHaveEmptyDirectories {
			// snip out virtualDir2 which will only be present if can't have empty dirs
			want = append(want[:2], want[3:]...)
		}
		checkListing(t, dir, want)

		// Check that forgetting the root doesn't invalidate the virtual entries
		root.ForgetAll()

		checkListing(t, dir, want)
	})
}

func TestDirOpen(t *testing.T) {
	_, _, dir, _, cleanup := dirCreate(t)
	defer cleanup()

	fd, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)
	_, ok := fd.(*DirHandle)
	assert.True(t, ok)
	require.NoError(t, fd.Close())

	_, err = dir.Open(os.O_WRONLY)
	assert.Equal(t, EPERM, err)
}

func TestDirCreate(t *testing.T) {
	_, vfs, dir, _, cleanup := dirCreate(t)
	defer cleanup()

	file, err := dir.Create("potato", os.O_WRONLY|os.O_CREATE)
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

	// Try creating the file again - make sure we get the same file node
	file3, err := dir.Create("potato", os.O_RDWR|os.O_CREATE)
	require.NoError(t, err)
	assert.Equal(t, int64(5), file3.Size())
	assert.Equal(t, fmt.Sprintf("%p", file), fmt.Sprintf("%p", file3), "didn't return same node")

	// Test read only fs creating new
	vfs.Opt.ReadOnly = true
	_, err = dir.Create("sausage", os.O_WRONLY|os.O_CREATE)
	assert.Equal(t, EROFS, err)
}

func TestDirMkdir(t *testing.T) {
	r, vfs, dir, file1, cleanup := dirCreate(t)
	defer cleanup()

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

func TestDirMkdirSub(t *testing.T) {
	r, vfs, dir, file1, cleanup := dirCreate(t)
	defer cleanup()

	_, err := dir.Mkdir("file1")
	assert.Error(t, err)

	sub, err := dir.Mkdir("sub")
	assert.NoError(t, err)

	subsub, err := sub.Mkdir("subsub")
	assert.NoError(t, err)

	// check the vfs
	checkListing(t, dir, []string{"file1,14,false", "sub,0,true"})
	checkListing(t, sub, []string{"subsub,0,true"})
	checkListing(t, subsub, []string(nil))

	// check the underlying r.Fremote
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir", "dir/sub", "dir/sub/subsub"}, r.Fremote.Precision())

	vfs.Opt.ReadOnly = true
	_, err = dir.Mkdir("sausage")
	assert.Equal(t, EROFS, err)
}

func TestDirRemove(t *testing.T) {
	r, vfs, dir, _, cleanup := dirCreate(t)
	defer cleanup()

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
	_, err = vfs.Stat("dir")
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
	r, vfs, dir, _, cleanup := dirCreate(t)
	defer cleanup()

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
	r, vfs, dir, _, cleanup := dirCreate(t)
	defer cleanup()

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
	r, vfs, dir, file1, cleanup := dirCreate(t)
	defer cleanup()

	features := r.Fremote.Features()
	if features.DirMove == nil && features.Move == nil && features.Copy == nil {
		t.Skip("can't rename directories")
	}

	file3 := r.WriteObject(context.Background(), "dir/file3", "file3 contents!", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file3}, []string{"dir"}, r.Fremote.Precision())

	root, err := vfs.Root()
	require.NoError(t, err)

	err = dir.Rename("not found", "tuba", dir)
	assert.Equal(t, ENOENT, err)

	// Rename a directory
	err = root.Rename("dir", "dir2", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true"})
	checkListing(t, dir, []string{"file1,14,false", "file3,15,false"})

	// check the underlying r.Fremote
	file1.Path = "dir2/file1"
	file3.Path = "dir2/file3"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file3}, []string{"dir2"}, r.Fremote.Precision())

	// refetch dir
	node, err := vfs.Stat("dir2")
	assert.NoError(t, err)
	dir = node.(*Dir)

	// Rename a file
	err = dir.Rename("file1", "file2", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true", "file2,14,false"})
	checkListing(t, dir, []string{"file3,15,false"})

	// check the underlying r.Fremote
	file1.Path = "file2"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file3}, []string{"dir2"}, r.Fremote.Precision())

	// Rename a file on top of another file
	err = root.Rename("file2", "file3", dir)
	assert.NoError(t, err)
	checkListing(t, root, []string{"dir2,0,true"})
	checkListing(t, dir, []string{"file3,14,false"})

	// check the underlying r.Fremote
	file1.Path = "dir2/file3"
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{"dir2"}, r.Fremote.Precision())

	// rename an empty directory
	_, err = root.Mkdir("empty directory")
	assert.NoError(t, err)
	checkListing(t, root, []string{
		"dir2,0,true",
		"empty directory,0,true",
	})
	err = root.Rename("empty directory", "renamed empty directory", root)
	assert.NoError(t, err)
	checkListing(t, root, []string{
		"dir2,0,true",
		"renamed empty directory,0,true",
	})
	// ...we don't check the underlying f.Fremote because on
	// bucket based remotes the directory won't be there

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.Rename("potato", "tuba", dir)
	assert.Equal(t, EROFS, err)
}
