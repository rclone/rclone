// Test suite for vfs

package vfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all the backends
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Some times used in the tests
var (
	t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
	t2 = fstest.Time("2011-12-25T12:59:59.123456789Z")
	t3 = fstest.Time("2011-12-30T12:59:59.000000000Z")
)

// Constants uses in the tests
const (
	writeBackDelay      = fs.Duration(100 * time.Millisecond) // A short writeback delay for testing
	waitForWritersDelay = 30 * time.Second                    // time to wait for existing writers
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// Clean up a test VFS
func cleanupVFS(t *testing.T, vfs *VFS) {
	vfs.WaitForWriters(waitForWritersDelay)
	err := vfs.CleanUp()
	require.NoError(t, err)
	vfs.Shutdown()
}

// Create a new VFS
func newTestVFSOpt(t *testing.T, opt *vfscommon.Options) (r *fstest.Run, vfs *VFS) {
	r = fstest.NewRun(t)
	vfs = New(r.Fremote, opt)
	t.Cleanup(func() {
		cleanupVFS(t, vfs)
	})
	return r, vfs
}

// Create a new VFS with default options
func newTestVFS(t *testing.T) (r *fstest.Run, vfs *VFS) {
	return newTestVFSOpt(t, nil)
}

// Check baseHandle performs as advertised
func TestVFSbaseHandle(t *testing.T) {
	fh := baseHandle{}

	err := fh.Chdir()
	assert.Equal(t, ENOSYS, err)

	err = fh.Chmod(0)
	assert.Equal(t, ENOSYS, err)

	err = fh.Chown(0, 0)
	assert.Equal(t, ENOSYS, err)

	err = fh.Close()
	assert.Equal(t, ENOSYS, err)

	fd := fh.Fd()
	assert.Equal(t, uintptr(0), fd)

	name := fh.Name()
	assert.Equal(t, "", name)

	_, err = fh.Read(nil)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.ReadAt(nil, 0)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.Readdir(0)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.Readdirnames(0)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.Seek(0, io.SeekStart)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.Stat()
	assert.Equal(t, ENOSYS, err)

	err = fh.Sync()
	assert.Equal(t, nil, err)

	err = fh.Truncate(0)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.Write(nil)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.WriteAt(nil, 0)
	assert.Equal(t, ENOSYS, err)

	_, err = fh.WriteString("")
	assert.Equal(t, ENOSYS, err)

	err = fh.Flush()
	assert.Equal(t, ENOSYS, err)

	err = fh.Release()
	assert.Equal(t, ENOSYS, err)

	node := fh.Node()
	assert.Nil(t, node)
}

// TestVFSNew sees if the New command works properly
func TestVFSNew(t *testing.T) {
	// Check active cache has this many entries
	checkActiveCacheEntries := func(i int) {
		_, count := activeCacheEntries()
		assert.Equal(t, i, count)
	}

	checkActiveCacheEntries(0)

	r, vfs := newTestVFS(t)

	// Check making a VFS with nil options
	var defaultOpt = vfscommon.Opt
	defaultOpt.Init()

	checkActiveCacheEntries(1)

	// Check that we get the same VFS if we ask for it again with
	// the same options
	vfs2 := New(r.Fremote, nil)
	assert.Equal(t, fmt.Sprintf("%p", vfs), fmt.Sprintf("%p", vfs2))

	checkActiveCacheEntries(1)

	// Shut the new VFS down and check the cache still has stuff in
	vfs2.Shutdown()

	checkActiveCacheEntries(1)

	cleanupVFS(t, vfs)

	checkActiveCacheEntries(0)
}

// TestVFSNewWithOpts sees if the New command works properly
func TestVFSNewWithOpts(t *testing.T) {
	var opt = vfscommon.Opt
	opt.DirPerms = 0777
	opt.FilePerms = 0666
	opt.Umask = 0002
	_, vfs := newTestVFSOpt(t, &opt)

	assert.Equal(t, vfscommon.FileMode(0775)|vfscommon.FileMode(os.ModeDir), vfs.Opt.DirPerms)
	assert.Equal(t, vfscommon.FileMode(0664), vfs.Opt.FilePerms)
}

// TestVFSRoot checks root directory is present and correct
func TestVFSRoot(t *testing.T) {
	_, vfs := newTestVFS(t)

	root, err := vfs.Root()
	require.NoError(t, err)
	assert.Equal(t, vfs.root, root)
	assert.True(t, root.IsDir())
	assert.Equal(t, os.FileMode(vfs.Opt.DirPerms).Perm(), root.Mode().Perm())
}

func TestVFSStat(t *testing.T) {
	r, vfs := newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2 contents", t2)
	r.CheckRemoteItems(t, file1, file2)

	node, err := vfs.Stat("file1")
	require.NoError(t, err)
	assert.True(t, node.IsFile())
	assert.Equal(t, "file1", node.Name())

	node, err = vfs.Stat("dir")
	require.NoError(t, err)
	assert.True(t, node.IsDir())
	assert.Equal(t, "dir", node.Name())

	node, err = vfs.Stat("dir/file2")
	require.NoError(t, err)
	assert.True(t, node.IsFile())
	assert.Equal(t, "file2", node.Name())

	_, err = vfs.Stat("not found")
	assert.Equal(t, os.ErrNotExist, err)

	_, err = vfs.Stat("dir/not found")
	assert.Equal(t, os.ErrNotExist, err)

	_, err = vfs.Stat("not found/not found")
	assert.Equal(t, os.ErrNotExist, err)

	_, err = vfs.Stat("file1/under a file")
	assert.Equal(t, os.ErrNotExist, err)
}

func TestVFSStatParent(t *testing.T) {
	r, vfs := newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2 contents", t2)
	r.CheckRemoteItems(t, file1, file2)

	node, leaf, err := vfs.StatParent("file1")
	require.NoError(t, err)
	assert.True(t, node.IsDir())
	assert.Equal(t, "/", node.Name())
	assert.Equal(t, "file1", leaf)

	node, leaf, err = vfs.StatParent("dir/file2")
	require.NoError(t, err)
	assert.True(t, node.IsDir())
	assert.Equal(t, "dir", node.Name())
	assert.Equal(t, "file2", leaf)

	node, leaf, err = vfs.StatParent("not found")
	require.NoError(t, err)
	assert.True(t, node.IsDir())
	assert.Equal(t, "/", node.Name())
	assert.Equal(t, "not found", leaf)

	_, _, err = vfs.StatParent("not found dir/not found")
	assert.Equal(t, os.ErrNotExist, err)

	_, _, err = vfs.StatParent("file1/under a file")
	assert.Equal(t, os.ErrExist, err)
}

func TestVFSOpenFile(t *testing.T) {
	r, vfs := newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2 contents", t2)
	r.CheckRemoteItems(t, file1, file2)

	fd, err := vfs.OpenFile("file1", os.O_RDONLY, 0777)
	require.NoError(t, err)
	assert.NotNil(t, fd)
	require.NoError(t, fd.Close())

	fd, err = vfs.OpenFile("dir", os.O_RDONLY, 0777)
	require.NoError(t, err)
	assert.NotNil(t, fd)
	require.NoError(t, fd.Close())

	fd, err = vfs.OpenFile("dir/new_file.txt", os.O_RDONLY, 0777)
	assert.Equal(t, os.ErrNotExist, err)
	assert.Nil(t, fd)

	fd, err = vfs.OpenFile("dir/new_file.txt", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	assert.NotNil(t, fd)
	err = fd.Close()
	if !errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
		require.NoError(t, err)
	}

	fd, err = vfs.OpenFile("not found/new_file.txt", os.O_WRONLY|os.O_CREATE, 0777)
	assert.Equal(t, os.ErrNotExist, err)
	assert.Nil(t, fd)
}

func TestVFSRename(t *testing.T) {
	r, vfs := newTestVFS(t)

	features := r.Fremote.Features()
	if features.Move == nil && features.Copy == nil {
		t.Skip("skip as can't rename files")
	}

	file1 := r.WriteObject(context.Background(), "dir/file2", "file2 contents", t2)
	r.CheckRemoteItems(t, file1)

	err := vfs.Rename("dir/file2", "dir/file1")
	require.NoError(t, err)
	file1.Path = "dir/file1"
	r.CheckRemoteItems(t, file1)

	err = vfs.Rename("dir/file1", "file0")
	require.NoError(t, err)
	file1.Path = "file0"
	r.CheckRemoteItems(t, file1)

	err = vfs.Rename("not found/file0", "file0")
	assert.Equal(t, os.ErrNotExist, err)

	err = vfs.Rename("file0", "not found/file0")
	assert.Equal(t, os.ErrNotExist, err)
}

func TestVFSStatfs(t *testing.T) {
	r, vfs := newTestVFS(t)

	// pre-conditions
	assert.Nil(t, vfs.usage)
	assert.True(t, vfs.usageTime.IsZero())

	aboutSupported := r.Fremote.Features().About != nil

	// read
	total, used, free := vfs.Statfs()
	if !aboutSupported {
		assert.Equal(t, int64(unknownFreeBytes), total)
		assert.Equal(t, int64(unknownFreeBytes), free)
		assert.Equal(t, int64(0), used)
		return // can't test anything else if About not supported
	}
	require.NotNil(t, vfs.usage)
	assert.False(t, vfs.usageTime.IsZero())
	if vfs.usage.Total != nil {
		assert.Equal(t, *vfs.usage.Total, total)
	} else {
		assert.True(t, total >= int64(unknownFreeBytes))
	}
	if vfs.usage.Free != nil {
		assert.Equal(t, *vfs.usage.Free, free)
	} else {
		if vfs.usage.Total != nil && vfs.usage.Used != nil {
			assert.Equal(t, free, total-used)
		} else {
			assert.True(t, free >= int64(unknownFreeBytes))
		}
	}
	if vfs.usage.Used != nil {
		assert.Equal(t, *vfs.usage.Used, used)
	} else {
		assert.Equal(t, int64(0), used)
	}

	// read cached
	oldUsage := vfs.usage
	oldTime := vfs.usageTime
	total2, used2, free2 := vfs.Statfs()
	assert.Equal(t, oldUsage, vfs.usage)
	assert.Equal(t, total, total2)
	assert.Equal(t, used, used2)
	assert.Equal(t, free, free2)
	assert.Equal(t, oldTime, vfs.usageTime)
}

func TestVFSMkdir(t *testing.T) {
	r, vfs := newTestVFS(t)

	if !r.Fremote.Features().CanHaveEmptyDirectories {
		return // can't test if can't have empty directories
	}

	r.CheckRemoteListing(t, nil, []string{})

	// Try making the root
	err := vfs.Mkdir("", 0777)
	require.NoError(t, err)
	r.CheckRemoteListing(t, nil, []string{})

	// Try making a sub directory
	err = vfs.Mkdir("a", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a"})

	// Try making an existing directory
	err = vfs.Mkdir("a", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a"})

	// Try making a new directory
	err = vfs.Mkdir("b/", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a", "b"})

	// Try making a new directory
	err = vfs.Mkdir("/c", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a", "b", "c"})

	// Try making a new directory
	err = vfs.Mkdir("/d/", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a", "b", "c", "d"})
}

func TestVFSMkdirAll(t *testing.T) {
	r, vfs := newTestVFS(t)

	if !r.Fremote.Features().CanHaveEmptyDirectories {
		return // can't test if can't have empty directories
	}

	r.CheckRemoteListing(t, nil, []string{})

	// Try making the root
	err := vfs.MkdirAll("", 0777)
	require.NoError(t, err)
	r.CheckRemoteListing(t, nil, []string{})

	// Try making a sub directory
	err = vfs.MkdirAll("a/b/c/d", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a", "a/b", "a/b/c", "a/b/c/d"})

	// Try making an existing directory
	err = vfs.MkdirAll("a/b/c", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a", "a/b", "a/b/c", "a/b/c/d"})

	// Try making an existing directory
	err = vfs.MkdirAll("/a/b/c/", 0777)
	require.NoError(t, err)

	r.CheckRemoteListing(t, nil, []string{"a", "a/b", "a/b/c", "a/b/c/d"})
}

func TestFillInMissingSizes(t *testing.T) {
	const unknownFree = 10
	for _, test := range []struct {
		total, free, used             int64
		wantTotal, wantUsed, wantFree int64
	}{
		{
			total: 20, free: 5, used: 15,
			wantTotal: 20, wantFree: 5, wantUsed: 15,
		},
		{
			total: 20, free: 5, used: -1,
			wantTotal: 20, wantFree: 5, wantUsed: 15,
		},
		{
			total: 20, free: -1, used: 15,
			wantTotal: 20, wantFree: 5, wantUsed: 15,
		},
		{
			total: 20, free: -1, used: -1,
			wantTotal: 20, wantFree: 20, wantUsed: 0,
		},
		{
			total: -1, free: 5, used: 15,
			wantTotal: 20, wantFree: 5, wantUsed: 15,
		},
		{
			total: -1, free: 15, used: -1,
			wantTotal: 15, wantFree: 15, wantUsed: 0,
		},
		{
			total: -1, free: -1, used: 15,
			wantTotal: 25, wantFree: 10, wantUsed: 15,
		},
		{
			total: -1, free: -1, used: -1,
			wantTotal: 10, wantFree: 10, wantUsed: 0,
		},
	} {
		t.Run(fmt.Sprintf("total=%d,free=%d,used=%d", test.total, test.free, test.used), func(t *testing.T) {
			gotTotal, gotUsed, gotFree := fillInMissingSizes(test.total, test.used, test.free, unknownFree)
			assert.Equal(t, test.wantTotal, gotTotal, "total")
			assert.Equal(t, test.wantUsed, gotUsed, "used")
			assert.Equal(t, test.wantFree, gotFree, "free")
		})
	}
}
