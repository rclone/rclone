//go:build !plan9 && !js

package s3

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltMetaStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test-meta.db")
	store, err := newBoltMetaStore(dbPath)
	require.NoError(t, err)
	testMetadataStore(t, store)
}

func TestBoltMetaStorePersistence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist-meta.db")

	store, err := newBoltMetaStore(dbPath)
	require.NoError(t, err)
	store.Store("mybucket/mykey", map[string]string{"Content-Type": "application/json"})
	require.NoError(t, store.Close())

	store2, err := newBoltMetaStore(dbPath)
	require.NoError(t, err)
	defer func() { _ = store2.Close() }()

	got, ok := store2.Load("mybucket/mykey")
	require.True(t, ok)
	assert.Equal(t, "application/json", got["Content-Type"])
}
