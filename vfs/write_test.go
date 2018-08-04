package vfs

import (
	"os"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Open a file for write
func writeHandleCreate(t *testing.T, r *fstest.Run) (*VFS, *WriteFileHandle) {
	vfs := New(r.Fremote, nil)

	h, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	fh, ok := h.(*WriteFileHandle)
	require.True(t, ok)

	return vfs, fh
}

func TestWriteFileHandleMethods(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := writeHandleCreate(t, r)

	// String
	assert.Equal(t, "file1 (w)", fh.String())
	assert.Equal(t, "<nil *WriteFileHandle>", (*WriteFileHandle)(nil).String())
	assert.Equal(t, "<nil *WriteFileHandle.file>", new(WriteFileHandle).String())

	// Node
	node := fh.Node()
	assert.Equal(t, "file1", node.Name())

	// Offset #1
	assert.Equal(t, int64(0), fh.Offset())
	assert.Equal(t, int64(0), node.Size())

	// Write (smoke test only since heavy lifting done in WriteAt)
	n, err := fh.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// Offset #2
	assert.Equal(t, int64(5), fh.Offset())
	assert.Equal(t, int64(5), node.Size())

	// Stat
	var fi os.FileInfo
	fi, err = fh.Stat()
	assert.NoError(t, err)
	assert.Equal(t, int64(5), fi.Size())
	assert.Equal(t, "file1", fi.Name())

	// Read
	var buf = make([]byte, 16)
	_, err = fh.Read(buf)
	assert.Equal(t, EPERM, err)

	// ReadAt
	_, err = fh.ReadAt(buf, 0)
	assert.Equal(t, EPERM, err)

	// Sync
	err = fh.Sync()
	assert.NoError(t, err)

	// Truncate - can only truncate where the file pointer is
	err = fh.Truncate(5)
	assert.NoError(t, err)
	err = fh.Truncate(6)
	assert.Equal(t, EPERM, err)

	// Close
	assert.NoError(t, fh.Close())

	// Check double close
	err = fh.Close()
	assert.Equal(t, ECLOSED, err)

	// check vfs
	root, err := vfs.Root()
	require.NoError(t, err)
	checkListing(t, root, []string{"file1,5,false"})

	// check the underlying r.Fremote but not the modtime
	file1 := fstest.NewItem("file1", "hello", t1)
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)

	// Check trying to open the file now it exists then closing it
	// immediately is OK
	h, err := vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,5,false"})

	// Check trying to open the file and writing it now it exists
	// returns an error
	h, err = vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE, 0777)
	require.NoError(t, err)
	_, err = h.Write([]byte("hello1"))
	require.Equal(t, EPERM, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,5,false"})

	// Check opening the file with O_TRUNC does actually truncate
	// it even if we don't write to it
	h, err = vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,0,false"})

	// Check opening the file with O_TRUNC and writing does work
	h, err = vfs.OpenFile("file1", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	require.NoError(t, err)
	_, err = h.WriteString("hello12")
	require.NoError(t, err)
	assert.NoError(t, h.Close())
	checkListing(t, root, []string{"file1,7,false"})
}

func TestWriteFileHandleWriteAt(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := writeHandleCreate(t, r)

	// Preconditions
	assert.Equal(t, int64(0), fh.offset)
	assert.False(t, fh.writeCalled)

	// Write the data
	n, err := fh.WriteAt([]byte("hello"), 0)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// After write
	assert.Equal(t, int64(5), fh.offset)
	assert.True(t, fh.writeCalled)

	// Check can't seek
	n, err = fh.WriteAt([]byte("hello"), 100)
	assert.Equal(t, ESPIPE, err)
	assert.Equal(t, 0, n)

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
	fstest.CheckListingWithPrecision(t, r.Fremote, []fstest.Item{file1}, []string{}, fs.ModTimeNotSupported)
}

func TestWriteFileHandleFlush(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := writeHandleCreate(t, r)

	// Check Flush already creates file for unwritten handles, without closing it
	err := fh.Flush()
	assert.NoError(t, err)
	assert.False(t, fh.closed)
	root, err := vfs.Root()
	assert.NoError(t, err)
	checkListing(t, root, []string{"file1,0,false"})

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

	// Check file was written properly
	root, err = vfs.Root()
	assert.NoError(t, err)
	checkListing(t, root, []string{"file1,5,false"})
}

func TestWriteFileHandleRelease(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	_, fh := writeHandleCreate(t, r)

	// Check Release closes file
	err := fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)

	// Check Release does nothing if called again
	err = fh.Release()
	assert.NoError(t, err)
	assert.True(t, fh.closed)
}

// tests mod time on open files
func TestWriteFileModTimeWithOpenWriters(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	vfs, fh := writeHandleCreate(t, r)

	mtime := time.Date(2012, time.November, 18, 17, 32, 31, 0, time.UTC)

	_, err := fh.Write([]byte{104, 105})
	require.NoError(t, err)

	err = fh.Node().SetModTime(mtime)
	require.NoError(t, err)

	err = fh.Close()
	require.NoError(t, err)

	info, err := vfs.Stat("file1")
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())
}
