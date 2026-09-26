//go:build !plan9 && !js

package s3

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltMetaStore(t *testing.T) {
	store, err := newBoltMetaStore(filepath.Join(t.TempDir(), "test-meta.db"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	testMetadataStore(t, store)
}

func TestBoltMetaStorePersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist-meta.db")

	store, err := newBoltMetaStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, store.Store("ns", "mybucket/mykey", rec(map[string]string{"Content-Type": "application/json"})))
	require.NoError(t, store.Close())

	store2, err := newBoltMetaStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = store2.Close() }()

	got, found, err := store2.Load("ns", "mybucket/mykey")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, rec(map[string]string{"Content-Type": "application/json"}), got)
}

func TestBoltMetaStoreLocked(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "locked-meta.db")
	store, err := newBoltMetaStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	_, err = newBoltMetaStore(dbPath)
	assert.Error(t, err)
}

// TestMetaDBServerRestart checks metadata survives a server restart with
// --meta-db, which also proves Shutdown releases the database lock.
func TestMetaDBServerRestart(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	f, err := fs.NewFs(ctx, root)
	require.NoError(t, err)
	require.NoError(t, f.Mkdir(ctx, "bucket"))

	opt := Opt
	opt.HTTP.ListenAddr = []string{endpoint}
	opt.MetaDB = filepath.Join(t.TempDir(), "meta.db")
	newTestServer := func() *Server {
		w, err := newServer(ctx, f, &opt, &vfscommon.Opt, &proxy.Opt)
		require.NoError(t, err)
		return w
	}

	w := newTestServer()
	content := []byte("hello")
	_, err = w.backend.PutObject(ctx, "bucket", "object.txt",
		map[string]string{"X-Amz-Meta-Foo": "bar"}, bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	require.NoError(t, w.Shutdown())

	w = newTestServer()
	t.Cleanup(func() { _ = w.Shutdown() })
	head, err := w.backend.HeadObject(ctx, "bucket", "object.txt")
	require.NoError(t, err)
	assert.Equal(t, "bar", head.Metadata["X-Amz-Meta-Foo"])

	_, err = w.backend.DeleteObject(ctx, "bucket", "object.txt")
	require.NoError(t, err)
	_vfs, err := w.getVFS(ctx)
	require.NoError(t, err)
	_, found, err := w.backend.meta.Load(metaNamespace(ctx, _vfs), "bucket/object.txt")
	require.NoError(t, err)
	assert.False(t, found)
}
