package mounttest

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileModTime tests mod times on files
func TestFileModTime(t *testing.T) {
	run.skipIfNoFUSE(t)

	run.createFile(t, "file", "123")

	mtime := time.Date(2012, 11, 18, 17, 32, 31, 0, time.UTC)
	err := os.Chtimes(run.path("file"), mtime, mtime)
	require.NoError(t, err)

	info, err := os.Stat(run.path("file"))
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())

	run.rm(t, "file")
}

// TestFileModTimeWithOpenWriters tests mod time on open files
func TestFileModTimeWithOpenWriters(t *testing.T) {
	run.skipIfNoFUSE(t)

	mtime := time.Date(2012, 11, 18, 17, 32, 31, 0, time.UTC)
	filepath := run.path("cp-archive-test")

	f, err := os.Create(filepath)
	require.NoError(t, err)

	_, err = f.Write([]byte{104, 105})
	require.NoError(t, err)

	err = os.Chtimes(filepath, mtime, mtime)
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	info, err := os.Stat(filepath)
	require.NoError(t, err)

	// avoid errors because of timezone differences
	assert.Equal(t, info.ModTime().Unix(), mtime.Unix())

	run.rm(t, "cp-archive-test")
}
