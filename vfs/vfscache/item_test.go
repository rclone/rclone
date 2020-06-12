package vfscache

// FIXME need to test async writeback here

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var zeroes = string(make([]byte, 100))

func newItemTestCache(t *testing.T) (r *fstest.Run, c *Cache, cleanup func()) {
	opt := vfscommon.DefaultOpt

	// Disable the cache cleaner as it interferes with these tests
	opt.CachePollInterval = 0

	// Disable synchronous write
	opt.WriteBack = 0

	return newTestCacheOpt(t, opt)
}

// Check the object has contents
func checkObject(t *testing.T, r *fstest.Run, remote string, contents string) {
	obj, err := r.Fremote.NewObject(context.Background(), remote)
	require.NoError(t, err)
	in, err := obj.Open(context.Background())
	require.NoError(t, err)
	buf, err := ioutil.ReadAll(in)
	require.NoError(t, err)
	require.NoError(t, in.Close())
	assert.Equal(t, contents, string(buf))
}

func newFileLength(t *testing.T, r *fstest.Run, c *Cache, remote string, length int) (contents string, obj fs.Object, item *Item) {
	contents = random.String(length)
	r.WriteObject(context.Background(), remote, contents, time.Now())
	item, _ = c.get(remote)
	obj, err := r.Fremote.NewObject(context.Background(), remote)
	require.NoError(t, err)
	return
}

func newFile(t *testing.T, r *fstest.Run, c *Cache, remote string) (contents string, obj fs.Object, item *Item) {
	return newFileLength(t, r, c, remote, 100)
}

func TestItemExists(t *testing.T) {
	_, c, cleanup := newItemTestCache(t)
	defer cleanup()
	item, _ := c.get("potato")

	assert.False(t, item.Exists())
	require.NoError(t, item.Open(nil))
	assert.True(t, item.Exists())
	require.NoError(t, item.Close(nil))
	assert.True(t, item.Exists())
	item.remove("test")
	assert.False(t, item.Exists())
}

func TestItemGetSize(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()
	item, _ := c.get("potato")
	require.NoError(t, item.Open(nil))

	size, err := item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	n, err := item.WriteAt([]byte("hello"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	size, err = item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(5), size)

	require.NoError(t, item.Close(nil))
	checkObject(t, r, "potato", "hello")
}

func TestItemDirty(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()
	item, _ := c.get("potato")
	require.NoError(t, item.Open(nil))

	assert.Equal(t, false, item.IsDirty())

	n, err := item.WriteAt([]byte("hello"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	assert.Equal(t, true, item.IsDirty())

	require.NoError(t, item.Close(nil))

	// Sync writeback so expect clean here
	assert.Equal(t, false, item.IsDirty())

	item.Dirty()

	assert.Equal(t, true, item.IsDirty())
	checkObject(t, r, "potato", "hello")
}

func TestItemSync(t *testing.T) {
	_, c, cleanup := newItemTestCache(t)
	defer cleanup()
	item, _ := c.get("potato")

	require.Error(t, item.Sync())

	require.NoError(t, item.Open(nil))

	require.NoError(t, item.Sync())

	require.NoError(t, item.Close(nil))
}

func TestItemTruncateNew(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()
	item, _ := c.get("potato")

	require.Error(t, item.Truncate(0))

	require.NoError(t, item.Open(nil))

	require.NoError(t, item.Truncate(100))

	size, err := item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(100), size)

	// Check the Close callback works
	callbackCalled := false
	callback := func(o fs.Object) {
		callbackCalled = true
		assert.Equal(t, "potato", o.Remote())
		assert.Equal(t, int64(100), o.Size())
	}
	require.NoError(t, item.Close(callback))
	assert.True(t, callbackCalled)

	checkObject(t, r, "potato", zeroes)
}

func TestItemTruncateExisting(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")

	require.Error(t, item.Truncate(40))
	checkObject(t, r, "existing", contents)

	require.NoError(t, item.Open(obj))

	require.NoError(t, item.Truncate(40))

	require.NoError(t, item.Truncate(60))

	require.NoError(t, item.Close(nil))

	checkObject(t, r, "existing", contents[:40]+zeroes[:20])
}

func TestItemReadAt(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")
	buf := make([]byte, 10)

	_, err := item.ReadAt(buf, 10)
	require.Error(t, err)

	require.NoError(t, item.Open(obj))

	n, err := item.ReadAt(buf, 10)
	assert.Equal(t, 10, n)
	require.NoError(t, err)
	assert.Equal(t, contents[10:20], string(buf[:n]))

	n, err = item.ReadAt(buf, 95)
	assert.Equal(t, 5, n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, contents[95:], string(buf[:n]))

	n, err = item.ReadAt(buf, 1000)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, contents[:0], string(buf[:n]))

	n, err = item.ReadAt(buf, -1)
	assert.Equal(t, 0, n)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, contents[:0], string(buf[:n]))

	require.NoError(t, item.Close(nil))
}

