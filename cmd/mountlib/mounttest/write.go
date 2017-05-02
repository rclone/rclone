// +build linux darwin freebsd

package mounttest

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test writing a file with no write()'s to it
func TestWriteFileNoWrite(t *testing.T) {
	run.skipIfNoFUSE(t)

	fd, err := os.Create(run.path("testnowrite"))
	assert.NoError(t, err)

	err = fd.Close()
	assert.NoError(t, err)

	// FIXME - wait for the Release on the file
	time.Sleep(10 * time.Millisecond)

	run.checkDir(t, "testnowrite 0")

	run.rm(t, "testnowrite")
}

// Test open file in directory listing
func FIXMETestWriteOpenFileInDirListing(t *testing.T) {
	run.skipIfNoFUSE(t)

	fd, err := os.Create(run.path("testnowrite"))
	assert.NoError(t, err)

	run.checkDir(t, "testnowrite 0")

	err = fd.Close()
	assert.NoError(t, err)

	run.rm(t, "testnowrite")
}

// Test writing a file and reading it back
func TestWriteFileWrite(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.createFile(t, "testwrite", "data")
	run.checkDir(t, "testwrite 4")
	contents := run.readFile(t, "testwrite")
	assert.Equal(t, "data", contents)
	run.rm(t, "testwrite")
}

// Test overwriting a file
func TestWriteFileOverwrite(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.createFile(t, "testwrite", "data")
	run.checkDir(t, "testwrite 4")
	run.createFile(t, "testwrite", "potato")
	contents := run.readFile(t, "testwrite")
	assert.Equal(t, "potato", contents)
	run.rm(t, "testwrite")
}

// Test double close
func TestWriteFileDoubleClose(t *testing.T) {
	run.skipIfNoFUSE(t)

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

// Test Fsync
//
// NB the code for this is in file.go rather than write.go
func TestWriteFileFsync(t *testing.T) {
	filepath := run.path("to be synced")
	fd, err := os.Create(filepath)
	require.NoError(t, err)
	_, err = fd.Write([]byte("hello"))
	require.NoError(t, err)
	err = fd.Sync()
	require.NoError(t, err)
	err = fd.Close()
	require.NoError(t, err)
}
