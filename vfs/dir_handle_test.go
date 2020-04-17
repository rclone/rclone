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

func TestDirHandleMethods(t *testing.T) {
	_, _, dir, _, cleanup := dirCreate(t)
	defer cleanup()

	h, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)
	fh, ok := h.(*DirHandle)
	assert.True(t, ok)

	// String
	assert.Equal(t, "dir/ (r)", fh.String())
	assert.Equal(t, "<nil *DirHandle>", (*DirHandle)(nil).String())
	assert.Equal(t, "<nil *DirHandle.d>", newDirHandle(nil).String())

	// Stat
	fi, err := fh.Stat()
	require.NoError(t, err)
	assert.Equal(t, dir, fi)

	// Node
	assert.Equal(t, dir, fh.Node())

	// Close
	require.NoError(t, h.Close())
	assert.Equal(t, []os.FileInfo(nil), fh.fis)
}

func TestDirHandleReaddir(t *testing.T) {
	r, vfs, cleanup := newTestVFS(t)
	defer cleanup()

	file1 := r.WriteObject(context.Background(), "dir/file1", "file1 contents", t1)
	file2 := r.WriteObject(context.Background(), "dir/file2", "file2- contents", t2)
	file3 := r.WriteObject(context.Background(), "dir/subdir/file3", "file3-- contents", t3)
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	node, err := vfs.Stat("dir")
	require.NoError(t, err)
	dir := node.(*Dir)

	// Read in one chunk
	fh, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)

	fis, err := fh.Readdir(-1)
	require.NoError(t, err)
	require.Equal(t, 3, len(fis))
	assert.Equal(t, "file1", fis[0].Name())
	assert.Equal(t, "file2", fis[1].Name())
	assert.Equal(t, "subdir", fis[2].Name())
	assert.False(t, fis[0].IsDir())
	assert.False(t, fis[1].IsDir())
	assert.True(t, fis[2].IsDir())

	require.NoError(t, fh.Close())

	// Read in multiple chunks
	fh, err = dir.Open(os.O_RDONLY)
	require.NoError(t, err)

	fis, err = fh.Readdir(2)
	require.NoError(t, err)
	require.Equal(t, 2, len(fis))
	assert.Equal(t, "file1", fis[0].Name())
	assert.Equal(t, "file2", fis[1].Name())
	assert.False(t, fis[0].IsDir())
	assert.False(t, fis[1].IsDir())

	fis, err = fh.Readdir(2)
	require.NoError(t, err)
	require.Equal(t, 1, len(fis))
	assert.Equal(t, "subdir", fis[0].Name())
	assert.True(t, fis[0].IsDir())

	fis, err = fh.Readdir(2)
	assert.Equal(t, io.EOF, err)
	require.Equal(t, 0, len(fis))

	require.NoError(t, fh.Close())

}

func TestDirHandleReaddirnames(t *testing.T) {
	_, _, dir, _, cleanup := dirCreate(t)
	defer cleanup()

	fh, err := dir.Open(os.O_RDONLY)
	require.NoError(t, err)

	// Smoke test only since heavy lifting done in Readdir
	fis, err := fh.Readdirnames(-1)
	require.NoError(t, err)
	require.Equal(t, 1, len(fis))
	assert.Equal(t, "file1", fis[0])

	require.NoError(t, fh.Close())
}
