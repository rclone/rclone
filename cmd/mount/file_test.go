// +build linux darwin freebsd

package mount

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
