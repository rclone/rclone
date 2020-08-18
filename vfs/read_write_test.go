package vfs

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check interfaces
var (
	_ io.Reader   = (*RWFileHandle)(nil)
	_ io.ReaderAt = (*RWFileHandle)(nil)
	_ io.Writer   = (*RWFileHandle)(nil)
	_ io.WriterAt = (*RWFileHandle)(nil)
	_ io.Seeker   = (*RWFileHandle)(nil)
	_ io.Closer   = (*RWFileHandle)(nil)
	_ Handle      = (*RWFileHandle)(nil)
)

// Create a file and open it with the flags passed in
func rwHandleCreateFlags(t *testing.T, create bool, filename string, flags int) (r *fstest.Run, vfs *VFS, fh *RWFileHandle, cleanup func()) {
	opt := vfscommon.DefaultOpt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = writeBackDelay
	r, vfs, cleanup = newTestVFSOpt(t, &opt)

	if create {
		file1 := r.WriteObject(context.Background(), filename, "0123456789abcdef", t1)
		fstest.CheckItems(t, r.Fremote, file1)
	}

	h, err := vfs.OpenFile(filename, flags, 0777)
	require.NoError(t, err)
	fh, ok := h.(*RWFileHandle)
	require.True(t, ok)

	return r, vfs, fh, cleanup
}

// Open a file for read
func rwHandleCreateReadOnly(t *testing.T) (r *fstest.Run, vfs *VFS, fh *RWFileHandle, cleanup func()) {
	return rwHandleCreateFlags(t, true, "dir/file1", os.O_RDONLY)
}

// Open a file for write
func rwHandleCreateWriteOnly(t *testing.T) (r *fstest.Run, vfs *VFS, fh *RWFileHandle, cleanup func()) {
	return rwHandleCreateFlags(t, false, "file1", os.O_WRONLY|os.O_CREATE)
}

// read data from the string
func rwReadString(t *testing.T, fh *RWFileHandle, n int) string {
	buf := make([]byte, n)
	n, err := fh.Read(buf)
	if err != io.EOF {
		assert.NoError(t, err)
	}
	return string(buf[:n])
}

func TestRWFileHandleMethodsRead(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateReadOnly(t)
	defer cleanup()

	// String
	assert.Equal(t, "dir/file1 (rw)", fh.String())
	assert.Equal(t, "<nil *RWFileHandle>", (*RWFileHandle)(nil).String())
	assert.Equal(t, "<nil *RWFileHandle.file>", new(RWFileHandle).String())

	// Node
	node := fh.Node()
	assert.Equal(t, "file1", node.Name())

	// Size
	assert.Equal(t, int64(16), fh.Size())

	// Read 1
	assert.Equal(t, "0", rwReadString(t, fh, 1))

	// Read remainder
	assert.Equal(t, "123456789abcdef", rwReadString(t, fh, 256))

	// Read EOF
	buf := make([]byte, 16)
	_, err := fh.Read(buf)
	assert.Equal(t, io.EOF, err)

	// Sync
	err = fh.Sync()
	assert.NoError(t, err)

	// Stat
	var fi os.FileInfo
	fi, err = fh.Stat()
	assert.NoError(t, err)
	assert.Equal(t, int64(16), fi.Size())
	assert.Equal(t, "file1", fi.Name())

	// Close
	assert.False(t, fh.closed)
	assert.Equal(t, nil, fh.Close())
	assert.True(t, fh.closed)

	// Close again
	assert.Equal(t, ECLOSED, fh.Close())
}

func TestRWFileHandleSeek(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateReadOnly(t)
	defer cleanup()

	assert.Equal(t, fh.opened, false)

	// Check null seeks don't open the file
	n, err := fh.Seek(0, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, fh.opened, false)
	n, err = fh.Seek(0, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, fh.opened, false)

	assert.Equal(t, "0", rwReadString(t, fh, 1))

	// 0 means relative to the origin of the file,
	n, err = fh.Seek(5, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
	assert.Equal(t, "5", rwReadString(t, fh, 1))

	// 1 means relative to the current offset
	n, err = fh.Seek(-3, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.Equal(t, "3", rwReadString(t, fh, 1))

	// 2 means relative to the end.
	n, err = fh.Seek(-3, io.SeekEnd)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), n)
	assert.Equal(t, "d", rwReadString(t, fh, 1))

	// Seek off the end
	_, err = fh.Seek(100, io.SeekStart)
	assert.NoError(t, err)

	// Get the error on read
	buf := make([]byte, 16)
	l, err := fh.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, l)

	// Close
	assert.Equal(t, nil, fh.Close())
}

