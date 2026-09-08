package vfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

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
func rwHandleCreateFlags(t *testing.T, create bool, filename string, flags int) (r *fstest.Run, vfs *VFS, fh *RWFileHandle) {
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = writeBackDelay
	r, vfs = newTestVFSOpt(t, &opt)

	if create {
		file1 := r.WriteObject(context.Background(), filename, "0123456789abcdef", t1)
		r.CheckRemoteItems(t, file1)
	}

	h, err := vfs.OpenFile(filename, flags, 0777)
	require.NoError(t, err)
	fh, ok := h.(*RWFileHandle)
	require.True(t, ok)

	return r, vfs, fh
}

// Open a file for read
func rwHandleCreateReadOnly(t *testing.T) (r *fstest.Run, vfs *VFS, fh *RWFileHandle) {
	return rwHandleCreateFlags(t, true, "dir/file1", os.O_RDONLY)
}

// Open a file for write
func rwHandleCreateWriteOnly(t *testing.T) (r *fstest.Run, vfs *VFS, fh *RWFileHandle) {
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
	_, _, fh := rwHandleCreateReadOnly(t)

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
	_, _, fh := rwHandleCreateReadOnly(t)

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
	_, _, fh := rwHandleCreateReadOnly(t)

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
	_, _, fh := rwHandleCreateReadOnly(t)

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
	_, _, fh := rwHandleCreateReadOnly(t)

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
	r, vfs, fh := rwHandleCreateWriteOnly(t)

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
	r, vfs, fh := rwHandleCreateWriteOnly(t)

	offset := func() int64 {
		n, err := fh.Seek(0, io.SeekCurrent)
		require.NoError(t, err)
		return n
	}

	// Name
	assert.Equal(t, "file1", fh.Name())

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
	r, vfs, fh := rwHandleCreateWriteOnly(t)

	// Close the file without writing to it
	err := fh.Close()
	if errors.Is(err, fs.ErrorCantUploadEmptyFiles) {
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
	_, _, fh := rwHandleCreateWriteOnly(t)

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
	_, _, fh := rwHandleCreateWriteOnly(t)

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
	_, vfs, fh := rwHandleCreateFlags(t, true, "dir/file1", os.O_WRONLY|os.O_TRUNC)

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
	_, vfs, fh := rwHandleCreateFlags(t, true, "dir/file1", os.O_WRONLY|os.O_CREATE)

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
	_, vfs, fh := rwHandleCreateFlags(t, false, "file1", os.O_WRONLY|os.O_CREATE)

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
	buf, err := io.ReadAll(f)
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
	for _, cacheMode := range []vfscommon.CacheMode{vfscommon.CacheModeWrites, vfscommon.CacheModeFull} {
		t.Run(cacheMode.String(), func(t *testing.T) {
			opt := vfscommon.Opt
			opt.CacheMode = cacheMode
			opt.WriteBack = writeBackDelay
			_, vfs := newTestVFSOpt(t, &opt)

			for _, test := range openTests {
				t.Run(test.what, func(t *testing.T) {
					testRWFileHandleOpenTest(t, vfs, &test)
				})
			}
		})
	}
}

// tests mod time on open files
func TestRWFileModTimeWithOpenWriters(t *testing.T) {
	r, vfs, fh := rwHandleCreateWriteOnly(t)
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
	r.CheckRemoteItems(t, file1)
}

func TestRWCacheRename(t *testing.T) {
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = writeBackDelay
	r, vfs := newTestVFSOpt(t, &opt)

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

// Test the cache reading a file that is updated externally
//
// See: https://github.com/rclone/rclone/issues/6053
func TestRWCacheUpdate(t *testing.T) {
	opt := vfscommon.Opt
	opt.CacheMode = vfscommon.CacheModeFull
	opt.WriteBack = writeBackDelay
	opt.DirCacheTime = fs.Duration(100 * time.Millisecond)
	opt.HandleCaching = 0
	r, vfs := newTestVFSOpt(t, &opt)

	if r.Fremote.Precision() == fs.ModTimeNotSupported {
		t.Skip("skip as modtime not supported")
	}

	const filename = "TestRWCacheUpdate"

	modTime := time.Now().Add(-time.Hour)
	for i := range 10 {
		modTime = modTime.Add(time.Minute)
		// Refresh test file
		contents := fmt.Sprintf("TestRWCacheUpdate%03d", i)
		// Increase the size for second half of test
		for j := 5; j < i; j++ {
			contents += "*"
		}
		file1 := r.WriteObject(context.Background(), filename, contents, modTime)
		r.CheckRemoteItems(t, file1)

		// Wait for directory cache to expire
		time.Sleep(time.Duration(2 * opt.DirCacheTime))

		// Check the file is OK via the VFS
		data, err := vfs.ReadFile(filename)
		require.NoError(t, err)
		require.Equal(t, contents, string(data))

		// Check Stat
		fi, err := vfs.Stat(filename)
		require.NoError(t, err)
		assert.Equal(t, int64(len(contents)), fi.Size())
		fstest.AssertTimeEqualWithPrecision(t, filename, modTime, fi.ModTime(), r.Fremote.Precision())
	}
}

// recordingFs wraps an fs.Fs and records the modtime of every source
// passed to Put. This lets tests see exactly what modtime the upload
// carries to the server, instead of the final state after the remote's
// SetModTime fallback has had a chance to correct it (which masks the
// issue - see #9637 where Nextcloud cannot fall back at all).
type recordingFs struct {
	fs.Fs
	mu   sync.Mutex
	puts []time.Time
}

func (f *recordingFs) record(mt time.Time) {
	f.mu.Lock()
	f.puts = append(f.puts, mt)
	f.mu.Unlock()
}

func (f *recordingFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	f.record(src.ModTime(ctx))
	return f.Fs.Put(ctx, in, src, options...)
}

func (f *recordingFs) putTimes() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.puts...)
}

