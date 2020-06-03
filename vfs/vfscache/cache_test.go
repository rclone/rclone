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

// open an item and write to it
func itemWrite(t *testing.T, item *Item, contents string) {
	require.NoError(t, item.Open(nil))
	_, err := item.WriteAt([]byte(contents), 0)
	require.NoError(t, err)
}

func assertPathNotExist(t *testing.T, path string) {
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func assertPathExist(t *testing.T, path string) os.FileInfo {
	fi, err := os.Stat(path)
	assert.NoError(t, err)
	return fi
}

func newTestCacheOpt(t *testing.T, opt vfscommon.Options) (r *fstest.Run, c *Cache, cleanup func()) {
	r = fstest.NewRun(t)

	ctx, cancel := context.WithCancel(context.Background())

	c, err := New(ctx, r.Fremote, &opt)
	require.NoError(t, err)

	cleanup = func() {
		err := c.CleanUp()
		require.NoError(t, err)
		assertPathNotExist(t, c.root)
		cancel()
		r.Finalise()
	}

	return r, c, cleanup
}

func newTestCache(t *testing.T) (r *fstest.Run, c *Cache, cleanup func()) {
	opt := vfscommon.DefaultOpt

	// Disable the cache cleaner as it interferes with these tests
	opt.CachePollInterval = 0

	// Disable synchronous write
	opt.WriteBack = 0

	return newTestCacheOpt(t, opt)
}

func TestCacheNew(t *testing.T) {
	r, c, cleanup := newTestCache(t)
	defer cleanup()

	assert.Contains(t, c.root, "vfs")
	assert.Contains(t, c.fcache.Root(), filepath.Base(r.Fremote.Root()))
	assert.Equal(t, []string(nil), itemAsString(c))

	// mkdir
	p, err := c.mkdir("potato")
	require.NoError(t, err)
	assert.Equal(t, "potato", filepath.Base(p))
	assert.Equal(t, []string(nil), itemAsString(c))

	fi := assertPathExist(t, filepath.Dir(p))
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

	assert.Equal(t, []string{
		`name="potato" opens=1 size=5`,
	}, itemAsString(c))
	assert.True(t, atime.Equal(potato.info.ATime), fmt.Sprintf("%v != %v", atime, potato.info.ATime))

	// try purging with file open
	c.purgeOld(10 * time.Second)
	assertPathExist(t, p)

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
	assertPathExist(t, p)

	//.. purge again with -ve age
	c.purgeOld(-10 * time.Second)
	assertPathNotExist(t, p)

	// clean - have tested the internals already
	c.clean()
}

func TestCacheOpens(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

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
	_, c, cleanup := newTestCache(t)
	defer cleanup()

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
	fi := assertPathExist(t, filepath.Dir(p))
	assert.True(t, fi.IsDir())

	// clean the cache
	c.purgeOld(-10 * time.Second)

	// test directory still exists
	fi = assertPathExist(t, filepath.Dir(p))
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

	// test directory does not exist
	assertPathNotExist(t, filepath.Dir(p))
}

func TestCachePurgeOld(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	// Test funcs
	var removed []string
	removeFile := func(item *Item) {
		removed = append(removed, item.name)
		item.remove("TestCachePurgeOld")
	}

	removed = nil
	c._purgeOld(-10*time.Second, removeFile)
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
	c._purgeOld(-10*time.Second, removeFile)
	assert.Equal(t, []string{
		"sub/dir2/potato2",
	}, removed)

	require.NoError(t, potato.Close(nil))

	removed = nil
	c._purgeOld(-10*time.Second, removeFile)
	assert.Equal(t, []string(nil), removed)

	require.NoError(t, potato.Close(nil))

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	c._purgeOld(10*time.Second, removeFile)
	assert.Equal(t, []string(nil), removed)

	assert.Equal(t, []string{
		`name="sub/dir/potato" opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	c._purgeOld(-10*time.Second, removeFile)
	assert.Equal(t, []string{
		"sub/dir/potato",
	}, removed)

	assert.Equal(t, []string(nil), itemAsString(c))
}

func TestCachePurgeOverQuota(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	// Test funcs
	var removed []string
	remove := func(item *Item) {
		removed = append(removed, item.name)
		item.remove("TestCachePurgeOverQuota")
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
	itemWrite(t, potato, "hello")

	potato2 := c.Item("sub/dir2/potato2")
	itemWrite(t, potato2, "hello2")

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

func TestCacheInUse(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	assert.False(t, c.InUse("potato"))

	potato := c.Item("potato")

	assert.False(t, c.InUse("potato"))

	require.NoError(t, potato.Open(nil))

	assert.True(t, c.InUse("potato"))

	require.NoError(t, potato.Close(nil))

	assert.False(t, c.InUse("potato"))
}

func TestCacheDirtyItem(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	assert.Nil(t, c.DirtyItem("potato"))

	potato := c.Item("potato")

	assert.Nil(t, c.DirtyItem("potato"))

	require.NoError(t, potato.Open(nil))
	require.NoError(t, potato.Truncate(5))

	assert.Equal(t, potato, c.DirtyItem("potato"))

	require.NoError(t, potato.Close(nil))

	assert.Nil(t, c.DirtyItem("potato"))
}

func TestCacheExistsAndRemove(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	assert.False(t, c.Exists("potato"))

	potato := c.Item("potato")

	assert.False(t, c.Exists("potato"))

	require.NoError(t, potato.Open(nil))

	assert.True(t, c.Exists("potato"))

	require.NoError(t, potato.Close(nil))

	assert.True(t, c.Exists("potato"))

	c.Remove("potato")

	assert.False(t, c.Exists("potato"))

}

func TestCacheRename(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	// setup

	assert.False(t, c.Exists("potato"))
	potato := c.Item("potato")
	require.NoError(t, potato.Open(nil))
	require.NoError(t, potato.Close(nil))
	assert.True(t, c.Exists("potato"))

	osPath := c.toOSPath("potato")
	osPathMeta := c.toOSPathMeta("potato")
	assertPathExist(t, osPath)
	assertPathExist(t, osPathMeta)

	// rename potato -> newPotato

	require.NoError(t, c.Rename("potato", "newPotato", nil))
	assertPathNotExist(t, osPath)
	assertPathNotExist(t, osPathMeta)
	assert.False(t, c.Exists("potato"))

	osPath = c.toOSPath("newPotato")
	osPathMeta = c.toOSPathMeta("newPotato")
	assertPathExist(t, osPath)
	assertPathExist(t, osPathMeta)
	assert.True(t, c.Exists("newPotato"))

	// rename newPotato -> sub/newPotato

	require.NoError(t, c.Rename("newPotato", "sub/newPotato", nil))
	assertPathNotExist(t, osPath)
	assertPathNotExist(t, osPathMeta)
	assert.False(t, c.Exists("potato"))

	osPath = c.toOSPath("sub/newPotato")
	osPathMeta = c.toOSPathMeta("sub/newPotato")
	assertPathExist(t, osPath)
	assertPathExist(t, osPathMeta)
	assert.True(t, c.Exists("sub/newPotato"))

	// remove

	c.Remove("sub/newPotato")
	assertPathNotExist(t, osPath)
	assertPathNotExist(t, osPathMeta)
	assert.False(t, c.Exists("sub/newPotato"))

	// non existent file - is ignored
	assert.NoError(t, c.Rename("nonexist", "nonexist2", nil))
}

func TestCacheCleaner(t *testing.T) {
	opt := vfscommon.DefaultOpt
	opt.CachePollInterval = 10 * time.Millisecond
	opt.CacheMaxAge = 20 * time.Millisecond
	_, c, cleanup := newTestCacheOpt(t, opt)
	defer cleanup()

	time.Sleep(2 * opt.CachePollInterval)

	potato := c.Item("potato")
	potato2, found := c.get("potato")
	assert.Equal(t, fmt.Sprintf("%p", potato), fmt.Sprintf("%p", potato2))
	assert.True(t, found)

	time.Sleep(10 * opt.CachePollInterval)

	potato2, found = c.get("potato")
	assert.NotEqual(t, fmt.Sprintf("%p", potato), fmt.Sprintf("%p", potato2))
	assert.False(t, found)
}

func TestCacheSetModTime(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	t1 := time.Date(2010, 1, 2, 3, 4, 5, 9, time.UTC)

	potato := c.Item("potato")
	require.NoError(t, potato.Open(nil))
	require.NoError(t, potato.Truncate(5))
	require.NoError(t, potato.Close(nil))

	c.SetModTime("potato", t1)
	osPath := potato.c.toOSPath("potato")
	fi, err := os.Stat(osPath)
	require.NoError(t, err)

	fstest.AssertTimeEqualWithPrecision(t, "potato", t1, fi.ModTime(), time.Second)
}

func TestCacheTotaInUse(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	assert.Equal(t, int(0), c.TotalInUse())

	potato := c.Item("potato")
	assert.Equal(t, int(0), c.TotalInUse())

	require.NoError(t, potato.Open(nil))
	assert.Equal(t, int(1), c.TotalInUse())

	require.NoError(t, potato.Truncate(5))
	assert.Equal(t, int(1), c.TotalInUse())

	potato2 := c.Item("potato2")
	assert.Equal(t, int(1), c.TotalInUse())

	require.NoError(t, potato2.Open(nil))
	assert.Equal(t, int(2), c.TotalInUse())

	require.NoError(t, potato2.Close(nil))
	assert.Equal(t, int(1), c.TotalInUse())

	require.NoError(t, potato.Close(nil))
	assert.Equal(t, int(0), c.TotalInUse())
}

func TestCacheDump(t *testing.T) {
	_, c, cleanup := newTestCache(t)
	defer cleanup()

	out := (*Cache)(nil).Dump()
	assert.Equal(t, "Cache: <nil>\n", out)

	out = c.Dump()
	assert.Equal(t, "Cache{\n}\n", out)

	c.Item("potato")

	out = c.Dump()
	want := "Cache{\n\t\"potato\": "
	assert.Equal(t, want, out[:len(want)])

	c.Remove("potato")

	out = c.Dump()
	assert.Equal(t, "Cache{\n}\n", out)
}