func TestRWFileHandleReadAt(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateReadOnly(t)
	defer cleanup()

	// read from start
	buf := make([]byte, 1)
	n, err := fh.ReadAt(buf, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "0", string(buf[:n]))

	// seek forwards
	n, err = fh.ReadAt(buf, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "5", string(buf[:n]))

	// seek backwards
	n, err = fh.ReadAt(buf, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "1", string(buf[:n]))

	// read exactly to the end
	buf = make([]byte, 6)
	n, err = fh.ReadAt(buf, 10)
	require.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "abcdef", string(buf[:n]))

	// read off the end
	buf = make([]byte, 256)
	n, err = fh.ReadAt(buf, 10)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 6, n)
	assert.Equal(t, "abcdef", string(buf[:n]))

	// read starting off the end
	n, err = fh.ReadAt(buf, 100)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	// Properly close the file
	assert.NoError(t, fh.Close())

	// check reading on closed file
	_, err = fh.ReadAt(buf, 100)
	assert.Equal(t, ECLOSED, err)
}

func TestRWFileHandleFlushRead(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateReadOnly(t)
	defer cleanup()

	// Check Flush does nothing if read not called
	err := fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Read data
	buf := make([]byte, 256)
	n, err := fh.Read(buf)
	assert.True(t, err == io.EOF || err == nil)
	assert.Equal(t, 16, n)

	// Check Flush does nothing if read called
	err = fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Check flush does nothing if called again
	err = fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Properly close the file
	assert.NoError(t, fh.Close())
}

func TestRWFileHandleReleaseRead(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateReadOnly(t)
	defer cleanup()

	// Read data
	buf := make([]byte, 256)
	n, err := fh.Read(buf)
	assert.True(t, err == io.EOF || err == nil)
	assert.Equal(t, 16, n)

	// Check Release closes file
	err = fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check Release does nothing if called again
	err = fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)
}

/// ------------------------------------------------------------

func TestRWFileHandleMethodsWrite(t *testing.T) {
	r, vfs, fh, cleanup := rwHandleCreateWriteOnly(t)
	defer cleanup()

	// String
	assert.Equal(t, "file1 (rw)", fh.String())
	assert.Equal(t, "<nil *RWFileHandle>", (*RWFileHandle)(nil).String())
	assert.Equal(t, "<nil *RWFileHandle.file>", new(RWFileHandle).String())

	// Node
	node := fh.Node()
	assert.Equal(t, "file1", node.Name())

	offset := func() int64 {
		n, err := fh.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		return n
	}

	// Offset #1
	assert.Equal(t, int64(0), offset())
	assert.Equal(t, int64(0), node.Size())

	// Size #1
	assert.Equal(t, int64(0), fh.Size())

	// Write
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Offset #2
	assert.Equal(t, int64(5), offset())
	assert.Equal(t, int64(5), node.Size())

	// Size #2
	assert.Equal(t, int64(5), fh.Size())

	// WriteString
	n, err = fh.WriteString(" world!")
	assert.NoError(t, err)
	assert.Equal(t, 7, n)

	// Sync
	err = fh.Sync()
	assert.NoError(t, err)

	// Stat
	var fi os.FileInfo
	fi, err = fh.Stat()
	assert.NoError(t, err)
	assert.Equal(t, int64(12), fi.Size())
	assert.Equal(t, "file1", fi.Name())

	// Truncate
	err = fh.Truncate(11)
	assert.NoError(t, err)

	// Close
	assert.NoError(t, fh.Close())

	// Check double close
	err = fh.Close()
	assert.Equal(t, ECLOSED, err)

	// check vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"file1,11,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello world", t1)
	vfs.WaitForWriters(waitForWritersDelay)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)
}

func TestRWFileHandleWriteAt(t *testing.T) {
	r, vfs, fh, cleanup := rwHandleCreateWriteOnly(t)
	defer cleanup()

	offset := func() int64 {
		n, err := fh.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		return n
	}

	// Preconditions
	assert.Equal(t, int64(0), offset())
	assert.True(t, fh.opened)
	assert.False(t, fh.writeCalled)

	// Write the data
	n, err := fh.WriteAt([]byte("hello**"), 0)
	assert.NoError(t, err)
	assert.Equal(t, 7, n)

	// After write
	assert.Equal(t, int64(0), offset())
	assert.True(t, fh.writeCalled)

	// Write more data
	n, err = fh.WriteAt([]byte(" world"), 5)
	assert.NoError(t, err)
	assert.Equal(t, 6, n)

	// Close
	assert.NoError(t, fh.Close())

	// Check can't write on closed handle
	n, err = fh.WriteAt([]byte("hello"), 0)
	assert.Equal(t, ECLOSED, err)
	assert.Equal(t, 0, n)

	// check vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"file1,11,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello world", t1)
	vfs.WaitForWriters(waitForWritersDelay)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)
}