// Features strips the server-side copy and move features so uploads go
// through Put, which is where the modtime is recorded. Server-side
// copies bypass Put entirely.
func (f *recordingFs) Features() *fs.Features {
	features := *f.Fs.Features()
	cp, mv := features.Copy, features.Move
	features.Copy = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		f.record(src.ModTime(ctx))
		return cp(ctx, src, remote)
	}
	features.Move = func(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
		f.record(src.ModTime(ctx))
		return mv(ctx, src, remote)
	}
	return &features
}

// newRecordingVFS creates a VFS backed by a recordingFs so tests can
// observe the modtime each upload carries to the remote.
func newRecordingVFS(t *testing.T, opt *vfscommon.Options) (r *fstest.Run, vfs *VFS, rec *recordingFs) {
	r = fstest.NewRun(t)
	rec = &recordingFs{Fs: r.Fremote}
	vfs = New(context.Background(), rec, opt)
	t.Cleanup(func() {
		cleanupVFS(t, vfs)
	})
	return r, vfs, rec
}

// Test that the modtime set by a writer (e.g. an editor calling
// utimens) is the one which reaches the remote, not the old modtime
// of the file - see issue #9637
func TestRWFileHandleModTimeAfterRewrite(t *testing.T) {
	r, vfs, rec := newRecordingVFS(t, &vfscommon.Options{
		CacheMode: vfscommon.CacheModeFull,
		WriteBack: writeBackDelay,
	})
	ctx := context.Background()

	// Create the file with some content and let it settle on the remote
	fh, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = fh.Write([]byte("old content"))
	require.NoError(t, err)
	require.NoError(t, fh.Close())
	vfs.WaitForWriters(waitForWritersDelay)

	// Give the remote object an old modtime, like a file which has
	// been on the remote for a while
	oldTime := time.Now().Add(-time.Hour).Truncate(time.Second)
	obj, err := r.Fremote.NewObject(ctx, "file1")
	require.NoError(t, err)
	require.NoError(t, obj.SetModTime(ctx, oldTime))

	// Reopen the file for read-write like an editor would, write new
	// content and set a fresh modtime (as the kernel does for utimens)
	h, err := vfs.OpenFile("file1", os.O_RDWR, 0777)
	require.NoError(t, err)
	rw, ok := h.(*RWFileHandle)
	require.True(t, ok)

	_, err = rw.Write([]byte("new content"))
	require.NoError(t, err)

	newTime := time.Now().Truncate(time.Second)
	require.NoError(t, rw.file.SetModTime(newTime))

	require.NoError(t, rw.Close())
	vfs.WaitForWriters(waitForWritersDelay)

	// The upload which carried the new content must have carried the
	// new modtime as well
	puts := rec.putTimes()
	require.NotEmpty(t, puts)
	got := puts[len(puts)-1]
	assert.False(t, got.Equal(oldTime), "upload carried modtime %v instead of the new time %v", got, newTime)
	assert.WithinDuration(t, newTime, got, time.Second)
}

