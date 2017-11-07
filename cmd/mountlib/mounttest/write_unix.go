// +build linux darwin freebsd

package mounttest

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWriteFileDoubleClose tests double close on write
func TestWriteFileDoubleClose(t *testing.T) {
	run.skipIfNoFUSE(t)

	out, err := osCreate(run.path("testdoubleclose"))
	assert.NoError(t, err)
	fd := out.Fd()

	fd1, err := syscall.Dup(int(fd))
	assert.NoError(t, err)

	fd2, err := syscall.Dup(int(fd))
	assert.NoError(t, err)

	// close one of the dups - should produce no error
	err = syscall.Close(fd1)
	assert.NoError(t, err)

	// write to the file
	buf := []byte("hello")
	n, err := out.Write(buf)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)

	// close it
	err = out.Close()
	assert.NoError(t, err)

	// write to the other dup - should produce an error
	_, err = syscall.Write(fd2, buf)
	assert.Error(t, err, "input/output error")

	// close the dup - should not produce an error
	err = syscall.Close(fd2)
	assert.NoError(t, err)

	run.rm(t, "testdoubleclose")
}