func TestRWFileHandleWriteNoWrite(t *testing.T) {
	r, vfs, fh, cleanup := rwHandleCreateWriteOnly(t)
	defer cleanup()

	// Close the file without writing to it
	err := fh.Close()
	if errors.Cause(err) == fs.ErrorCantUploadEmptyFiles {
		t.Logf("skipping test: %v", err)
		return
	}
	assert.NoError(t, err)

	// Create a different file (not in the cache)
	h, err := vfs.OpenFile("file2", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)

	// Close it with Flush and Release
	err = h.Flush()
	assert.NoError(t, err)
	err = h.Release()
	assert.NoError(t, err)

	// check vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"file1,0,false", "file2,0,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "", t1)
	file2 := fstest.NewItem("file2", "", t1)
	vfs.WaitForWriters(waitForWritersDelay)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2}, []string{}, fs.ModTimeNotSupported)
}

func TestRWFileHandleFlushWrite(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateWriteOnly(t)
	defer cleanup()

	// Check that the file has been create and is open
	assert.True(t, fh.opened)

	// Write some data
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.True(t, fh.opened)

	// Check Flush does not close file if write called
	err = fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Check flush does nothing if called again
	err = fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Check that Close closes the file
	err = fh.Close()
	assert.NoError(t, err)
	assert.True(t, fh.closed)
}

func TestRWFileHandleReleaseWrite(t *testing.T) {
	_, _, fh, cleanup := rwHandleCreateWriteOnly(t)
	defer cleanup()

	// Write some data
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Check Release closes file
	err = fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check Release does nothing if called again
	err = fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)
}

// check the size of the file through the open file (if not nil) and via stat
func assertSize(t *testing.T, vfs *VFS, fh *RWFileHandle, filepath string, size int64) {
	if fh != nil {
		assert.Equal(t, size, fh.Size())
	}
	fi, err := vfs.Stat(filepath)
	require.NoError(t, err)
	assert.Equal(t, size, fi.Size())
}

func TestRWFileHandleSizeTruncateExisting(t *testing.T) {
	_, vfs, fh, cleanup := rwHandleCreateFlags(t, true, "dir/file1", os.O_WRONLY|os.O_TRUNC)
	defer cleanup()

	// check initial size after opening
	assertSize(t, vfs, fh, "dir/file1", 0)

	// write some bytes
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// check size after writing
	assertSize(t, vfs, fh, "dir/file1", 5)

	// close
	assert.NoError(t, fh.Close())

	// check size after close
	assertSize(t, vfs, nil, "dir/file1", 5)
}

func TestRWFileHandleSizeCreateExisting(t *testing.T) {
	_, vfs, fh, cleanup := rwHandleCreateFlags(t, true, "dir/file1", os.O_WRONLY|os.O_CREATE)
	defer cleanup()

	// check initial size after opening
	assertSize(t, vfs, fh, "dir/file1", 16)

	// write some bytes
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// check size after writing
	assertSize(t, vfs, fh, "dir/file1", 16)

	// write some more bytes
	n, err = fh.Write([]byte("helloHELLOhello"))
	assert.NoError(t, err)
	assert.Equal(t, 15, n)

	// check size after writing
	assertSize(t, vfs, fh, "dir/file1", 20)

	// close
	assert.NoError(t, fh.Close())

	// check size after close
	assertSize(t, vfs, nil, "dir/file1", 20)
}

func TestRWFileHandleSizeCreateNew(t *testing.T) {
	_, vfs, fh, cleanup := rwHandleCreateFlags(t, false, "file1", os.O_WRONLY|os.O_CREATE)
	defer cleanup()

	// check initial size after opening
	assertSize(t, vfs, fh, "file1", 0)

	// write some bytes
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// check size after writing
	assertSize(t, vfs, fh, "file1", 5)

	// check size after writing
	assertSize(t, vfs, fh, "file1", 5)

	// close
	assert.NoError(t, fh.Close())

	// check size after close
	assertSize(t, vfs, nil, "file1", 5)
}

