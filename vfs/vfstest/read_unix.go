//go:build linux || darwin || freebsd

package vfstest

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReadFileDoubleClose tests double close on read
func TestReadFileDoubleClose(t *testing.T) {
	run.skipIfVFS(t)
	run.skipIfNoFUSE(t)

	run.createFile(t, "testdoubleclose", "hello")

	in, err := run.os.Open(run.path("testdoubleclose"))
	assert.NoError(t, err)
	fd := in.Fd()

	fd1, err := syscall.Dup(int(fd))
	assert.NoError(t, err)

	fd2, err := syscall.Dup(int(fd))
	assert.NoError(t, err)

	// close one of the dups - should produce no error
	err = syscall.Close(fd1)
	assert.NoError(t, err)

	// read from the file
	buf := make([]byte, 1)
	_, err = in.Read(buf)
	assert.NoError(t, err)

	// close it
	err = in.Close()
	assert.NoError(t, err)

	// read from the other dup - should produce no error as this
	// file is now buffered
	n, err := syscall.Read(fd2, buf)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)

	// close the dup - should not produce an error
	err = syscall.Close(fd2)
	assert.NoError(t, err, "input/output error")

	run.rm(t, "testdoubleclose")
}
