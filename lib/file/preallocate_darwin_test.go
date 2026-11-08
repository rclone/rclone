//go:build darwin

package file

import (
	"bytes"
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blocks returns the number of 512 byte blocks allocated for the file
func blocks(t *testing.T, f *os.File) int64 {
	t.Helper()
	require.NoError(t, f.Sync())
	info, err := f.Stat()
	require.NoError(t, err)
	stat, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok, "not a syscall.Stat_t")
	return stat.Blocks
}

// Test PreAllocate creates a sparse file whose size is the final size
// immediately, and whose allocated blocks grow as data is written.
func TestPreAllocate(t *testing.T) {
	dir := t.TempDir()
	filepath := path.Join(dir, "file1")
	f, err := os.Create(filepath)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, f.Close())
	}()

	const size = 1 << 20
	require.NoError(t, PreAllocate(size, f))

	// size is the final size immediately, with no blocks allocated yet
	info, err := f.Stat()
	require.NoError(t, err)
	assert.Equal(t, int64(size), info.Size())
	blocksAfterPreAllocate := blocks(t, f)

	// write half the file - blocks should grow
	_, err = f.Write(bytes.Repeat([]byte{0xAB}, size/2))
	require.NoError(t, err)
	blocksAfterHalf := blocks(t, f)
	assert.Greater(t, blocksAfterHalf, blocksAfterPreAllocate)

	// write the rest - blocks should grow again
	_, err = f.Write(bytes.Repeat([]byte{0xAB}, size/2))
	require.NoError(t, err)
	blocksAfterFull := blocks(t, f)
	assert.Greater(t, blocksAfterFull, blocksAfterHalf)

	// size is still the final size
	info, err = f.Stat()
	require.NoError(t, err)
	assert.Equal(t, int64(size), info.Size())
}

// Test PreAllocate with an unknown size does nothing
func TestPreAllocateUnknownSize(t *testing.T) {
	dir := t.TempDir()
	filepath := path.Join(dir, "file1")
	f, err := os.Create(filepath)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, f.Close())
	}()

	assert.NoError(t, PreAllocate(0, f))
	assert.NoError(t, PreAllocate(-1, f))

	info, err := f.Stat()
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}
