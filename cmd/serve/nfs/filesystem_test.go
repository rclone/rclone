//go:build unix

package nfs

import (
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Chmod/Chown arrive as plain SETATTR calls on the link path after a
// SYMLINK RPC, so they must not follow symlinks - a freshly created
// symlink usually dangles and following it would fail with ENOENT,
// which the NFS layer surfaces as an IO error. See #9627.
func TestChmodDanglingSymlink(t *testing.T) {
	ctx := t.Context()
	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)
	opt := vfscommon.Opt
	opt.Links = true
	opt.CacheMode = vfscommon.CacheModeWrites
	v := vfs.New(ctx, f, &opt)
	defer v.Shutdown()
	bfs := &FS{vfs: v}

	// Create a symlink pointing at a target which doesn't exist yet
	require.NoError(t, bfs.Symlink("does-not-exist", "link"))

	// SETATTR after SYMLINK must not fail
	assert.NoError(t, bfs.Chmod("link", 0777))
	assert.NoError(t, bfs.Chown("link", 1000, 1000))
	assert.NoError(t, bfs.Lchown("link", 1000, 1000))

	// A genuinely missing node must still report ENOENT
	assert.ErrorIs(t, bfs.Chmod("missing", 0777), vfs.ENOENT)
	assert.ErrorIs(t, bfs.Chown("missing", 1000, 1000), vfs.ENOENT)
}
