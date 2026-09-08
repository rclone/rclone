package vfs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sort"
	"testing"
	"time"
	"unsafe"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dirCreate(t *testing.T) (r *fstest.Run, vfs *VFS, dir *Dir, item fstest.Item) {
	r, vfs = newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	r.CheckRemoteItems(t, file1)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	require.True(t, node.IsDir())

	return r, vfs, node.(*Dir), file1
}

func TestDirMethods(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	// String
	assert.Equal(t, "dir/", dir.String())
	assert.Equal(t, "<nil *Dir>", (*Dir)(nil).String())

	// IsDir
	assert.Equal(t, true, dir.IsDir())

	// IsFile
	assert.Equal(t, false, dir.IsFile())

	// Mode
	assert.Equal(t, os.FileMode(vfs.Opt.DirPerms), dir.Mode())

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
	_, vfs, dir, file1 := dirCreate(t)

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
	_, vfs, dir, file1 := dirCreate(t)

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
	r, vfs, _, file1 := dirCreate(t)

	file2 := r.WriteObject(context.Background(), "fil/a/b/c", "super long file", t1)
	r.CheckRemoteItems(t, file1, file2)

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
	_, vfs, dir, _ := dirCreate(t)

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
	_, _, dir, _ := dirCreate(t)

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
	r, vfs := newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2- contents", t2)
	file3 := r.WriteObject(context.Background(), "dir/subdir/file3", "file3-- contents", t3)
	r.CheckRemoteItems(t, file1, file2, file3)

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
			want = slices.Delete(want, 2, 3)
		}
		checkListing(t, dir, want)

		// Check that forgetting the root doesn't invalidate the virtual entries
		root.ForgetAll()

		checkListing(t, dir, want)
	})
}

func TestDirOpen(t *testing.T) {
	_, _, dir, _ := dirCreate(t)

	fd, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)
	_, ok := fd.(*DirHandle)
	assert.True(t, ok)
	require.NoError(t, fd.Close())

	_, err = dir.Open(os.O_WRONLY)
	assert.Equal(t, EPERM, err)
}

func TestDirCreate(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	file, err := dir.Create("potato", os.O_WRONLY|os.O_CREATE)
	require.NoError(t, err)
	assert.Equal(t, int64(0), file.Size())
	assert.True(t, dir.ModTime().After(origModTime))

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
	r, vfs, dir, file1 := dirCreate(t)

	_, err := dir.Mkdir("file1")
	assert.Error(t, err)

	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	sub, err := dir.Mkdir("sub")
	assert.NoError(t, err)
	assert.True(t, dir.ModTime().After(origModTime))

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
	r, vfs, dir, file1 := dirCreate(t)

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
	r, vfs, dir, _ := dirCreate(t)

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
	r, vfs, dir, _ := dirCreate(t)

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
	r, vfs, dir, _ := dirCreate(t)

	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	err := dir.RemoveName("file1")
	require.NoError(t, err)
	assert.True(t, dir.ModTime().After(origModTime))
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
	r, vfs, dir, file1 := dirCreate(t)

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
	origModTime := dir.ModTime()
	time.Sleep(100 * time.Millisecond) // for low rez Windows timers
	err = dir.Rename("file1", "file2", root)
	assert.NoError(t, err)
	assert.True(t, dir.ModTime().After(origModTime))
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
	// bucket-based remotes the directory won't be there

	// read only check
	vfs.Opt.ReadOnly = true
	err = dir.Rename("potato", "tuba", dir)
	assert.Equal(t, EROFS, err)

	// Rename a dir, check that key was correctly renamed in dir.parent.items
	vfs.Opt.ReadOnly = false
	_, ok := dir.parent.items["dir2"]
	assert.True(t, ok, "dir.parent.items should have 'dir2' key before rename")
	_, ok = dir.parent.items["dir3"]
	assert.False(t, ok, "dir.parent.items should not have 'dir3' key before rename")
	dir.renameTree("dir3") // rename dir2 to dir3
	_, ok = dir.parent.items["dir2"]
	assert.False(t, ok, "dir.parent.items should not have 'dir2' key after rename")
	d, ok := dir.parent.items["dir3"]
	assert.True(t, ok, fmt.Sprintf("expected to find 'dir3' key in dir.parent.items after rename, got %v", dir.parent.items))
	assert.Equal(t, dir, d, `expected renamed dir to match value of dir.parent.items["dir3"]`)
}

