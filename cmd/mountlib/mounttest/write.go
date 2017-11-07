package mounttest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteFileNoWrite tests writing a file with no write()'s to it
func TestWriteFileNoWrite(t *testing.T) {
	run.skipIfNoFUSE(t)

	fd, err := osCreate(run.path("testnowrite"))
	assert.NoError(t, err)

	err = fd.Close()
	assert.NoError(t, err)

	// FIXME - wait for the Release on the file
	time.Sleep(10 * time.Millisecond)

	run.checkDir(t, "testnowrite 0")

	run.rm(t, "testnowrite")
}

// FIXMETestWriteOpenFileInDirListing tests open file in directory listing
func FIXMETestWriteOpenFileInDirListing(t *testing.T) {
	run.skipIfNoFUSE(t)

	fd, err := osCreate(run.path("testnowrite"))
	assert.NoError(t, err)

	run.checkDir(t, "testnowrite 0")

	err = fd.Close()
	assert.NoError(t, err)

	run.rm(t, "testnowrite")
}

// TestWriteFileWrite tests writing a file and reading it back
func TestWriteFileWrite(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.createFile(t, "testwrite", "data")
	run.checkDir(t, "testwrite 4")
	contents := run.readFile(t, "testwrite")
	assert.Equal(t, "data", contents)
	run.rm(t, "testwrite")
}

// TestWriteFileOverwrite tests overwriting a file
func TestWriteFileOverwrite(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.createFile(t, "testwrite", "data")
	run.checkDir(t, "testwrite 4")
	run.createFile(t, "testwrite", "potato")
	contents := run.readFile(t, "testwrite")
	assert.Equal(t, "potato", contents)
	run.rm(t, "testwrite")
}

// TestWriteFileFsync tests Fsync
//
// NB the code for this is in file.go rather than write.go
func TestWriteFileFsync(t *testing.T) {
	filepath := run.path("to be synced")
	fd, err := osCreate(filepath)
	require.NoError(t, err)
	_, err = fd.Write([]byte("hello"))
	require.NoError(t, err)
	err = fd.Sync()
	require.NoError(t, err)
	err = fd.Close()
	require.NoError(t, err)
}
