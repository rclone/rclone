//go:build linux || darwin || freebsd || openbsd

package vfstest

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestMknod checks that Mknod creates a regular file through the mount.
//
// The VFS only supports regular files (S_IFREG) - this is the path the
// kernel NFS server drives when a client creates a file over an exported
// mount (device nodes are rejected). All three mount backends (mount,
// mount2, cmount) implement Mknod, so this runs against each via RunTests.
func TestMknod(t *testing.T) {
	run.skipIfVFS(t) // Mknod is a mount syscall; no Mknod on the direct-VFS pass
	run.skipIfNoFUSE(t)
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping test on OSX") // macFUSE Mknod support is unreliable
	}

	const name = "testmknod"
	path := run.path(name)

	// S_IFREG => regular file; dev 0 since it is not a device node.
	err := unix.Mknod(path, unix.S_IFREG|0640, 0)
	require.NoError(t, err)

	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Truef(t, fi.Mode().IsRegular(), "expected a regular file, got mode %v", fi.Mode())
	assert.Equal(t, int64(0), fi.Size())

	run.waitForWriters()
	run.rm(t, name)
}
