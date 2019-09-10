package vfs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/djherbis/times"
	"github.com/rclone/rclone/fstest"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check CacheMode it satisfies the pflag interface
var _ pflag.Value = (*CacheMode)(nil)

func TestCacheModeString(t *testing.T) {
	assert.Equal(t, "off", CacheModeOff.String())
	assert.Equal(t, "full", CacheModeFull.String())
	assert.Equal(t, "CacheMode(17)", CacheMode(17).String())
}

func TestCacheModeSet(t *testing.T) {
	var m CacheMode

	err := m.Set("full")
	assert.NoError(t, err)
	assert.Equal(t, CacheModeFull, m)

	err = m.Set("potato")
	assert.Error(t, err, "Unknown cache mode level")

	err = m.Set("")
	assert.Error(t, err, "Unknown cache mode level")
}

func TestCacheModeType(t *testing.T) {
	var m CacheMode
	assert.Equal(t, "CacheMode", m.Type())
}

// convert c.item to a string
func itemAsString(c *cache) []string {
	c.itemMu.Lock()
	defer c.itemMu.Unlock()
	var out []string
	for name, item := range c.item {
		out = append(out, fmt.Sprintf("name=%q isFile=%v opens=%d size=%d", name, item.isFile, item.opens, item.size))
	}
	sort.Strings(out)
	return out
}