func TestDirStructSize(t *testing.T) {
	t.Logf("Dir struct has size %d bytes", unsafe.Sizeof(Dir{}))
}

// Check that open files appear in the directory listing properly after a forget
func TestDirFileOpen(t *testing.T) {
	_, vfs, dir, _ := dirCreate(t)

	assert.False(t, dir.hasVirtual())
	assert.False(t, dir.parent.hasVirtual())

	_, err := dir.Mkdir("sub")
	require.NoError(t, err)

	assert.True(t, dir.hasVirtual())
	assert.True(t, dir.parent.hasVirtual())

	fd0, err := vfs.Create("dir/sub/file0")
	require.NoError(t, err)
	_, err = fd0.Write([]byte("hello"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, fd0.Close())
	}()

	fd2, err := vfs.Create("dir/sub/file2")
	require.NoError(t, err)
	_, err = fd2.Write([]byte("hello world!"))
	require.NoError(t, err)
	require.NoError(t, fd2.Close())
	assert.True(t, dir.hasVirtual())

	assert.True(t, dir.hasVirtual())
	assert.True(t, dir.parent.hasVirtual())

	// Now forget the directory
	hasVirtual := dir.parent.ForgetAll()
	assert.True(t, hasVirtual)

	assert.True(t, dir.hasVirtual())
	assert.True(t, dir.parent.hasVirtual())

	// Check the files can still be found
	fi, err := vfs.Stat("dir/sub/file0")
	require.NoError(t, err)
	assert.Equal(t, int64(5), fi.Size())

	fi, err = vfs.Stat("dir/sub/file2")
	require.NoError(t, err)
	assert.Equal(t, int64(12), fi.Size())
}

func TestDirEntryModTimeInvalidation(t *testing.T) {
	r, vfs := newTestVFS(t)
	features := r.Fremote.Features()
	if !features.DirModTimeUpdatesOnWrite {
		t.Skip("Need DirModTimeUpdatesOnWrite")
	}
	if features.IsLocal && runtime.GOOS == "windows" {
		t.Skip("dirent modtime is unreliable on Windows filesystems")
	}

	r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)

	// Read the modtime of the directory fresh
	vfs.FlushDirCache()
	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	modTime1 := node.(*Dir).DirEntry().ModTime(context.Background())

	// Wait some time (we wait for Precision+10%), then write another file
	// which should update the ModTime of the directory.
	prec := (11 * vfs.f.Precision()) / 10
	time.Sleep(max(100*time.Millisecond, prec))
	r.WriteObject(context.Background(), "dir/file2", "file2 contents", t2)

	// Read the modtime of the directory fresh again - it should have changed
	vfs.FlushDirCache()
	node2, err := vfs.Stat("dir")
	require.NoError(t, err)
	modTime2 := node2.(*Dir).DirEntry().ModTime(context.Background())

	// ModTime of directory must be different after second file was written.
	if modTime1.Equal(modTime2) {
		t.Error("ModTime not invalidated")
	}
}

