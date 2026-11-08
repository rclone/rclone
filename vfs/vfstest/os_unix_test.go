//go:build unix

package vfstest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntriesInfoSkipsVanished checks that an entry removed between
// reading the directory and stat-ing it is skipped rather than
// returning an error.
func TestEntriesInfoSkipsVanished(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"keep", "vanish"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	require.NoError(t, os.Remove(filepath.Join(dir, "vanish")))

	infos, err := entriesInfo(entries)
	require.NoError(t, err)
	require.Len(t, infos, 1)
	assert.Equal(t, "keep", infos[0].Name())
}
