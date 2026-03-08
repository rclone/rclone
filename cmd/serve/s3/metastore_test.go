package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMetadataStore runs a contract test suite against a metadataStore implementation.
func testMetadataStore(t *testing.T, store metadataStore) {
	t.Run("LoadMissing", func(t *testing.T) {
		_, ok := store.Load("bucket/no-such-key")
		assert.False(t, ok)
	})

	t.Run("StoreAndLoad", func(t *testing.T) {
		meta := map[string]string{"Content-Type": "text/plain", "X-Amz-Meta-Foo": "bar"}
		store.Store("bucket/key1", meta)

		got, ok := store.Load("bucket/key1")
		require.True(t, ok)
		assert.Equal(t, meta, got)
	})

	t.Run("StoreOverwrite", func(t *testing.T) {
		store.Store("bucket/key1", map[string]string{"a": "1"})
		store.Store("bucket/key1", map[string]string{"b": "2"})

		got, ok := store.Load("bucket/key1")
		require.True(t, ok)
		assert.Equal(t, map[string]string{"b": "2"}, got)
	})

	t.Run("IsolationBetweenBuckets", func(t *testing.T) {
		store.Store("alpha/obj", map[string]string{"x": "1"})
		store.Store("beta/obj", map[string]string{"x": "2"})

		got, ok := store.Load("alpha/obj")
		require.True(t, ok)
		assert.Equal(t, "1", got["x"])

		got, ok = store.Load("beta/obj")
		require.True(t, ok)
		assert.Equal(t, "2", got["x"])
	})

	t.Run("Delete", func(t *testing.T) {
		store.Store("bucket/del-me", map[string]string{"a": "1"})
		store.Delete("bucket/del-me")

		_, ok := store.Load("bucket/del-me")
		assert.False(t, ok)
	})

	t.Run("DeleteNonexistent", func(t *testing.T) {
		store.Delete("bucket/never-existed")
	})

	t.Run("DeleteAll", func(t *testing.T) {
		store.Store("delbucket/a", map[string]string{"k": "1"})
		store.Store("delbucket/b", map[string]string{"k": "2"})
		store.Store("keeper/c", map[string]string{"k": "3"})

		store.DeleteAll("delbucket")

		_, ok := store.Load("delbucket/a")
		assert.False(t, ok)
		_, ok = store.Load("delbucket/b")
		assert.False(t, ok)

		got, ok := store.Load("keeper/c")
		require.True(t, ok)
		assert.Equal(t, "3", got["k"])
	})

	t.Run("DeleteAllNonexistent", func(t *testing.T) {
		store.DeleteAll("no-such-bucket")
	})

	t.Run("Close", func(t *testing.T) {
		err := store.Close()
		assert.NoError(t, err)
	})
}

func TestMemoryMetaStore(t *testing.T) {
	testMetadataStore(t, newMemoryMetaStore())
}
