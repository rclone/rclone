//go:build unix

package nfs

import (
	"io"
	"os"
	"strings"
	"testing"

	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
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

// go-nfs joins the file name from the client onto the directory path
// with Join without checking it for ".." elements or a "/", so such a
// name must not reach outside the directory served.
//
// The memory backend is used because it resolves ".." by joining paths,
// unlike the local backend which refuses paths outside its root.
func TestPathTraversal(t *testing.T) {
	ctx := t.Context()
	modTime := fstest.Time("2001-02-03T04:05:06.499999999Z")

	// outside is the parent of the directory served and holds an object
	// which must stay unreachable through the server.
	outside, err := fs.NewFs(ctx, ":memory:nfs-traversal-test")
	require.NoError(t, err)
	_, err = operations.Rcat(ctx, outside, "outside-secret.txt", io.NopCloser(strings.NewReader("SECRET")), modTime, nil)
	require.NoError(t, err)

	f, err := fs.NewFs(ctx, ":memory:nfs-traversal-test/served-root")
	require.NoError(t, err)
	_, err = operations.Rcat(ctx, f, "inside.txt", io.NopCloser(strings.NewReader("INSIDE")), modTime, nil)
	require.NoError(t, err)

	opt := vfscommon.Opt
	opt.Links = true
	opt.CacheMode = vfscommon.CacheModeWrites
	v := vfs.New(ctx, f, &opt)
	defer v.Shutdown()
	bfs := &FS{vfs: v}

	// The MkdirAll calls come first so that, should one succeed, the
	// later calls are routed through the directory node it made.
	assert.Error(t, bfs.MkdirAll(bfs.Join("", "../new-dir"), 0777))
	assert.Error(t, bfs.MkdirAll(bfs.Join("", ".."), 0777))

	_, err = bfs.Create(bfs.Join("", "../outside-write.txt"))
	assert.Error(t, err)
	_, err = bfs.OpenFile(bfs.Join("", "../outside-secret.txt"), os.O_WRONLY|os.O_TRUNC, 0666)
	assert.Error(t, err)
	_, err = bfs.Open(bfs.Join("", "../outside-secret.txt"))
	assert.Error(t, err)
	assert.Error(t, bfs.Symlink("target", bfs.Join("", "../outside-link")))
	assert.Error(t, bfs.Rename("inside.txt", bfs.Join("", "../outside-move.txt")))
	assert.Error(t, bfs.Rename(bfs.Join("", "../outside-secret.txt"), "stolen.txt"))
	assert.Error(t, bfs.Remove(bfs.Join("", "../outside-secret.txt")))

	// Nothing was created, modified, moved or removed, either outside
	// the served root or inside it.
	fstest.CheckListingWithPrecision(t, outside, []fstest.Item{
		fstest.NewItem("outside-secret.txt", "SECRET", modTime),
		fstest.NewItem("served-root/inside.txt", "INSIDE", modTime),
	}, []string{"served-root"}, fs.GetModifyWindow(ctx, outside))
}
