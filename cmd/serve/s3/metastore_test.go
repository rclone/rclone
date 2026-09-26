package s3

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rec(meta map[string]string) metaRecord {
	return metaRecord{Meta: meta, Size: 1, ModTime: time.Unix(1, 0).UTC()}
}

// testMetadataStore runs a contract test suite against a metadataStore implementation.
func testMetadataStore(t *testing.T, store metadataStore) {
	load := func(t *testing.T, ns, fp string) (metaRecord, bool) {
		got, found, err := store.Load(ns, fp)
		require.NoError(t, err)
		return got, found
	}

	t.Run("LoadMissing", func(t *testing.T) {
		_, found := load(t, "ns", "bucket/no-such-key")
		assert.False(t, found)
	})

	t.Run("StoreAndLoad", func(t *testing.T) {
		want := rec(map[string]string{"Content-Type": "text/plain", "X-Amz-Meta-Foo": "bar"})
		require.NoError(t, store.Store("ns", "bucket/key1", want))

		got, found := load(t, "ns", "bucket/key1")
		require.True(t, found)
		assert.Equal(t, want, got)
	})

	t.Run("StoreCopiesMap", func(t *testing.T) {
		meta := map[string]string{"a": "1"}
		require.NoError(t, store.Store("ns", "bucket/copied", rec(meta)))
		meta["a"] = "2"

		got, found := load(t, "ns", "bucket/copied")
		require.True(t, found)
		assert.Equal(t, "1", got.Meta["a"])
	})

	t.Run("StoreOverwrite", func(t *testing.T) {
		require.NoError(t, store.Store("ns", "bucket/key1", rec(map[string]string{"a": "1"})))
		require.NoError(t, store.Store("ns", "bucket/key1", rec(map[string]string{"b": "2"})))

		got, found := load(t, "ns", "bucket/key1")
		require.True(t, found)
		assert.Equal(t, map[string]string{"b": "2"}, got.Meta)
	})

	t.Run("IsolationBetweenBuckets", func(t *testing.T) {
		require.NoError(t, store.Store("ns", "alpha/obj", rec(map[string]string{"x": "1"})))
		require.NoError(t, store.Store("ns", "beta/obj", rec(map[string]string{"x": "2"})))

		got, found := load(t, "ns", "alpha/obj")
		require.True(t, found)
		assert.Equal(t, "1", got.Meta["x"])

		got, found = load(t, "ns", "beta/obj")
		require.True(t, found)
		assert.Equal(t, "2", got.Meta["x"])
	})

	t.Run("IsolationBetweenNamespaces", func(t *testing.T) {
		require.NoError(t, store.Store("ns1", "bucket/obj", rec(map[string]string{"x": "1"})))

		_, found := load(t, "ns2", "bucket/obj")
		assert.False(t, found)

		require.NoError(t, store.DeleteAll("ns2", "bucket"))
		require.NoError(t, store.Delete("ns2", "bucket/obj"))
		_, found = load(t, "ns1", "bucket/obj")
		assert.True(t, found)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, store.Store("ns", "bucket/del-me", rec(map[string]string{"a": "1"})))
		require.NoError(t, store.Delete("ns", "bucket/del-me"))

		_, found := load(t, "ns", "bucket/del-me")
		assert.False(t, found)
	})

	t.Run("DeleteNonexistent", func(t *testing.T) {
		assert.NoError(t, store.Delete("ns", "bucket/never-existed"))
		assert.NoError(t, store.Delete("no-such-ns", "bucket/never-existed"))
	})

	t.Run("DeleteAll", func(t *testing.T) {
		require.NoError(t, store.Store("ns", "delbucket/a", rec(map[string]string{"k": "1"})))
		require.NoError(t, store.Store("ns", "delbucket/b", rec(map[string]string{"k": "2"})))
		require.NoError(t, store.Store("ns", "keeper/c", rec(map[string]string{"k": "3"})))

		require.NoError(t, store.DeleteAll("ns", "delbucket"))

		_, found := load(t, "ns", "delbucket/a")
		assert.False(t, found)
		_, found = load(t, "ns", "delbucket/b")
		assert.False(t, found)

		got, found := load(t, "ns", "keeper/c")
		require.True(t, found)
		assert.Equal(t, "3", got.Meta["k"])
	})

	t.Run("DeleteAllNonexistent", func(t *testing.T) {
		assert.NoError(t, store.DeleteAll("ns", "no-such-bucket"))
		assert.NoError(t, store.DeleteAll("no-such-ns", "no-such-bucket"))
	})
}

func TestMemoryMetaStore(t *testing.T) {
	store := newMemoryMetaStore()
	testMetadataStore(t, store)
	assert.NoError(t, store.Close())
}

