//go:build unix

package nfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A renamed directory, and everything below it, keeps its handle; a removed
// file's handle goes stale; a file replaced by a rename goes stale.
func TestMemoryHandlerRename(t *testing.T) {
	f := &FS{}
	c := newMemoryHandler(100)
	f.renamed = c.renamed

	dir := c.ToHandle(f, []string{"jobs", "untitled folder"})
	child := c.ToHandle(f, []string{"jobs", "untitled folder", "inner", "file.txt"})
	sibling := c.ToHandle(f, []string{"jobs", "untitled folder 2"})
	target := c.ToHandle(f, []string{"jobs", "G"})

	// what go-nfs does for RENAME: rename, then invalidate the old handle
	c.renamed("/jobs/untitled folder", "/jobs/G")
	require.NoError(t, c.InvalidateHandle(f, dir))

	_, p, err := c.FromHandle(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"jobs", "G"}, p)
	assert.Equal(t, dir, c.ToHandle(f, []string{"jobs", "G"}), "new path resolves to the same handle")

	_, p, err = c.FromHandle(child)
	require.NoError(t, err)
	assert.Equal(t, []string{"jobs", "G", "inner", "file.txt"}, p)

	_, p, err = c.FromHandle(sibling)
	require.NoError(t, err)
	assert.Equal(t, []string{"jobs", "untitled folder 2"}, p, "a name sharing the prefix is not moved")

	_, _, err = c.FromHandle(target)
	assert.Equal(t, errStaleHandle, err, "the replaced target goes stale")

	assert.NotEqual(t, dir, c.ToHandle(f, []string{"jobs", "untitled folder"}), "the old path gets a new handle")

	// what go-nfs does for REMOVE
	require.NoError(t, c.InvalidateHandle(f, child))
	_, _, err = c.FromHandle(child)
	assert.Equal(t, errStaleHandle, err)
}

// Eviction keeps the path index in step with the LRU
func TestMemoryHandlerEviction(t *testing.T) {
	f := &FS{}
	c := newMemoryHandler(2)
	a := c.ToHandle(f, []string{"a"})
	c.ToHandle(f, []string{"b"})
	c.ToHandle(f, []string{"c"})
	_, _, err := c.FromHandle(a)
	assert.Equal(t, errStaleHandle, err)
	assert.Len(t, c.byPath, 2)
	assert.NotEqual(t, a, c.ToHandle(f, []string{"a"}))
}