func TestCacheNew(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Disable the cache cleaner as it interferes with these tests
	opt := DefaultOpt
	opt.CachePollInterval = 0
	c, err := newCache(ctx, r.Fremote, &opt)
	require.NoError(t, err)

	assert.Contains(t, c.root, "vfs")
	assert.Contains(t, c.f.Root(), filepath.Base(r.Fremote.Root()))
	assert.Equal(t, []string(nil), itemAsString(c))

	// mkdir
	p, err := c.mkdir("potato")
	require.NoError(t, err)
	assert.Equal(t, "potato", filepath.Base(p))
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
	}, itemAsString(c))

	fi, err := os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// get
	item := c.get("potato")
	item2 := c.get("potato")
	assert.Equal(t, item, item2)
	assert.WithinDuration(t, time.Now(), item.atime, time.Second)

	// updateTime
	//.. before
	t1 := time.Now().Add(-60 * time.Minute)
	c.updateStat("potato", t1, 0)
	item = c.get("potato")
	assert.NotEqual(t, t1, item.atime)
	assert.Equal(t, 0, item.opens)
	//..after
	t2 := time.Now().Add(60 * time.Minute)
	c.updateStat("potato", t2, 0)
	item = c.get("potato")
	assert.Equal(t, t2, item.atime)
	assert.Equal(t, 0, item.opens)

	// open
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="potato" isFile=true opens=0 size=0`,
	}, itemAsString(c))
	c.open("/potato")
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=0`,
	}, itemAsString(c))
	item = c.get("potato")
	assert.WithinDuration(t, time.Now(), item.atime, time.Second)
	assert.Equal(t, 1, item.opens)

	// write the file
	err = ioutil.WriteFile(p, []byte("hello"), 0600)
	require.NoError(t, err)

	// read its atime
	fi, err = os.Stat(p)
	assert.NoError(t, err)
	atime := times.Get(fi).AccessTime()

	// updateAtimes
	item = c.get("potato")
	item.atime = time.Now().Add(-24 * time.Hour)
	err = c.updateStats()
	require.NoError(t, err)
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=5`,
	}, itemAsString(c))
	item = c.get("potato")
	assert.Equal(t, atime, item.atime)

	// updateAtimes - not in the cache
	oldItem := item
	c.itemMu.Lock()
	delete(c.item, "potato") // remove from cache
	c.itemMu.Unlock()
	err = c.updateStats()
	require.NoError(t, err)
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=0 size=5`,
	}, itemAsString(c))
	item = c.get("potato")
	assert.Equal(t, atime, item.atime)
	c.itemMu.Lock()
	c.item["potato"] = oldItem // restore to cache
	c.itemMu.Unlock()

	// try purging with file open
	c.purgeOld(10 * time.Second)
	_, err = os.Stat(p)
	assert.NoError(t, err)

	// close
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=5`,
	}, itemAsString(c))
	c.updateStat("potato", t2, 6)
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=6`,
	}, itemAsString(c))
	c.close("potato/")
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="potato" isFile=true opens=0 size=5`,
	}, itemAsString(c))
	item = c.get("potato")
	assert.WithinDuration(t, time.Now(), item.atime, time.Second)
	assert.Equal(t, 0, item.opens)

	// try purging with file closed
	c.purgeOld(10 * time.Second)
	// ...nothing should happend
	_, err = os.Stat(p)
	assert.NoError(t, err)

	//.. purge again with -ve age
	c.purgeOld(-10 * time.Second)
	_, err = os.Stat(p)
	assert.True(t, os.IsNotExist(err))

	// clean - have tested the internals already
	c.clean()

	// cleanup
	err = c.cleanUp()
	require.NoError(t, err)
	_, err = os.Stat(c.root)
	assert.True(t, os.IsNotExist(err))
}

func TestCacheOpens(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := newCache(ctx, r.Fremote, &DefaultOpt)
	require.NoError(t, err)

	assert.Equal(t, []string(nil), itemAsString(c))
	c.open("potato")
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=0`,
	}, itemAsString(c))
	c.open("potato")
	assert.Equal(t, []string{
		`name="" isFile=false opens=2 size=0`,
		`name="potato" isFile=true opens=2 size=0`,
	}, itemAsString(c))
	c.close("potato")
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=0`,
	}, itemAsString(c))
	c.close("potato")
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="potato" isFile=true opens=0 size=0`,
	}, itemAsString(c))

	c.open("potato")
	c.open("a//b/c/d/one")
	c.open("a/b/c/d/e/two")
	c.open("a/b/c/d/e/f/three")
	assert.Equal(t, []string{
		`name="" isFile=false opens=4 size=0`,
		`name="a" isFile=false opens=3 size=0`,
		`name="a/b" isFile=false opens=3 size=0`,
		`name="a/b/c" isFile=false opens=3 size=0`,
		`name="a/b/c/d" isFile=false opens=3 size=0`,
		`name="a/b/c/d/e" isFile=false opens=2 size=0`,
		`name="a/b/c/d/e/f" isFile=false opens=1 size=0`,
		`name="a/b/c/d/e/f/three" isFile=true opens=1 size=0`,
		`name="a/b/c/d/e/two" isFile=true opens=1 size=0`,
		`name="a/b/c/d/one" isFile=true opens=1 size=0`,
		`name="potato" isFile=true opens=1 size=0`,
	}, itemAsString(c))
	c.close("potato")
	c.close("a/b/c/d/one")
	c.close("a/b/c/d/e/two")
	c.close("a/b/c//d/e/f/three")
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="a" isFile=false opens=0 size=0`,
		`name="a/b" isFile=false opens=0 size=0`,
		`name="a/b/c" isFile=false opens=0 size=0`,
		`name="a/b/c/d" isFile=false opens=0 size=0`,
		`name="a/b/c/d/e" isFile=false opens=0 size=0`,
		`name="a/b/c/d/e/f" isFile=false opens=0 size=0`,
		`name="a/b/c/d/e/f/three" isFile=true opens=0 size=0`,
		`name="a/b/c/d/e/two" isFile=true opens=0 size=0`,
		`name="a/b/c/d/one" isFile=true opens=0 size=0`,
		`name="potato" isFile=true opens=0 size=0`,
	}, itemAsString(c))
}

// test the open, mkdir, purge, close, purge sequence
func TestCacheOpenMkdir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Disable the cache cleaner as it interferes with these tests
	opt := DefaultOpt
	opt.CachePollInterval = 0
	c, err := newCache(ctx, r.Fremote, &opt)
	require.NoError(t, err)

	// open
	c.open("sub/potato")

	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="sub" isFile=false opens=1 size=0`,
		`name="sub/potato" isFile=true opens=1 size=0`,
	}, itemAsString(c))

	// mkdir
	p, err := c.mkdir("sub/potato")
	require.NoError(t, err)
	assert.Equal(t, "potato", filepath.Base(p))
	assert.Equal(t, []string{
		`name="" isFile=false opens=1 size=0`,
		`name="sub" isFile=false opens=1 size=0`,
		`name="sub/potato" isFile=true opens=1 size=0`,
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
	c.close("sub/potato")

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/potato" isFile=true opens=0 size=0`,
	}, itemAsString(c))

	// clean the cache
	c.purgeOld(-10 * time.Second)
	c.purgeEmptyDirs()

	assert.Equal(t, []string(nil), itemAsString(c))

	// test directory does not exist
	fi, err = os.Stat(filepath.Dir(p))
	require.True(t, os.IsNotExist(err))
}