// TestRWFileHandleModTimeAfterRewriteReopen reproduces the LibreOffice
// save flow from issue #9637: write the file, close it (which queues the
// writeback), then reopen it without writing and close again while the
// upload is still pending. The second close must not clobber the cache
// file modtime with the old remote modtime, otherwise the pending upload
// pushes the stale time to the server.
func TestRWFileHandleModTimeAfterRewriteReopen(t *testing.T) {
	r, vfs, _ := newRecordingVFS(t, &vfscommon.Options{
		CacheMode: vfscommon.CacheModeFull,
		WriteBack: fs.Duration(5 * time.Second),
	})
	ctx := context.Background()

	// 1. Create the file, let the first upload settle on the remote
	fh, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = fh.Write([]byte("old content"))
	require.NoError(t, err)
	require.NoError(t, fh.Close())
	vfs.WaitForWriters(waitForWritersDelay)

	// 2. Give the remote object an old modtime, like a file which has
	// been on the remote for a while
	oldTime := time.Now().Add(-time.Hour).Truncate(time.Second)
	obj, err := r.Fremote.NewObject(ctx, "file1")
	require.NoError(t, err)
	require.NoError(t, obj.SetModTime(ctx, oldTime))

	// 3. Rewrite the file: write new content, set a fresh modtime (as
	// the kernel does for utimens) and close. This queues the writeback
	// which fires in 5s.
	h, err := vfs.OpenFile("file1", os.O_RDWR, 0777)
	require.NoError(t, err)
	rw, ok := h.(*RWFileHandle)
	require.True(t, ok)
	_, err = rw.Write([]byte("new content"))
	require.NoError(t, err)
	newTime := time.Now().Truncate(time.Second)
	require.NoError(t, rw.file.SetModTime(newTime))
	require.NoError(t, rw.Close())

	// 4. Reopen without writing and close again (LibreOffice reopens the
	// file to verify it after saving). This must not clobber the pending
	// upload's modtime.
	h2, err := vfs.OpenFile("file1", os.O_RDWR, 0777)
	require.NoError(t, err)
	rw2, ok := h2.(*RWFileHandle)
	require.True(t, ok)
	require.NoError(t, rw2.Close())
	vfs.WaitForWriters(waitForWritersDelay)

	// 5. The upload which carried the new content to the remote must
	// have carried the new modtime as well - the remote must not be
	// left with the old modtime (issue #9637).
	obj, err = r.Fremote.NewObject(ctx, "file1")
	require.NoError(t, err)
	got := obj.ModTime(ctx)
	assert.False(t, got.Equal(oldTime), "remote modtime is %v instead of the new time %v", got, newTime)
	assert.WithinDuration(t, newTime, got, time.Second)
}

// Same as above but with a rename in between, like LibreOffice's
// safe-save: write the new content into a temp file, then rename it over
// the target while the writeback is still queued.
func TestRWFileHandleModTimeAfterRewriteRename(t *testing.T) {
	r, vfs, _ := newRecordingVFS(t, &vfscommon.Options{
		CacheMode: vfscommon.CacheModeFull,
		WriteBack: fs.Duration(5 * time.Second),
	})
	ctx := context.Background()

	// 1. Target file on the remote with an old modtime
	fh, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = fh.Write([]byte("old content"))
	require.NoError(t, err)
	require.NoError(t, fh.Close())
	vfs.WaitForWriters(waitForWritersDelay)
	oldTime := time.Now().Add(-time.Hour).Truncate(time.Second)
	obj, err := r.Fremote.NewObject(ctx, "file1")
	require.NoError(t, err)
	require.NoError(t, obj.SetModTime(ctx, oldTime))

	// 2. Write the new content into a temp file, set a fresh modtime
	// and close (queues the writeback in 5s)
	h, err := vfs.OpenFile("file1.tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0777)
	require.NoError(t, err)
	rw, ok := h.(*RWFileHandle)
	require.True(t, ok)
	_, err = rw.Write([]byte("new content"))
	require.NoError(t, err)
	newTime := time.Now().Truncate(time.Second)
	require.NoError(t, rw.file.SetModTime(newTime))
	require.NoError(t, rw.Close())

	// 3. Rename the temp file over the target (safe save)
	require.NoError(t, vfs.Rename("file1.tmp", "file1"))

	// 4. Reopen the renamed file without writing and close again
	h2, err := vfs.OpenFile("file1", os.O_RDWR, 0777)
	require.NoError(t, err)
	rw2, ok := h2.(*RWFileHandle)
	require.True(t, ok)
	require.NoError(t, rw2.Close())
	vfs.WaitForWriters(waitForWritersDelay)

	// 5. The upload which carried the renamed file to the remote must
	// have carried the new modtime as well - the remote must not be
	// left with the old modtime (issue #9637).
	obj, err = r.Fremote.NewObject(ctx, "file1")
	require.NoError(t, err)
	got := obj.ModTime(ctx)
	assert.False(t, got.Equal(oldTime), "remote modtime is %v instead of the new time %v", got, newTime)
	assert.WithinDuration(t, newTime, got, time.Second)
}