func testRWFileHandleOpenTest(t *testing.T, vfs *VFS, test *openTest) {
	fileName := "open-test-file"

	// Make sure we delete the file on failure too
	defer func() {
		_ = vfs.Remove(fileName)
	}()

	// first try with file not existing
	_, err := vfs.Stat(fileName)
	require.True(t, os.IsNotExist(err))

	f, openNonExistentErr := vfs.OpenFile(fileName, test.flags, 0666)

	var readNonExistentErr error
	var writeNonExistentErr error
	if openNonExistentErr == nil {
		// read some bytes
		buf := []byte{0, 0}
		_, readNonExistentErr = f.Read(buf)

		// write some bytes
		_, writeNonExistentErr = f.Write([]byte("hello"))

		// close
		err = f.Close()
		require.NoError(t, err)
	}

	// write the file
	f, err = vfs.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = f.Write([]byte("hello"))
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	// then open file and try with file existing

	f, openExistingErr := vfs.OpenFile(fileName, test.flags, 0666)
	var readExistingErr error
	var writeExistingErr error
	if openExistingErr == nil {
		// read some bytes
		buf := []byte{0, 0}
		_, readExistingErr = f.Read(buf)

		// write some bytes
		_, writeExistingErr = f.Write([]byte("HEL"))

		// close
		err = f.Close()
		require.NoError(t, err)
	}

	// read the file
	f, err = vfs.OpenFile(fileName, os.O_RDONLY, 0)
	require.NoError(t, err)
	buf, err := ioutil.ReadAll(f)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)
	contents := string(buf)

	// remove file
	node, err := vfs.Stat(fileName)
	require.NoError(t, err)
	err = node.Remove()
	require.NoError(t, err)

	// check
	assert.Equal(t, test.openNonExistentErr, openNonExistentErr, "openNonExistentErr: want=%v, got=%v", test.openNonExistentErr, openNonExistentErr)
	assert.Equal(t, test.readNonExistentErr, readNonExistentErr, "readNonExistentErr: want=%v, got=%v", test.readNonExistentErr, readNonExistentErr)
	assert.Equal(t, test.writeNonExistentErr, writeNonExistentErr, "writeNonExistentErr: want=%v, got=%v", test.writeNonExistentErr, writeNonExistentErr)
	assert.Equal(t, test.openExistingErr, openExistingErr, "openExistingErr: want=%v, got=%v", test.openExistingErr, openExistingErr)
	assert.Equal(t, test.readExistingErr, readExistingErr, "readExistingErr: want=%v, got=%v", test.readExistingErr, readExistingErr)
	assert.Equal(t, test.writeExistingErr, writeExistingErr, "writeExistingErr: want=%v, got=%v", test.writeExistingErr, writeExistingErr)
	assert.Equal(t, test.contents, contents)
}

func TestRWFileHandleOpenTests(t *testing.T) {
	opt := vfscommon.DefaultOpt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = writeBackDelay
	_, vfs, cleanup := newTestVFSOpt(t, &opt)
	defer cleanup()

	for _, test := range openTests {
		t.Run(test.what, func(t *testing.T) {
			testRWFileHandleOpenTest(t, vfs, &test)
		})
	}
}

// tests mod time on open files
func TestRWFileModTimeWithOpenWriters(t *testing.T) {
	r, vfs, fh, cleanup := rwHandleCreateWriteOnly(t)
	defer cleanup()
	if !canSetModTime(t, r) {
		t.Skip("can't set mod time")
	}

	mtime := time.Date(2012, time.November, 18, 17, 32, 31, 0, time.UTC)

	_, err := fh.Write([]byte{104, 105})
	require.NoError(t, err)

	err = fh.Node().SetModTime(mtime)
	require.NoError(t, err)

	// Using Flush/Release to mimic mount instead of Close

	err = fh.Flush()
	require.NoError(t, err)

	err = fh.Release()
	require.NoError(t, err)

	info, err := vfs.Stat("file1")
	require.NoError(t, err)

	if r.Fremote.Precision() != fs.ModTimeNotSupported {
		// avoid errors because of timezone differences
		assert.Equal(t, info.ModTime().Unix(), mtime.Unix(), fmt.Sprintf("Time mismatch: %v != %v", info.ModTime(), mtime))
	}

	file1 := fstest.NewItem("file1", "hi", mtime)
	vfs.WaitForWriters(waitForWritersDelay)
	fstest.CheckItems(t, r.Fremote, file1)
}

func TestRWCacheRename(t *testing.T) {
	opt := vfscommon.DefaultOpt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = writeBackDelay
	r, vfs, cleanup := newTestVFSOpt(t, &opt)
	defer cleanup()

	if !operations.CanServerSideMove(r.Fremote) {
		t.Skip("skip as can't rename files")
	}

	h, err := vfs.OpenFile("rename_me", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = h.WriteString("hello")
	require.NoError(t, err)
	fh, ok := h.(*RWFileHandle)
	require.True(t, ok)

	err = fh.Sync()
	require.NoError(t, err)
	err = fh.Close()
	require.NoError(t, err)

	assert.True(t, vfs.cache.Exists("rename_me"))

	err = vfs.Rename("rename_me", "i_was_renamed")
	require.NoError(t, err)

	assert.False(t, vfs.cache.Exists("rename_me"))
	assert.True(t, vfs.cache.Exists("i_was_renamed"))
}
