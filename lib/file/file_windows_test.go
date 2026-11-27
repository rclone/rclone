//go:build windows

package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTestFiles creates a source file with newData and a destination file
// with oldData and returns their paths
func makeTestFiles(t *testing.T, oldData, newData string) (src, dst string) {
	t.Helper()
	dir := t.TempDir()
	src = filepath.Join(dir, "src.tmp")
	dst = filepath.Join(dir, "dst.txt")
	require.NoError(t, os.WriteFile(src, []byte(newData), 0600))
	require.NoError(t, os.WriteFile(dst, []byte(oldData), 0600))
	return src, dst
}

// TestRenameOverOpenDestination checks that Rename can replace a
// destination which has open handles, as long as they were opened with
// FILE_SHARE_DELETE which is how the VFS cache opens its files.
//
// This is the atomic save pattern (write temp, rename over target) which
// applications such as editors use. See issue #9943.
func TestRenameOverOpenDestination(t *testing.T) {
	src, dst := makeTestFiles(t, "old data", "new data")

	// Open the destination the way the VFS cache does
	fh, err := OpenFile(dst, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer func() {
		_ = fh.Close()
	}()

	require.NoError(t, Rename(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new data", string(got))

	// the source must be gone
	_, err = os.Stat(src)
	assert.True(t, os.IsNotExist(err))
}

// TestRenameOverOpenSource checks that Rename can move a source which has
// open handles, again as long as they were opened with FILE_SHARE_DELETE.
func TestRenameOverOpenSource(t *testing.T) {
	src, dst := makeTestFiles(t, "old data", "new data")

	fh, err := OpenFile(src, os.O_RDWR, 0600)
	require.NoError(t, err)
	defer func() {
		_ = fh.Close()
	}()

	require.NoError(t, Rename(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new data", string(got))
}

// TestRenameOpenDestinationNoShareDelete documents the limitation of the
// Windows POSIX semantics rename: a destination held open by a handle
// which does not share delete still cannot be replaced, and now reports
// an error instead of silently failing.
func TestRenameOpenDestinationNoShareDelete(t *testing.T) {
	src, dst := makeTestFiles(t, "old data", "new data")

	// os.Open does not open with FILE_SHARE_DELETE
	fh, err := os.Open(dst)
	require.NoError(t, err)
	defer func() {
		_ = fh.Close()
	}()

	err = Rename(src, dst)
	if err == nil {
		// Nothing can stop this handle being held on this system (eg if the
		// file is on a file system which does not implement the share modes)
		// so just check the rename did what it said on the tin.
		t.Logf("rename over a destination opened without FILE_SHARE_DELETE succeeded on this system")
		got, readErr := os.ReadFile(dst)
		require.NoError(t, readErr)
		assert.Equal(t, "new data", string(got))
		return
	}
	assert.Error(t, err)
	// the destination must be untouched
	got, readErr := os.ReadFile(dst)
	require.NoError(t, readErr)
	assert.Equal(t, "old data", string(got))
}

// TestRenameOverClosedFile checks the plain rename path still works
func TestRenameOverClosedFile(t *testing.T) {
	src, dst := makeTestFiles(t, "old data", "new data")

	require.NoError(t, Rename(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "new data", string(got))

	_, err = os.Stat(src)
	assert.True(t, os.IsNotExist(err))
}
