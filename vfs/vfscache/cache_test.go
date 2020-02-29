package vfscache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local" // import the local backend
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

// convert c.item to a string
func itemAsString(c *Cache) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for name, item := range c.item {
		out = append(out, fmt.Sprintf("name=%q opens=%d size=%d", filepath.ToSlash(name), item.opens, item.info.Size))
	}
	sort.Strings(out)
	return out
}

func TestCacheNew(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// FIXME need to be writing to the actual file here
	// need a item.WriteAt/item.ReadAt method I think

	// Disable the cache cleaner as it interferes with these tests
	opt := vfscommon.DefaultOpt
	opt.CachePollInterval = 0
	c, err := New(ctx, r.Fremote, &opt)
	require.NoError(t, err)

	assert.Contains(t, c.root, "vfs")
	assert.Contains(t, c.fcache.Root(), filepath.Base(r.Fremote.Root()))
	assert.Equal(t, []string(nil), itemAsString(c))

	// mkdir
	p, err := c.mkdir("potato")
	require.NoError(t, err)
	assert.Equal(t, "potato", filepath.Base(p))
	assert.Equal(t, []string(nil), itemAsString(c))

	fi, err := os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// get
	item, _ := c.get("potato")
	item2, _ := c.get("potato")
	assert.Equal(t, item, item2)
	assert.WithinDuration(t, time.Now(), item.info.ATime, time.Second)

	// open
	assert.Equal(t, []string{
		`name="potato" opens=0 size=0`,
	}, itemAsString(c))
	potato := c.Item("/potato")
	require.NoError(t, potato.Open(nil))
	assert.Equal(t, []string{
		`name="potato" opens=1 size=0`,
	}, itemAsString(c))
	assert.WithinDuration(t, time.Now(), potato.info.ATime, time.Second)
	assert.Equal(t, 1, potato.opens)

	// write the file
	require.NoError(t, potato.Truncate(5))
	atime := time.Now()
	potato.info.ATime = atime
	// err = ioutil.WriteFile(p, []byte("hello"), 0600)
	// require.NoError(t, err)

	// read its atime

	// updateAtimes
	//potato.ATime = time.Now().Add(-24 * time.Hour)

	assert.Equal(t, []string{
		`name="potato" opens=1 size=5`,
	}, itemAsString(c))
	assert.True(t, atime.Equal(potato.info.ATime), fmt.Sprintf("%v != %v", atime, potato.info.ATime))

	// try purging with file open
	c.purgeOld(10 * time.Second)
	// _, err = os.Stat(p)
	// assert.NoError(t, err)

	// close
	assert.Equal(t, []string{
		`name="potato" opens=1 size=5`,
	}, itemAsString(c))
	require.NoError(t, potato.Truncate(6))
	assert.Equal(t, []string{
		`name="potato" opens=1 size=6`,
	}, itemAsString(c))
	require.NoError(t, potato.Close(nil))
	assert.Equal(t, []string{
		`name="potato" opens=0 size=6`,
	}, itemAsString(c))
	item, _ = c.get("potato")
	assert.WithinDuration(t, time.Now(), item.info.ATime, time.Second)
	assert.Equal(t, 0, item.opens)

	// try purging with file closed
	c.purgeOld(10 * time.Second)
	// ...nothing should happend
	// _, err = os.Stat(p)
	// assert.NoError(t, err)

	//.. purge again with -ve age
	c.purgeOld(-10 * time.Second)
	_, err = os.Stat(p)
	assert.True(t, os.IsNotExist(err))

	// clean - have tested the internals already
	c.clean()

	// cleanup
	err = c.CleanUp()
	require.NoError(t, err)
	_, err = os.Stat(c.root)
	assert.True(t, os.IsNotExist(err))
}