// TestMetaStale checks metadata is not served once the file it was stored
// for has been changed without going through serve s3.
func TestMetaStale(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name   string
		change func(t *testing.T, _vfs *vfs.VFS, fp string)
	}{{
		name: "Size",
		change: func(t *testing.T, _vfs *vfs.VFS, fp string) {
			require.NoError(t, _vfs.WriteFile(fp, []byte("changed contents"), 0666))
		},
	}, {
		name: "ModTime",
		change: func(t *testing.T, _vfs *vfs.VFS, fp string) {
			ti := time.Now().Add(time.Hour)
			require.NoError(t, _vfs.Chtimes(fp, ti, ti))
		},
	}} {
		t.Run(test.name, func(t *testing.T) {
			b, _ := newTestBackend(t)
			_vfs, err := b.s.getVFS(ctx)
			require.NoError(t, err)
			_, err = b.PutObject(ctx, "bucket", "stale.txt", map[string]string{"X-Amz-Meta-Foo": "bar"}, strings.NewReader("data"), 4)
			require.NoError(t, err)
			head, err := b.HeadObject(ctx, "bucket", "stale.txt")
			require.NoError(t, err)
			require.Equal(t, "bar", head.Metadata["X-Amz-Meta-Foo"])

			test.change(t, _vfs, "bucket/stale.txt")

			head, err = b.HeadObject(ctx, "bucket", "stale.txt")
			require.NoError(t, err)
			assert.NotContains(t, head.Metadata, "X-Amz-Meta-Foo")
		})
	}
}

// TestMetaMtimeNotStale checks setting the modtime from the mtime metadata
// doesn't make the metadata look stale.
func TestMetaMtimeNotStale(t *testing.T) {
	ctx := context.Background()
	b, _ := newTestBackend(t)
	mtime := "1600000000.123456789"
	_, err := b.PutObject(ctx, "bucket", "mtime.txt", map[string]string{"X-Amz-Meta-Mtime": mtime, "X-Amz-Meta-Foo": "bar"}, strings.NewReader("data"), 4)
	require.NoError(t, err)

	head, err := b.HeadObject(ctx, "bucket", "mtime.txt")
	require.NoError(t, err)
	assert.Equal(t, "bar", head.Metadata["X-Amz-Meta-Foo"])
	assert.Equal(t, mtime, head.Metadata["X-Amz-Meta-Mtime"])
}

// TestMetaAuthProxyIsolation checks auth proxy users can neither see nor
// delete each other's metadata, even at the same path.
func TestMetaAuthProxyIsolation(t *testing.T) {
	fstest.Initialise()
	ctx := context.Background()
	opt := Opt
	opt.HTTP.ListenAddr = []string{endpoint}
	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "/path/to/auth/proxy"
	w, err := newServer(ctx, nil, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Shutdown() })

	userCtx := func(accessKey string) context.Context {
		f, err := fs.NewFs(ctx, t.TempDir())
		require.NoError(t, err)
		userVFS := vfs.New(ctx, f, &vfscommon.Opt)
		t.Cleanup(userVFS.Shutdown)
		userCtx := context.WithValue(ctx, ctxKeyID, userVFS)
		userCtx = context.WithValue(userCtx, ctxKeyAccessKey, accessKey)
		require.NoError(t, w.backend.CreateBucket(userCtx, "bucket"))
		return userCtx
	}
	alice, bob := userCtx("alice"), userCtx("bob")

	_, err = w.backend.PutObject(alice, "bucket", "object", map[string]string{"X-Amz-Meta-Owner": "alice"}, strings.NewReader("data"), 4)
	require.NoError(t, err)
	_, err = w.backend.PutObject(bob, "bucket", "object", nil, strings.NewReader("data"), 4)
	require.NoError(t, err)

	head, err := w.backend.HeadObject(bob, "bucket", "object")
	require.NoError(t, err)
	assert.NotContains(t, head.Metadata, "X-Amz-Meta-Owner")

	_, err = w.backend.DeleteObject(bob, "bucket", "object")
	require.NoError(t, err)
	require.NoError(t, w.backend.DeleteBucket(bob, "bucket"))

	head, err = w.backend.HeadObject(alice, "bucket", "object")
	require.NoError(t, err)
	assert.Equal(t, "alice", head.Metadata["X-Amz-Meta-Owner"])
}

type failingMetaStore struct {
	memoryMetaStore
}

var errMetaStore = errors.New("metadata store failed")

func (s *failingMetaStore) Store(ns, fp string, rec metaRecord) error {
	return errMetaStore
}

// TestMetaStoreErrorFailsUpload checks an upload whose metadata can't be
// stored is reported as failed rather than silently losing the metadata.
func TestMetaStoreErrorFailsUpload(t *testing.T) {
	ctx := context.Background()
	b, _ := newTestBackend(t)
	b.meta = &failingMetaStore{}

	_, err := b.PutObject(ctx, "bucket", "object", map[string]string{"X-Amz-Meta-Foo": "bar"}, strings.NewReader("data"), 4)
	assert.ErrorIs(t, err, errMetaStore)
	_, err = b.CopyObject(ctx, "bucket", "object.txt", "bucket", "object.txt", map[string]string{"X-Amz-Meta-Foo": "bar"})
	assert.ErrorIs(t, err, errMetaStore)
}
