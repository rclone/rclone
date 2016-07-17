// +build linux darwin freebsd

package mount

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test writing a file with no write()'s to it
func TestWriteFileNoWrite(t *testing.T) {
	fd, err := os.Create(run.path("testnowrite"))
	assert.NoError(t, err)

	err = fd.Close()
	assert.NoError(t, err)

	run.checkDir(t, "testnowrite 0")

	run.rm(t, "testnowrite")
}

// Test open file in directory listing
func FIXMETestWriteOpenFileInDirListing(t *testing.T) {
	fd, err := os.Create(run.path("testnowrite"))
	assert.NoError(t, err)

	run.checkDir(t, "testnowrite 0")

	err = fd.Close()
	assert.NoError(t, err)

	run.rm(t, "testnowrite")
}

// Test writing a file and reading it back
func TestWriteFileWrite(t *testing.T) {
	run.createFile(t, "testwrite", "data")
	run.checkDir(t, "testwrite 4")
	contents := run.readFile(t, "testwrite")
	assert.Equal(t, "data", contents)
	run.rm(t, "testwrite")
}

// Test overwriting a file
func TestWriteFileOverwrite(t *testing.T) {
	run.createFile(t, "testwrite", "data")
	run.checkDir(t, "testwrite 4")
	run.createFile(t, "testwrite", "potato")
	contents := run.readFile(t, "testwrite")
	assert.Equal(t, "potato", contents)
	run.rm(t, "testwrite")
}

// Test double close
func TestWriteFileDoubleClose(t *testing.T) {
	out, err := os.Create(run.path("testdoubleclose"))
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
	n, err = syscall.Write(fd2, buf)
	assert.Error(t, err, "input/output error")

	// close the dup - should produce an error
	err = syscall.Close(fd2)
	assert.Error(t, err, "input/output error")

	run.rm(t, "testdoubleclose")
}
