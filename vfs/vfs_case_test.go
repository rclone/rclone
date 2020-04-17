package vfs

import (
	"context"
	"os"
	"testing"

	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaseSensitivity(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	if r.Fremote.Features().CaseInsensitive {
		t.Skip("Can't test case sensitivity - this remote is officially not case-sensitive")
	}

	// Create test files
	ctx := context.Background()
	file1 := r.WriteObject(ctx, "FiLeA", "data1", t1)
	file2 := r.WriteObject(ctx, "FiLeB", "data2", t2)
	fstest.CheckItems(t, r.Fremote, file1, file2)

	// Create file3 with name differing from file2 name only by case.
	// On a case-Sensitive remote this will be a separate file.
	// On a case-INsensitive remote this file will either not exist
	// or overwrite file2 depending on how file system diverges.
	// On a box.com remote this step will even fail.
	file3 := r.WriteObject(ctx, "FilEb", "data3", t3)

	// Create a case-Sensitive and case-INsensitive VFS
	optCS := vfscommon.DefaultOpt
	optCS.CaseInsensitive = false
	vfsCS := New(r.Fremote, &optCS)
	defer cleanupVFS(t, vfsCS)

	optCI := vfscommon.DefaultOpt
	optCI.CaseInsensitive = true
	vfsCI := New(r.Fremote, &optCI)
	defer cleanupVFS(t, vfsCI)

	// Run basic checks that must pass on VFS of any type.
	assertFileDataVFS(t, vfsCI, "FiLeA", "data1")
	assertFileDataVFS(t, vfsCS, "FiLeA", "data1")

	// Detect case sensitivity of the underlying remote.
	remoteIsOK := true
	if !checkFileDataVFS(t, vfsCS, "FiLeA", "data1") {
		remoteIsOK = false
	}
	if !checkFileDataVFS(t, vfsCS, "FiLeB", "data2") {
		remoteIsOK = false
	}
	if !checkFileDataVFS(t, vfsCS, "FilEb", "data3") {
		remoteIsOK = false
	}

	// The remaining test is only meaningful on a case-Sensitive file system.
	if !remoteIsOK {
		t.Skip("Can't test case sensitivity - this remote doesn't comply as case-sensitive")
	}

	// Continue with test as the underlying remote is fully case-Sensitive.
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	// See how VFS handles case-INsensitive flag
	assertFileDataVFS(t, vfsCI, "FiLeA", "data1")
	assertFileDataVFS(t, vfsCI, "fileA", "data1")
	assertFileDataVFS(t, vfsCI, "filea", "data1")
	assertFileDataVFS(t, vfsCI, "FILEA", "data1")

	assertFileDataVFS(t, vfsCI, "FiLeB", "data2")
	assertFileDataVFS(t, vfsCI, "FilEb", "data3")

	fd, err := vfsCI.OpenFile("fileb", os.O_RDONLY, 0777)
	assert.Nil(t, fd)
	assert.Error(t, err)
	assert.NotEqual(t, err, ENOENT)

	fd, err = vfsCI.OpenFile("FILEB", os.O_RDONLY, 0777)
	assert.Nil(t, fd)
	assert.Error(t, err)
	assert.NotEqual(t, err, ENOENT)

	// Run the same set of checks with case-Sensitive VFS, for comparison.
	assertFileDataVFS(t, vfsCS, "FiLeA", "data1")

	assertFileAbsentVFS(t, vfsCS, "fileA")
	assertFileAbsentVFS(t, vfsCS, "filea")
	assertFileAbsentVFS(t, vfsCS, "FILEA")

	assertFileDataVFS(t, vfsCS, "FiLeB", "data2")
	assertFileDataVFS(t, vfsCS, "FilEb", "data3")

	assertFileAbsentVFS(t, vfsCS, "fileb")
	assertFileAbsentVFS(t, vfsCS, "FILEB")
}

func checkFileDataVFS(t *testing.T, vfs *VFS, name string, expect string) bool {
	fd, err := vfs.OpenFile(name, os.O_RDONLY, 0777)
	if fd == nil || err != nil {
		return false
	}
	defer func() {
		// File must be closed - otherwise Run.cleanUp() will fail on Windows.
		_ = fd.Close()
	}()

	fh, ok := fd.(*ReadFileHandle)
	if !ok {
		return false
	}

	size := len(expect)
	buf := make([]byte, size)
	num, err := fh.Read(buf)
	if err != nil || num != size {
		return false
	}

	return string(buf) == expect
}

func assertFileDataVFS(t *testing.T, vfs *VFS, name string, expect string) {
	fd, errOpen := vfs.OpenFile(name, os.O_RDONLY, 0777)
	assert.NotNil(t, fd)
	assert.NoError(t, errOpen)

	defer func() {
		// File must be closed - otherwise Run.cleanUp() will fail on Windows.
		if errOpen == nil && fd != nil {
			_ = fd.Close()
		}
	}()

	fh, ok := fd.(*ReadFileHandle)
	require.True(t, ok)

	size := len(expect)
	buf := make([]byte, size)
	numRead, errRead := fh.Read(buf)
	assert.NoError(t, errRead)
	assert.Equal(t, numRead, size)

	assert.Equal(t, string(buf), expect)
}

func assertFileAbsentVFS(t *testing.T, vfs *VFS, name string) {
	fd, err := vfs.OpenFile(name, os.O_RDONLY, 0777)
	defer func() {
		// File must be closed - otherwise Run.cleanUp() will fail on Windows.
		if err == nil && fd != nil {
			_ = fd.Close()
		}
	}()
	assert.Nil(t, fd)
	assert.Error(t, err)
	assert.Equal(t, err, ENOENT)
}
