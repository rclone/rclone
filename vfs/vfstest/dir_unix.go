//go:build linux || darwin || freebsd

package vfstest

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countDirFd reads the directory referred to by fd to end-of-stream using
// raw getdents syscalls and returns the number of entries (excluding "." and
// ".." which ParseDirent skips).
//
// This deliberately uses syscall.ReadDirent rather than (*os.File).Readdir so
// that it exercises the kernel readdir path directly on a single file
// descriptor, the same way an in-process rewinddir does.
func countDirFd(t *testing.T, fd int) int {
	buf := make([]byte, 8192)
	var names []string
	for {
		n, err := syscall.ReadDirent(fd, buf)
		require.NoError(t, err)
		if n <= 0 {
			break
		}
		_, _, names = syscall.ParseDirent(buf[:n], -1, names)
	}
	return len(names)
}

// TestDirRewind checks that re-reading a directory after rewinding the
// directory stream (lseek(fd, 0, SEEK_SET), as rewinddir(3) does) returns the
// same entries as the first read.
//
// This reproduces the bug where the mount2 backend returned an empty listing
// on every read after the first because go-fuse v2.9.0 rewinds a directory by
// calling Seekdir, which dirStream did not implement. See PR #9469 and
// https://github.com/hanwen/go-fuse/issues/549
func TestDirRewind(t *testing.T) {
	run.skipIfVFS(t)
	run.skipIfNoFUSE(t)

	run.mkdir(t, "dir")
	run.createFile(t, "dir/f1", "1")
	run.createFile(t, "dir/f2", "2")
	run.createFile(t, "dir/f3", "3")
	run.checkDir(t, "dir/|dir/f1 1|dir/f2 1|dir/f3 1")

	fh, err := run.os.Open(run.path("dir"))
	require.NoError(t, err)
	defer func() {
		_ = fh.Close()
	}()
	fd := int(fh.Fd())

	first := countDirFd(t, fd)
	assert.Equal(t, 3, first, "first read should see all entries")

	// rewinddir == lseek(fd, 0, SEEK_SET)
	_, err = syscall.Seek(fd, 0, 0)
	require.NoError(t, err)

	second := countDirFd(t, fd)
	assert.Equal(t, first, second, "re-read after rewind should match first read")

	require.NoError(t, fh.Close())
	run.rm(t, "dir/f1")
	run.rm(t, "dir/f2")
	run.rm(t, "dir/f3")
	run.rmdir(t, "dir")
	run.checkDir(t, "")
}