func TestCacheOpens(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := New(ctx, r.Fremote, &vfscommon.DefaultOpt)
	require.NoError(t, err)
	defer func() { require.NoError(t, c.CleanUp()) }()

	assert.Equal(t, []string(nil), itemAsString(c))
	potato := c.Item("potato")
	require.NoError(t, potato.Open(nil))
	assert.Equal(t, []string{
		`name="potato" opens=1 size=0`,
	}, itemAsString(c))
	require.NoError(t, potato.Open(nil))
	assert.Equal(t, []string{
		`name="potato" opens=2 size=0`,
	}, itemAsString(c))
	require.NoError(t, potato.Close(nil))
	assert.Equal(t, []string{
		`name="potato" opens=1 size=0`,
	}, itemAsString(c))
	require.NoError(t, potato.Close(nil))
	assert.Equal(t, []string{
		`name="potato" opens=0 size=0`,
	}, itemAsString(c))

	require.NoError(t, potato.Open(nil))
	a1 := c.Item("a//b/c/d/one")
	a2 := c.Item("a/b/c/d/e/two")
	a3 := c.Item("a/b/c/d/e/f/three")
	require.NoError(t, a1.Open(nil))
	require.NoError(t, a2.Open(nil))
	require.NoError(t, a3.Open(nil))
	assert.Equal(t, []string{
		`name="a/b/c/d/e/f/three" opens=1 size=0`,
		`name="a/b/c/d/e/two" opens=1 size=0`,
		`name="a/b/c/d/one" opens=1 size=0`,
		`name="potato" opens=1 size=0`,
	}, itemAsString(c))
	require.NoError(t, potato.Close(nil))
	require.NoError(t, a1.Close(nil))
	require.NoError(t, a2.Close(nil))
	require.NoError(t, a3.Close(nil))
	assert.Equal(t, []string{
		`name="a/b/c/d/e/f/three" opens=0 size=0`,
		`name="a/b/c/d/e/two" opens=0 size=0`,
		`name="a/b/c/d/one" opens=0 size=0`,
		`name="potato" opens=0 size=0`,
	}, itemAsString(c))
}

// test the open, mkdir, purge, close, purge sequence
func TestCacheOpenMkdir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Disable the cache cleaner as it interferes with these tests
	opt := vfscommon.DefaultOpt
	opt.CachePollInterval = 0
	c, err := New(ctx, r.Fremote, &opt)
	require.NoError(t, err)
	defer func() { require.NoError(t, c.CleanUp()) }()

	// open
	potato := c.Item("sub/potato")
	require.NoError(t, potato.Open(nil))

	assert.Equal(t, []string{
		`name="sub/potato" opens=1 size=0`,
	}, itemAsString(c))

	// mkdir
	p, err := c.mkdir("sub/potato")
	require.NoError(t, err)
	assert.Equal(t, "potato", filepath.Base(p))
	assert.Equal(t, []string{
		`name="sub/potato" opens=1 size=0`,
	}, itemAsString(c))

	// test directory exists
	fi, err := os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// clean the cache
	c.purgeOld(-10 * time.Second)

	// test directory still exists
	fi, err = os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// close
	require.NoError(t, potato.Close(nil))

	assert.Equal(t, []string{
		`name="sub/potato" opens=0 size=0`,
	}, itemAsString(c))

	// clean the cache
	c.purgeOld(-10 * time.Second)
	c.purgeEmptyDirs()

	assert.Equal(t, []string(nil), itemAsString(c))

	// FIXME test directory does not exist
	// _, err = os.Stat(filepath.Dir(p))
	// require.True(t, os.IsNotExist(err))
}

func TestCachePurgeOld(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := New(ctx, r.Fremote, &vfscommon.DefaultOpt)
	require.NoError(t, err)
	defer func() { require.NoError(t, c.CleanUp()) }()

	// Test funcs
	var removed []string
	//removedDir := true
	removeFile := func(item *Item) {
		removed = append(removed, item.name)
		item._remove("TestCachePurgeOld")
	}
	// removeDir := func(name string) bool {
	// 	if removedDir {
	// 		removed = append(removed, filepath.ToSlash(name)+"/")
	// 	}
	// 	return removedDir
	// }

	removed = nil
	c._purgeOld(-10*time.Second, removeFile)
	// FIXME c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string(nil), removed)

	potato2 := c.Item("sub/dir2/potato2")
	require.NoError(t, potato2.Open(nil))
	potato := c.Item("sub/dir/potato")
	require.NoError(t, potato.Open(nil))
	require.NoError(t, potato2.Close(nil))
	require.NoError(t, potato.Open(nil))

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=2 size=0`,
		`name="sub/dir2/potato2" opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	// removedDir = true
	c._purgeOld(-10*time.Second, removeFile)
	// FIXME c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string{
		"sub/dir2/potato2",
	}, removed)

	require.NoError(t, potato.Close(nil))

	removed = nil
	// removedDir = true
	c._purgeOld(-10*time.Second, removeFile)
	// FIXME c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string(nil), removed)

	require.NoError(t, potato.Close(nil))

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	// removedDir = false
	c._purgeOld(10*time.Second, removeFile)
	// FIXME c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string(nil), removed)

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	// removedDir = true
	c._purgeOld(-10*time.Second, removeFile)
	// FIXME c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string{
		"sub/dir/potato",
	}, removed)

	assert.Equal(t, []string(nil), itemAsString(c))
}

