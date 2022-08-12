//go:build linux || darwin || freebsd
// +build linux darwin freebsd

package vfstest

import (
	"runtime"
	"testing"

	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

// TestWriteFileDoubleClose tests double close on write
func TestWriteFileDoubleClose(t *testing.T) {
	run.skipIfVFS(t)
	run.skipIfNoFUSE(t)
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping test on OSX")
	}

	out, err := osCreate(run.path("testdoubleclose"))
	assert.NoError(t, err)
	fd := out.Fd()

	fd1, err := unix.Dup(int(fd))
	assert.NoError(t, err)

	fd2, err := unix.Dup(int(fd))
	assert.NoError(t, err)

	// close one of the dups - should produce no error
	err = unix.Close(fd1)
	assert.NoError(t, err)

	// write to the file
	buf := []byte("hello")
	n, err := out.Write(buf)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// close it
	err = out.Close()
	assert.NoError(t, err)

	// write to the other dup
	_, err = unix.Write(fd2, buf)
	if run.vfsOpt.CacheMode < vfscommon.CacheModeWrites {
		// produces an error if cache mode < writes
		assert.Error(t, err, "input/output error")
	} else {
		// otherwise does not produce an error
		assert.NoError(t, err)
	}

	// close the dup - should not produce an error
	err = unix.Close(fd2)
	assert.NoError(t, err)

	run.waitForWriters()
	run.rm(t, "testdoubleclose")
}

// writeTestDup performs the platform-specific implementation of the dup() unix
func writeTestDup(oldfd uintptr) (uintptr, error) {
	newfd, err := unix.Dup(int(oldfd))
	return uintptr(newfd), err
}