func TestItemWriteAtNew(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()
	item, _ := c.get("potato")
	buf := make([]byte, 10)

	_, err := item.WriteAt(buf, 10)
	require.Error(t, err)

	require.NoError(t, item.Open(nil))

	assert.Equal(t, int64(0), item.getDiskSize())

	n, err := item.WriteAt([]byte("HELLO"), 10)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	// FIXME we account for the sparse data we've "written" to
	// disk here so this is actually 5 bytes higher than expected
	assert.Equal(t, int64(15), item.getDiskSize())

	n, err = item.WriteAt([]byte("THEND"), 20)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	assert.Equal(t, int64(25), item.getDiskSize())

	require.NoError(t, item.Close(nil))

	checkObject(t, r, "potato", zeroes[:10]+"HELLO"+zeroes[:5]+"THEND")
}

func TestItemWriteAtExisting(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")

	require.NoError(t, item.Open(obj))

	n, err := item.WriteAt([]byte("HELLO"), 10)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = item.WriteAt([]byte("THEND"), 95)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = item.WriteAt([]byte("THEVERYEND"), 120)
	require.NoError(t, err)
	assert.Equal(t, 10, n)

	require.NoError(t, item.Close(nil))

	checkObject(t, r, "existing", contents[:10]+"HELLO"+contents[15:95]+"THEND"+zeroes[:20]+"THEVERYEND")
}

func TestItemLoadMeta(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")
	_ = contents

	// Open the object to create metadata for it
	require.NoError(t, item.Open(obj))
	require.NoError(t, item.Close(nil))
	info := item.info

	// Remove the item from the cache
	c.mu.Lock()
	delete(c.item, item.name)
	c.mu.Unlock()

	// Reload the item so we have to load the metadata
	item2, _ := c._get("existing")
	require.NoError(t, item2.Open(obj))
	info2 := item.info
	require.NoError(t, item2.Close(nil))

	// Check that the item is different
	assert.NotEqual(t, item, item2)
	// ... but the info is the same
	assert.Equal(t, info, info2)
}

func TestItemReload(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")
	_ = contents

	// Open the object to create metadata for it
	require.NoError(t, item.Open(obj))

	// Make it dirty
	n, err := item.WriteAt([]byte("THEENDMYFRIEND"), 95)
	require.NoError(t, err)
	assert.Equal(t, 14, n)
	assert.True(t, item.IsDirty())

	// Close the file to pacify Windows, but don't call item.Close()
	item.mu.Lock()
	require.NoError(t, item.fd.Close())
	item.fd = nil
	item.mu.Unlock()

	// Remove the item from the cache
	c.mu.Lock()
	delete(c.item, item.name)
	c.mu.Unlock()

	// Reload the item so we have to load the metadata and restart
	// the transfer
	item2, _ := c._get("existing")
	require.NoError(t, item2.reload(context.Background()))
	assert.False(t, item2.IsDirty())

	// Check that the item is different
	assert.NotEqual(t, item, item2)

	// And check the contents got written back to the remote
	checkObject(t, r, "existing", contents[:95]+"THEENDMYFRIEND")
}

func TestItemReloadRemoteGone(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")
	_ = contents

	// Open the object to create metadata for it
	require.NoError(t, item.Open(obj))

	size, err := item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(100), size)

	// Read something to instantiate the cache file
	buf := make([]byte, 10)
	_, err = item.ReadAt(buf, 10)
	require.NoError(t, err)

	// Test cache file present
	_, err = os.Stat(item.c.toOSPath(item.name))
	require.NoError(t, err)

	require.NoError(t, item.Close(nil))

	// Remove the remote object
	require.NoError(t, obj.Remove(context.Background()))

	// Re-open with no object
	require.NoError(t, item.Open(nil))

	// Check size is now 0
	size, err = item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)

	// Test cache file is now empty
	fi, err := os.Stat(item.c.toOSPath(item.name))
	require.NoError(t, err)
	assert.Equal(t, int64(0), fi.Size())

	require.NoError(t, item.Close(nil))
}

func TestItemReloadCacheStale(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()

	contents, obj, item := newFile(t, r, c, "existing")

	// Open the object to create metadata for it
	require.NoError(t, item.Open(obj))

	size, err := item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(100), size)

	// Read something to instantiate the cache file
	buf := make([]byte, 10)
	_, err = item.ReadAt(buf, 10)
	require.NoError(t, err)

	// Test cache file present
	_, err = os.Stat(item.c.toOSPath(item.name))
	require.NoError(t, err)

	require.NoError(t, item.Close(nil))

	// Update the remote to something different
	contents2, obj, item := newFileLength(t, r, c, "existing", 110)
	assert.NotEqual(t, contents, contents2)

	// Re-open with updated object
	require.NoError(t, item.Open(obj))

	// Check size is now 110
	size, err = item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(110), size)

	// Test cache file is now correct size
	fi, err := os.Stat(item.c.toOSPath(item.name))
	require.NoError(t, err)
	assert.Equal(t, int64(110), fi.Size())

	// Write to the file to make it dirty
	// This checks we aren't re-using stale data
	n, err := item.WriteAt([]byte("HELLO"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, true, item.IsDirty())

	require.NoError(t, item.Close(nil))

	// Now check with all that swizzling stuff around that the
	// object is correct

	checkObject(t, r, "existing", "HELLO"+contents2[5:])
}