func TestCachePurgeOverQuota(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Disable the cache cleaner as it interferes with these tests
	opt := vfscommon.DefaultOpt
	opt.CachePollInterval = 0
	c, err := New(ctx, r.Fremote, &opt)
	require.NoError(t, err)

	// Test funcs
	var removed []string
	remove := func(item *Item) {
		removed = append(removed, item.name)
		item._remove("TestCachePurgeOverQuota")
	}

	removed = nil
	c._purgeOverQuota(-1, remove)
	assert.Equal(t, []string(nil), removed)

	removed = nil
	c._purgeOverQuota(0, remove)
	assert.Equal(t, []string(nil), removed)

	removed = nil
	c._purgeOverQuota(1, remove)
	assert.Equal(t, []string(nil), removed)

	// Make some test files
	potato := c.Item("sub/dir/potato")
	require.NoError(t, potato.Open(nil))
	require.NoError(t, potato.Truncate(5))

	potato2 := c.Item("sub/dir2/potato2")
	require.NoError(t, potato2.Open(nil))
	require.NoError(t, potato2.Truncate(6))

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=1 size=5`,
		`name="sub/dir2/potato2" opens=1 size=6`,
	}, itemAsString(c))

	// Check nothing removed
	removed = nil
	c._purgeOverQuota(1, remove)
	assert.Equal(t, []string(nil), removed)

	// Close the files
	require.NoError(t, potato.Close(nil))
	require.NoError(t, potato2.Close(nil))

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=5`,
		`name="sub/dir2/potato2" opens=0 size=6`,
	}, itemAsString(c))

	// Update the stats to read the total size
	c.updateUsed()

	// make potato2 definitely after potato
	t1 := time.Now().Add(10 * time.Second)
	require.NoError(t, potato2.Truncate(6))
	potato2.info.ATime = t1

	// Check only potato removed to get below quota
	removed = nil
	c._purgeOverQuota(10, remove)
	assert.Equal(t, []string{
		"sub/dir/potato",
	}, removed)
	assert.Equal(t, int64(6), c.used)

	assert.Equal(t, []string{
		`name="sub/dir2/potato2" opens=0 size=6`,
	}, itemAsString(c))

	// Put potato back
	potato = c.Item("sub/dir/potato")
	require.NoError(t, potato.Open(nil))
	require.NoError(t, potato.Truncate(5))
	require.NoError(t, potato.Close(nil))

	// Update the stats to read the total size
	c.updateUsed()

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=5`,
		`name="sub/dir2/potato2" opens=0 size=6`,
	}, itemAsString(c))

	// make potato definitely after potato2
	t2 := t1.Add(20 * time.Second)
	require.NoError(t, potato.Truncate(5))
	potato.info.ATime = t2

	// Check only potato2 removed to get below quota
	removed = nil
	c._purgeOverQuota(10, remove)
	assert.Equal(t, []string{
		"sub/dir2/potato2",
	}, removed)
	assert.Equal(t, int64(5), c.used)
	c.purgeEmptyDirs()

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=5`,
	}, itemAsString(c))

	// Now purge everything
	removed = nil
	c._purgeOverQuota(1, remove)
	assert.Equal(t, []string{
		"sub/dir/potato",
	}, removed)
	assert.Equal(t, int64(0), c.used)
	c.purgeEmptyDirs()

	assert.Equal(t, []string(nil), itemAsString(c))

	// Check nothing left behind
	c.clean()
	assert.Equal(t, int64(0), c.used)
	assert.Equal(t, []string(nil), itemAsString(c))
}