func TestCacheCacheDir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := newCache(ctx, r.Fremote, &DefaultOpt)
	require.NoError(t, err)

	assert.Equal(t, []string(nil), itemAsString(c))

	c.cacheDir("dir")
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="dir" isFile=false opens=0 size=0`,
	}, itemAsString(c))

	c.cacheDir("dir/sub")
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="dir" isFile=false opens=0 size=0`,
		`name="dir/sub" isFile=false opens=0 size=0`,
	}, itemAsString(c))

	c.cacheDir("dir/sub2/subsub2")
	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="dir" isFile=false opens=0 size=0`,
		`name="dir/sub" isFile=false opens=0 size=0`,
		`name="dir/sub2" isFile=false opens=0 size=0`,
		`name="dir/sub2/subsub2" isFile=false opens=0 size=0`,
	}, itemAsString(c))
}

func TestCachePurgeOld(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := newCache(ctx, r.Fremote, &DefaultOpt)
	require.NoError(t, err)

	// Test funcs
	var removed []string
	removedDir := true
	removeFile := func(name string) {
		removed = append(removed, name)
	}
	removeDir := func(name string) bool {
		if removedDir {
			removed = append(removed, name+"/")
		}
		return removedDir
	}

	removed = nil
	c._purgeOld(-10*time.Second, removeFile)
	c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string(nil), removed)

	c.open("sub/dir2/potato2")
	c.open("sub/dir/potato")
	c.close("sub/dir2/potato2")
	c.open("sub/dir/potato")

	assert.Equal(t, []string{
		`name="" isFile=false opens=2 size=0`,
		`name="sub" isFile=false opens=2 size=0`,
		`name="sub/dir" isFile=false opens=2 size=0`,
		`name="sub/dir/potato" isFile=true opens=2 size=0`,
		`name="sub/dir2" isFile=false opens=0 size=0`,
		`name="sub/dir2/potato2" isFile=true opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	removedDir = true
	c._purgeOld(-10*time.Second, removeFile)
	c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string{
		"sub/dir2/potato2",
		"sub/dir2/",
	}, removed)

	c.close("sub/dir/potato")

	removed = nil
	removedDir = true
	c._purgeOld(-10*time.Second, removeFile)
	c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string(nil), removed)

	c.close("sub/dir/potato")

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/dir" isFile=false opens=0 size=0`,
		`name="sub/dir/potato" isFile=true opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	removedDir = false
	c._purgeOld(10*time.Second, removeFile)
	c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string(nil), removed)

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/dir" isFile=false opens=0 size=0`,
		`name="sub/dir/potato" isFile=true opens=0 size=0`,
	}, itemAsString(c))

	removed = nil
	removedDir = true
	c._purgeOld(-10*time.Second, removeFile)
	c._purgeEmptyDirs(removeDir)
	assert.Equal(t, []string{
		"sub/dir/potato",
		"sub/dir/",
		"sub/",
		"/",
	}, removed)

	assert.Equal(t, []string(nil), itemAsString(c))
}

