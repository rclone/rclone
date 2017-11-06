package vfs

import (
	"io"
	"os"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Open a file for write
func rwHandleCreateReadOnly(t *testing.T, r *fstest.Run) (*VFS, *RWFileHandle) {
	vfs := New(r.Fremote, nil)
	vfs.Opt.CacheMode = CacheModeFull

	file1 := r.WriteObject("dir/file1", "0123456789abcdef", t1)
	fstest.CheckItems(t, r.Fremote, file1)

	h, err := vfs.OpenFile("dir/file1", os.O_RDONLY, 0777)
	require.NoError(t, err)
	fh, ok := h.(*RWFileHandle)
	require.True(t, ok)

	return vfs, fh
}

// Open a file for write
func rwHandleCreateWriteOnly(t *testing.T, r *fstest.Run) (*VFS, *RWFileHandle) {
	vfs := New(r.Fremote, nil)
	vfs.Opt.CacheMode = CacheModeFull

	h, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	fh, ok := h.(*RWFileHandle)
	require.True(t, ok)

	return vfs, fh
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
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateReadOnly(t, r)

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
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateReadOnly(t, r)

	assert.Equal(t, "0", rwReadString(t, fh, 1))

	// 0 means relative to the origin of the file,
	n, err := fh.Seek(5, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
	assert.Equal(t, "5", rwReadString(t, fh, 1))

	// 1 means relative to the current offset
	n, err = fh.Seek(-3, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.Equal(t, "3", rwReadString(t, fh, 1))

	// 2 means relative to the end.
	n, err = fh.Seek(-3, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), n)
	assert.Equal(t, "d", rwReadString(t, fh, 1))

	// Seek off the end
	n, err = fh.Seek(100, 0)
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
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateReadOnly(t, r)

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
	n, err = fh.ReadAt(buf, 100)
	assert.Equal(t, ECLOSED, err)
}

func TestRWFileHandleFlushRead(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateReadOnly(t, r)

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
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateReadOnly(t, r)

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
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := rwHandleCreateWriteOnly(t, r)

	// String
	assert.Equal(t, "file1 (rw)", fh.String())
	assert.Equal(t, "<nil *RWFileHandle>", (*RWFileHandle)(nil).String())
	assert.Equal(t, "<nil *RWFileHandle.file>", new(RWFileHandle).String())

	// Node
	node := fh.Node()
	assert.Equal(t, "file1", node.Name())

	offset := func() int64 {
		n, err := fh.Seek(0, 1)
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
	checkListing(t, root, []string{"file1,11,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello world", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)
}

func TestRWFileHandleWriteAt(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := rwHandleCreateWriteOnly(t, r)

	offset := func() int64 {
		n, err := fh.Seek(0, 1)
		require.NoError(t, err)
		return n
	}

	// Preconditions
	assert.Equal(t, int64(0), offset())
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
	checkListing(t, root, []string{"file1,11,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello world", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)
}

func TestRWFileHandleWriteNoWrite(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := rwHandleCreateWriteOnly(t, r)

	// Close the file without writing to it
	err := fh.Close()
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
	checkListing(t, root, []string{"file1,0,false", "file2,0,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "", t1)
	file2 := fstest.NewItem("file2", "", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1, file2}, []string{}, fs.ModTimeNotSupported)
}

func TestRWFileHandleFlushWrite(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateWriteOnly(t, r)

	// Check Flush does nothing if write not called
	err := fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Write some data
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Check Flush closes file if write called
	err = fh.Flush()
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check flush does nothing if called again
	err = fh.Flush()
	assert.NoError(t, err)
	assert.True(t, fh.closed)
}

func TestRWFileHandleReleaseWrite(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := rwHandleCreateWriteOnly(t, r)

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