func TestDirMetadataExtension(t *testing.T) {
	r, vfs, dir, _ := dirCreate(t)
	root, err := vfs.Root()
	require.NoError(t, err)
	features := r.Fremote.Features()

	checkListing(t, dir, []string{"file1,14,false"})
	checkListing(t, root, []string{"dir,0,true"})

	node, err := vfs.Stat("dir/file1")
	require.NoError(t, err)
	require.True(t, node.IsFile())

	node, err = vfs.Stat("dir")
	require.NoError(t, err)
	require.True(t, node.IsDir())

	// Check metadata files do not exist
	_, err = vfs.Stat("dir/file1.metadata")
	require.Error(t, err, ENOENT)
	_, err = vfs.Stat("dir.metadata")
	require.Error(t, err, ENOENT)

	// Configure metadata extension
	vfs.Opt.MetadataExtension = ".metadata"

	// Check metadata for file does exist
	node, err = vfs.Stat("dir/file1.metadata")
	require.NoError(t, err)
	require.True(t, node.IsFile())
	size := node.Size()
	assert.Greater(t, size, int64(1))
	modTime := node.ModTime()

	// ...and is now in the listing
	checkListing(t, dir, []string{"file1,14,false", fmt.Sprintf("file1.metadata,%d,false", size)})

	// ...and is a JSON blob with correct "mtime" key
	blob, err := vfs.ReadFile("dir/file1.metadata")
	require.NoError(t, err)
	var metadata map[string]string
	err = json.Unmarshal(blob, &metadata)
	require.NoError(t, err)
	if features.ReadMetadata {
		assert.Equal(t, modTime.Format(time.RFC3339Nano), metadata["mtime"])
	}

	// Check metadata for dir does exist
	node, err = vfs.Stat("dir.metadata")
	require.NoError(t, err)
	require.True(t, node.IsFile())
	size = node.Size()
	assert.Greater(t, size, int64(1))
	modTime = node.ModTime()

	// ...and is now in the listing
	checkListing(t, root, []string{"dir,0,true", fmt.Sprintf("dir.metadata,%d,false", size)})

	// ...and is a JSON blob with correct "mtime" key
	blob, err = vfs.ReadFile("dir.metadata")
	require.NoError(t, err)
	clear(metadata)
	err = json.Unmarshal(blob, &metadata)
	require.NoError(t, err)
	if features.ReadDirMetadata {
		assert.Equal(t, modTime.Format(time.RFC3339Nano), metadata["mtime"])
	}
}

// TestDirStatLazy verifies that --vfs-lazy-dir-read uses NewObject (HeadObject
// for S3) instead of a full directory listing when stat-ing a single file.
func TestDirStatLazy(t *testing.T) {
	opt := vfscommon.Opt
	opt.LazyDirRead = true
	opt.CaseInsensitive = false // default is true on macOS/Windows; must be false for lazy stat to activate

	// Unicode normalization (NoUnicodeNormalization=false by default) also
	// triggers a full listing fallback. Disable it so the lazy path is active.
	ci := fs.GetConfig(context.Background())
	oldNorm := ci.NoUnicodeNormalization
	ci.NoUnicodeNormalization = true
	t.Cleanup(func() { ci.NoUnicodeNormalization = oldNorm })
	r, vfs := newTestVFSOpt(t, &opt)

	// Write a test file directly to the remote.
	file1 := r.WriteObject(context.Background(), "file1", "hello lazy", t1)
	r.CheckRemoteItems(t, file1)

	// stat the root — this stat triggers lazy lookup, not full listing.
	root, err := vfs.Root()
	require.NoError(t, err)

	// The directory cache is empty at this point (no full listing has run).
	root.mu.Lock()
	cacheEmpty := root.read.IsZero()
	root.mu.Unlock()
	assert.True(t, cacheEmpty, "directory cache should be empty before lazy stat")

	// Stat the single file — should use NewObject, not List.
	node, err := root.Stat("file1")
	require.NoError(t, err)
	_, ok := node.(*File)
	assert.True(t, ok)
	assert.Equal(t, "file1", node.Name())
	assert.Equal(t, int64(10), node.Size())

	// After lazy stat, the item should be in cache but d.read should still be
	// zero (no full listing has been done yet).
	root.mu.Lock()
	_, inCache := root.items["file1"]
	stillNoFullRead := root.read.IsZero()
	root.mu.Unlock()
	assert.True(t, inCache, "lazily fetched item should be in dir cache")
	assert.True(t, stillNoFullRead, "full dir listing should not have been triggered")

	// A second stat of the same file should be a pure cache hit.
	node2, err := root.Stat("file1")
	require.NoError(t, err)
	assert.Equal(t, node, node2, "second stat should return the cached node")

	// Stat of a non-existent file should return ENOENT.
	_, err = root.Stat("does-not-exist")
	assert.Equal(t, ENOENT, err)
}

