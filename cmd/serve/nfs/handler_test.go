//go:build unix

package nfs

import (
	"bytes"
	"context"
	"io"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	nfs "github.com/willscott/go-nfs"
)

// newTestHandler builds a Handler backed by a writable local-filesystem VFS
// rooted in a per-test temp directory with the layout the Mount tests need.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	ctx := context.Background()
	f, err := fs.NewFs(ctx, t.TempDir())
	require.NoError(t, err)

	vfsOpt := vfscommon.Opt
	vfsOpt.CacheMode = vfscommon.CacheModeFull
	V := vfs.New(ctx, f, &vfsOpt)
	t.Cleanup(V.Shutdown)

	// Layout used across the tests:
	//   /sub/                (directory — valid subpath mount target)
	//   /sub/hello.txt       (file — not a valid subpath mount target)
	//   /sub/nested/         (directory inside the subpath)
	require.NoError(t, V.Mkdir("/sub", 0755))
	require.NoError(t, V.Mkdir("/sub/nested", 0755))
	hello, err := V.Create("/sub/hello.txt")
	require.NoError(t, err)
	_, err = io.Copy(hello, bytes.NewReader([]byte("world")))
	require.NoError(t, err)
	require.NoError(t, hello.Close())

	h := &Handler{vfs: V, billyFS: &FS{vfs: V}}
	h.opt.HandleLimit = 1000
	h.opt.HandleCache = cacheMemory
	cache, err := h.getCache()
	require.NoError(t, err)
	h.Cache = cache
	return h
}

func TestMountHandlerRoot(t *testing.T) {
	h := newTestHandler(t)
	status, fsh, _ := h.Mount(context.Background(), nil, nfs.MountRequest{Dirpath: []byte("/")})
	assert.Equal(t, nfs.MountStatusOk, status)
	rfs, ok := fsh.(*FS)
	require.True(t, ok)
	assert.Equal(t, "", rfs.root, "root mount must return the unrooted FS")
	assert.Same(t, h.billyFS, rfs)
}

func TestMountHandlerSubpath(t *testing.T) {
	h := newTestHandler(t)
	for _, raw := range []string{"/sub", "/sub/", "sub", "/./sub", "/foo/../sub"} {
		status, fsh, _ := h.Mount(context.Background(), nil, nfs.MountRequest{Dirpath: []byte(raw)})
		assert.Equal(t, nfs.MountStatusOk, status, "Dirpath %q should succeed", raw)
		rfs, ok := fsh.(*FS)
		require.True(t, ok, "Dirpath %q", raw)
		assert.Equal(t, "/sub", rfs.root, "Dirpath %q should land at /sub", raw)

		// The subpath FS should expose hello.txt at its root.
		entries, err := rfs.ReadDir("")
		require.NoError(t, err)
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		assert.ElementsMatch(t, []string{"hello.txt", "nested"}, names)
	}
}

func TestMountHandlerRejects(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		name    string
		dirpath string
		status  nfs.MountStatus
	}{
		{"missing", "/does-not-exist", nfs.MountStatusErrNoEnt},
		{"file", "/sub/hello.txt", nfs.MountStatusErrNotDir},
		{"deep-missing", "/sub/nope/deeper", nfs.MountStatusErrNoEnt},
		// path.Clean collapses ".." past the VFS root so this becomes /etc,
		// which does not exist in the VFS — proving traversal cannot escape.
		{"traversal", "/../../etc", nfs.MountStatusErrNoEnt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, fsh, _ := h.Mount(context.Background(), nil, nfs.MountRequest{Dirpath: []byte(tc.dirpath)})
			assert.Equal(t, tc.status, status)
			// onMount calls ToHandle on the returned filesystem regardless of
			// status, so the handler must return a non-nil billy.Filesystem
			// even on rejection. We unconditionally hand back the root FS.
			assert.NotNil(t, fsh)
		})
	}
}

// Subpath operations must end up at the right absolute VFS path. We
// exercise the round-trip by writing a file via the subpath FS and
// reading it back via the root FS.
func TestMountHandlerSubpathWrites(t *testing.T) {
	h := newTestHandler(t)
	_, fsh, _ := h.Mount(context.Background(), nil, nfs.MountRequest{Dirpath: []byte("/sub")})
	subFS := fsh.(*FS)

	wf, err := subFS.Create("greeting.txt")
	require.NoError(t, err)
	_, err = io.Copy(wf, bytes.NewReader([]byte("howdy")))
	require.NoError(t, err)
	require.NoError(t, wf.Close())

	// The same file must be visible at /sub/greeting.txt via the root VFS.
	node, err := h.vfs.Stat("/sub/greeting.txt")
	require.NoError(t, err)
	assert.False(t, node.IsDir())

	// And invisible at the VFS root.
	_, err = h.vfs.Stat("/greeting.txt")
	assert.Equal(t, vfs.ENOENT, err)
}

// Same file via root and subpath mounts must produce the same NFS handle.
func TestMountHandlerHandleStability(t *testing.T) {
	h := newTestHandler(t)
	_, rootFS, _ := h.Mount(context.Background(), nil, nfs.MountRequest{Dirpath: []byte("/")})
	_, subFS, _ := h.Mount(context.Background(), nil, nfs.MountRequest{Dirpath: []byte("/sub")})

	rootHandle := h.ToHandle(rootFS, []string{"sub", "hello.txt"})
	subHandle := h.ToHandle(subFS, []string{"hello.txt"})
	assert.Equal(t, rootHandle, subHandle,
		"a file's NFS handle must not depend on which mount reached it")
}
