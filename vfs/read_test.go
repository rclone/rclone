package vfs

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Open a file for write
func readHandleCreate(t *testing.T) (r *fstest.Run, vfs *VFS, fh *ReadFileHandle) {
	r, vfs = newTestVFS(t)

	file1 := r.WriteObject(context.Background(), "dir/file1", "0123456789abcdef", t1)
	r.CheckRemoteItems(t, file1)

	h, err := vfs.OpenFile("dir/file1", os.O_RDONLY, 0777)
	require.NoError(t, err)
	fh, ok := h.(*ReadFileHandle)
	require.True(t, ok)

	return r, vfs, fh
}

// read data from the string
func readString(t *testing.T, fh *ReadFileHandle, n int) string {
	buf := make([]byte, n)
	n, err := fh.Read(buf)
	if err != io.EOF {
		assert.NoError(t, err)
	}
	return string(buf[:n])
}

func TestReadFileHandleMethods(t *testing.T) {
	_, _, fh := readHandleCreate(t)

	// String
	assert.Equal(t, "dir/file1 (r)", fh.String())
	assert.Equal(t, "<nil *ReadFileHandle>", (*ReadFileHandle)(nil).String())
	assert.Equal(t, "<nil *ReadFileHandle.file>", new(ReadFileHandle).String())

	// Name
	assert.Equal(t, "dir/file1", fh.Name())

	// Node
	node := fh.Node()
	assert.Equal(t, "file1", node.Name())

	// Size
	assert.Equal(t, int64(16), fh.Size())

	// Read 1
	assert.Equal(t, "0", readString(t, fh, 1))

	// Read remainder
	assert.Equal(t, "123456789abcdef", readString(t, fh, 256))

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

func TestReadFileHandleSeek(t *testing.T) {
	_, _, fh := readHandleCreate(t)

	assert.Equal(t, "0", readString(t, fh, 1))

	// 0 means relative to the origin of the file,
	n, err := fh.Seek(5, io.SeekStart)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), n)
	assert.Equal(t, "5", readString(t, fh, 1))

	// 1 means relative to the current offset
	n, err = fh.Seek(-3, io.SeekCurrent)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
	assert.Equal(t, "3", readString(t, fh, 1))

	// 2 means relative to the end.
	n, err = fh.Seek(-3, io.SeekEnd)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), n)
	assert.Equal(t, "d", readString(t, fh, 1))

	// Seek off the end
	_, err = fh.Seek(100, io.SeekStart)
	assert.NoError(t, err)

	// Get the error on read
	buf := make([]byte, 16)
	l, err := fh.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, l)

	// Check if noSeek is set we get an error
	fh.noSeek = true
	_, err = fh.Seek(0, io.SeekStart)
	assert.Equal(t, ESPIPE, err)

	// Close
	assert.Equal(t, nil, fh.Close())
}

func TestReadFileHandleReadAt(t *testing.T) {
	_, _, fh := readHandleCreate(t)

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

	// check noSeek gives an error
	fh.noSeek = true
	_, err = fh.ReadAt(buf, 100)
	assert.Equal(t, ESPIPE, err)

	// Properly close the file
	assert.NoError(t, fh.Close())

	// check reading on closed file
	fh.noSeek = true
	_, err = fh.ReadAt(buf, 100)
	assert.Equal(t, ECLOSED, err)
}

func TestReadFileHandleFlush(t *testing.T) {
	_, _, fh := readHandleCreate(t)

	// Check Flush does nothing if read not called
	err := fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Read data
	buf := make([]byte, 256)
	n, err := fh.Read(buf)
	assert.Equal(t, io.EOF, err)
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

func TestReadFileHandleRelease(t *testing.T) {
	_, _, fh := readHandleCreate(t)

	// Check Release does nothing if file not read from
	err := fh.Release()
	assert.NoError(t, err)
	assert.False(t, fh.closed)

	// Read data
	buf := make([]byte, 256)
	n, err := fh.Read(buf)
	assert.Equal(t, io.EOF, err)
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