// TestDirStatLazyCacheHit verifies that when a full listing has already been
// performed, a subsequent lazy stat returns the cached value without a network
// round-trip (the cache mechanism works the same in lazy mode).
func TestDirStatLazyCacheHit(t *testing.T) {
	opt := vfscommon.Opt
	opt.LazyDirRead = true
	opt.CaseInsensitive = false

	ci := fs.GetConfig(context.Background())
	oldNorm := ci.NoUnicodeNormalization
	ci.NoUnicodeNormalization = true
	t.Cleanup(func() { ci.NoUnicodeNormalization = oldNorm })
	r, vfs := newTestVFSOpt(t, &opt)

	file1 := r.WriteObject(context.Background(), "file1", "hello", t1)
	r.CheckRemoteItems(t, file1)

	root, err := vfs.Root()
	require.NoError(t, err)

	// Trigger a full listing via ReadDirAll.
	_, err = root.ReadDirAll()
	require.NoError(t, err)

	root.mu.Lock()
	fullyRead := !root.read.IsZero()
	root.mu.Unlock()
	assert.True(t, fullyRead, "ReadDirAll should mark dir as fully read")

	// Now stat should return the cached result.
	node, err := root.Stat("file1")
	require.NoError(t, err)
	assert.Equal(t, "file1", node.Name())
}

// TestDirStatLazyCaseInsensitiveFallback verifies that lazy stat is disabled
// when --vfs-case-insensitive is set OR when --ignore-case-sync is set,
// because case folding requires the full directory listing.
func TestDirStatLazyCaseInsensitiveFallback(t *testing.T) {
	opt := vfscommon.Opt
	opt.LazyDirRead = true
	opt.CaseInsensitive = true

	// Disable unicode normalization to isolate the CaseInsensitive check.
	ci := fs.GetConfig(context.Background())
	oldNorm := ci.NoUnicodeNormalization
	ci.NoUnicodeNormalization = true
	t.Cleanup(func() { ci.NoUnicodeNormalization = oldNorm })
	r, vfs := newTestVFSOpt(t, &opt)

	file1 := r.WriteObject(context.Background(), "file1", "hello ci", t1)
	r.CheckRemoteItems(t, file1)

	root, err := vfs.Root()
	require.NoError(t, err)

	// With CaseInsensitive, stat() should fall back to full _readDir().
	node, err := root.Stat("FILE1")
	require.NoError(t, err)
	assert.Equal(t, "file1", node.Name(), "case-insensitive match should work via full listing")

	// A full read must have been done.
	root.mu.Lock()
	fullyRead := !root.read.IsZero()
	root.mu.Unlock()
	assert.True(t, fullyRead, "case-insensitive stat should have triggered a full directory listing")
}

// TestDirStatLazyUnicodeNormFallback verifies that lazy stat is disabled when
// unicode normalization is active (i.e. --no-unicode-normalization is NOT set),
// because normalization matching requires the full directory listing.
func TestDirStatLazyUnicodeNormFallback(t *testing.T) {
	opt := vfscommon.Opt
	opt.LazyDirRead = true
	opt.CaseInsensitive = false // isolate unicode normalization check

	// Ensure unicode normalization is active (NoUnicodeNormalization = false).
	ci := fs.GetConfig(context.Background())
	oldNorm := ci.NoUnicodeNormalization
	ci.NoUnicodeNormalization = false // normalization ON → lazy stat must fall back
	t.Cleanup(func() { ci.NoUnicodeNormalization = oldNorm })

	r, vfs := newTestVFSOpt(t, &opt)

	file1 := r.WriteObject(context.Background(), "file1", "hello norm", t1)
	r.CheckRemoteItems(t, file1)

	root, err := vfs.Root()
	require.NoError(t, err)

	// stat() should fall back to full _readDir() because unicode normalization
	// is active (NoUnicodeNormalization == false).
	node, err := root.Stat("file1")
	require.NoError(t, err)
	assert.Equal(t, "file1", node.Name())

	root.mu.Lock()
	fullyRead := !root.read.IsZero()
	root.mu.Unlock()
	assert.True(t, fullyRead, "unicode-normalization stat should have triggered a full directory listing")
}

// statLazySetup creates a VFS with LazyDirRead=true and all normalization
// disabled, so statLazy() is guaranteed to be the active code path.
// It returns the root Dir ready for statLazy tests.
func statLazySetup(t *testing.T) (r *fstest.Run, root *Dir) {
t.Helper()
opt := vfscommon.Opt
opt.LazyDirRead = true
opt.CaseInsensitive = false
r2, vfs := newTestVFSOpt(t, &opt)

ci := fs.GetConfig(context.Background())
oldNorm := ci.NoUnicodeNormalization
ci.NoUnicodeNormalization = true
t.Cleanup(func() { ci.NoUnicodeNormalization = oldNorm })

rootDir, err := vfs.Root()
if err != nil {
t.Fatal(err)
}
return r2, rootDir
}