func TestCachePurgeOverQuota(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Disable the cache cleaner as it interferes with these tests
	opt := DefaultOpt
	opt.CachePollInterval = 0
	c, err := newCache(ctx, r.Fremote, &opt)
	require.NoError(t, err)

	// Test funcs
	var removed []string
	remove := func(name string) {
		removed = append(removed, name)
		c.remove(name)
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
	c.open("sub/dir/potato")
	p, err := c.mkdir("sub/dir/potato")
	require.NoError(t, err)
	err = ioutil.WriteFile(p, []byte("hello"), 0600)
	require.NoError(t, err)

	p, err = c.mkdir("sub/dir2/potato2")
	c.open("sub/dir2/potato2")
	require.NoError(t, err)
	err = ioutil.WriteFile(p, []byte("hello2"), 0600)
	require.NoError(t, err)

	assert.Equal(t, []string{
		`name="" isFile=false opens=2 size=0`,
		`name="sub" isFile=false opens=2 size=0`,
		`name="sub/dir" isFile=false opens=1 size=0`,
		`name="sub/dir/potato" isFile=true opens=1 size=0`,
		`name="sub/dir2" isFile=false opens=1 size=0`,
		`name="sub/dir2/potato2" isFile=true opens=1 size=0`,
	}, itemAsString(c))

	// Check nothing removed
	removed = nil
	c._purgeOverQuota(1, remove)
	assert.Equal(t, []string(nil), removed)

	// Close the files
	c.close("sub/dir/potato")
	c.close("sub/dir2/potato2")

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/dir" isFile=false opens=0 size=0`,
		`name="sub/dir/potato" isFile=true opens=0 size=5`,
		`name="sub/dir2" isFile=false opens=0 size=0`,
		`name="sub/dir2/potato2" isFile=true opens=0 size=6`,
	}, itemAsString(c))

	// Update the stats to read the total size
	err = c.updateStats()
	require.NoError(t, err)
	assert.Equal(t, int64(11), c.used)

	// make potato2 definitely after potato
	t1 := time.Now().Add(10 * time.Second)
	c.updateStat("sub/dir2/potato2", t1, 6)

	// Check only potato removed to get below quota
	removed = nil
	c._purgeOverQuota(10, remove)
	assert.Equal(t, []string{
		"sub/dir/potato",
	}, removed)
	assert.Equal(t, int64(6), c.used)

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/dir" isFile=false opens=0 size=0`,
		`name="sub/dir2" isFile=false opens=0 size=0`,
		`name="sub/dir2/potato2" isFile=true opens=0 size=6`,
	}, itemAsString(c))

	// Put potato back
	c.open("sub/dir/potato")
	p, err = c.mkdir("sub/dir/potato")
	require.NoError(t, err)
	err = ioutil.WriteFile(p, []byte("hello"), 0600)
	require.NoError(t, err)
	c.close("sub/dir/potato")

	// Update the stats to read the total size
	err = c.updateStats()
	require.NoError(t, err)
	assert.Equal(t, int64(11), c.used)

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/dir" isFile=false opens=0 size=0`,
		`name="sub/dir/potato" isFile=true opens=0 size=5`,
		`name="sub/dir2" isFile=false opens=0 size=0`,
		`name="sub/dir2/potato2" isFile=true opens=0 size=6`,
	}, itemAsString(c))

	// make potato definitely after potato2
	t2 := t1.Add(20 * time.Second)
	c.updateStat("sub/dir/potato", t2, 5)

	// Check only potato2 removed to get below quota
	removed = nil
	c._purgeOverQuota(10, remove)
	assert.Equal(t, []string{
		"sub/dir2/potato2",
	}, removed)
	assert.Equal(t, int64(5), c.used)
	c.purgeEmptyDirs()

	assert.Equal(t, []string{
		`name="" isFile=false opens=0 size=0`,
		`name="sub" isFile=false opens=0 size=0`,
		`name="sub/dir" isFile=false opens=0 size=0`,
		`name="sub/dir/potato" isFile=true opens=0 size=5`,
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