func TestItemReadWrite(t *testing.T) {
	r, c, cleanup := newItemTestCache(t)
	defer cleanup()
	const (
		size     = 50*1024*1024 + 123
		fileName = "large"
	)

	item, _ := c.get(fileName)
	require.NoError(t, item.Open(nil))

	// Create the test file
	in := readers.NewPatternReader(size)
	buf := make([]byte, 1024*1024)
	buf2 := make([]byte, 1024*1024)
	offset := int64(0)
	for {
		n, err := in.Read(buf)
		n2, err2 := item.WriteAt(buf[:n], offset)
		offset += int64(n2)
		require.NoError(t, err2)
		require.Equal(t, n, n2)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}

	// Check it is the right size
	readSize, err := item.GetSize()
	require.NoError(t, err)
	assert.Equal(t, int64(size), readSize)

	require.NoError(t, item.Close(nil))

	assert.False(t, item.remove(fileName))

	obj, err := r.Fremote.NewObject(context.Background(), fileName)
	require.NoError(t, err)
	assert.Equal(t, int64(size), obj.Size())

	// read and check a block of size N at offset
	// It returns eof true if the end of file has been reached
	readCheckBuf := func(t *testing.T, in io.ReadSeeker, buf, buf2 []byte, item *Item, offset int64, N int) (n int, eof bool) {
		what := fmt.Sprintf("buf=%p, buf2=%p, item=%p, offset=%d, N=%d", buf, buf2, item, offset, N)
		n, err := item.ReadAt(buf, offset)

		_, err2 := in.Seek(offset, io.SeekStart)
		require.NoError(t, err2, what)
		n2, err2 := in.Read(buf2[:n])
		require.Equal(t, n, n2, what)
		assert.Equal(t, buf[:n], buf2[:n2], what)
		assert.Equal(t, buf[:n], buf2[:n2], what)

		if err == io.EOF {
			return n, true
		}
		require.NoError(t, err, what)
		require.NoError(t, err2, what)
		return n, false
	}
	readCheck := func(t *testing.T, item *Item, offset int64, N int) (n int, eof bool) {
		return readCheckBuf(t, in, buf, buf2, item, offset, N)
	}

	// Read it back sequentially
	t.Run("Sequential", func(t *testing.T) {
		require.NoError(t, item.Open(obj))
		assert.False(t, item.present())
		offset := int64(0)
		for {
			n, eof := readCheck(t, item, offset, len(buf))
			offset += int64(n)
			if eof {
				break
			}
		}
		assert.Equal(t, int64(size), offset)
		require.NoError(t, item.Close(nil))
		assert.False(t, item.remove(fileName))
	})

	// Read it back randomly
	t.Run("Random", func(t *testing.T) {
		require.NoError(t, item.Open(obj))
		assert.False(t, item.present())
		for !item.present() {
			blockSize := rand.Intn(len(buf))
			offset := rand.Int63n(size+2*int64(blockSize)) - int64(blockSize)
			if offset < 0 {
				offset = 0
			}
			_, _ = readCheck(t, item, offset, blockSize)
		}
		require.NoError(t, item.Close(nil))
		assert.False(t, item.remove(fileName))
	})

	// Read it back randomly concurently
	t.Run("RandomConcurrent", func(t *testing.T) {
		require.NoError(t, item.Open(obj))
		assert.False(t, item.present())
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				in := readers.NewPatternReader(size)
				buf := make([]byte, 1024*1024)
				buf2 := make([]byte, 1024*1024)
				for !item.present() {
					blockSize := rand.Intn(len(buf))
					offset := rand.Int63n(size+2*int64(blockSize)) - int64(blockSize)
					if offset < 0 {
						offset = 0
					}
					_, _ = readCheckBuf(t, in, buf, buf2, item, offset, blockSize)
				}
			}()
		}
		wg.Wait()
		require.NoError(t, item.Close(nil))
		assert.False(t, item.remove(fileName))
	})

	// Read it back in reverse which creates the maximum number of
	// downloaders
	t.Run("Reverse", func(t *testing.T) {
		require.NoError(t, item.Open(obj))
		assert.False(t, item.present())
		offset := int64(size)
		for {
			blockSize := len(buf)
			offset -= int64(blockSize)
			if offset < 0 {
				offset = 0
				blockSize += int(offset)
			}
			_, _ = readCheck(t, item, offset, blockSize)
			if offset == 0 {
				break
			}
		}
		require.NoError(t, item.Close(nil))
		assert.False(t, item.remove(fileName))
	})
}