// TestStatLazyFileFound verifies that statLazy returns a *File node when the
// object exists, caches it in d.items, and does NOT update d.read.
func TestStatLazyFileFound(t *testing.T) {
r, root := statLazySetup(t)

r.WriteObject(context.Background(), "lazyme", "contents", t1)

node, err := root.statLazy("lazyme")
require.NoError(t, err)

_, isFile := node.(*File)
assert.True(t, isFile, "expected *File node")
assert.Equal(t, "lazyme", node.Name())
assert.Equal(t, int64(8), node.Size())

root.mu.Lock()
_, inCache := root.items["lazyme"]
readIsZero := root.read.IsZero()
root.mu.Unlock()
assert.True(t, inCache, "node should be in d.items after lazy lookup")
assert.True(t, readIsZero, "d.read must stay zero — no full listing should have occurred")
}

// TestStatLazyFileNotFound verifies that statLazy returns ENOENT for a key
// that does not exist as a file or as a virtual directory prefix.
func TestStatLazyFileNotFound(t *testing.T) {
r, root := statLazySetup(t)
_ = r

_, err := root.statLazy("does-not-exist")
assert.Equal(t, ENOENT, err)
}

// TestStatLazyDir verifies that statLazy returns a *Dir node when the leaf
// does not exist as a plain object but has children (i.e. it is a virtual
// directory prefix in a flat-namespace remote).
func TestStatLazyDir(t *testing.T) {
r, root := statLazySetup(t)

// Create a file inside a "subdirectory" so the prefix "subdir/" exists.
r.WriteObject(context.Background(), "subdir/child.txt", "hi", t1)

node, err := root.statLazy("subdir")
require.NoError(t, err)

_, isDir := node.(*Dir)
assert.True(t, isDir, "expected *Dir node for virtual directory prefix")
assert.Equal(t, "subdir", node.Name())

root.mu.Lock()
_, inCache := root.items["subdir"]
readIsZero := root.read.IsZero()
root.mu.Unlock()
assert.True(t, inCache, "Dir node should be cached in d.items")
assert.True(t, readIsZero, "d.read must stay zero — only a bounded List was issued")
}

// TestStatLazyCacheHitNoop verifies that a second call to statLazy for the
// same leaf is a pure cache hit — the node is returned from d.items with no
// additional network round-trip (d.read stays zero throughout).
func TestStatLazyCacheHitNoop(t *testing.T) {
r, root := statLazySetup(t)

r.WriteObject(context.Background(), "once", "data", t1)

// First call: populates the cache.
node1, err := root.statLazy("once")
require.NoError(t, err)

// Second call: must return the exact same node from cache.
node2, err := root.statLazy("once")
require.NoError(t, err)
assert.Equal(t, node1, node2, "second statLazy call should return the cached node")

root.mu.Lock()
readIsZero := root.read.IsZero()
root.mu.Unlock()
assert.True(t, readIsZero, "d.read must remain zero after two statLazy calls")
}

// TestStatLazySubdirFile verifies that statLazy resolves files nested inside a
// sub-directory. The VFS path join must be correct for both root-level and
// nested directories.
func TestStatLazySubdirFile(t *testing.T) {
r, root := statLazySetup(t)

r.WriteObject(context.Background(), "a/b/deep.txt", "deep", t1)

// Stat "a" from root — should be a Dir.
aNode, err := root.statLazy("a")
require.NoError(t, err)
aDir, ok := aNode.(*Dir)
assert.True(t, ok, "expected *Dir for 'a'")

// From aDir, stat "b" — should also be a Dir.
bNode, err := aDir.statLazy("b")
require.NoError(t, err)
_, ok = bNode.(*Dir)
assert.True(t, ok, "expected *Dir for 'a/b'")

// From bDir, stat "deep.txt" — should be a File.
bDir := bNode.(*Dir)
fileNode, err := bDir.statLazy("deep.txt")
require.NoError(t, err)
_, ok = fileNode.(*File)
assert.True(t, ok, "expected *File for 'a/b/deep.txt'")
assert.Equal(t, "deep.txt", fileNode.Name())
}
