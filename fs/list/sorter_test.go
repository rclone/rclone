package list

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSorter(t *testing.T) {
	ctx := context.Background()
	da := mockdir.New("a")
	oA := mockobject.Object("A")
	callback := func(entries fs.DirEntries) error {
		require.Equal(t, fs.DirEntries{oA, da}, entries)
		return nil
	}
	ls, err := NewSorter(ctx, callback, nil)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%p", callback), fmt.Sprintf("%p", ls.callback))
	assert.Equal(t, fmt.Sprintf("%p", identityKeyFn), fmt.Sprintf("%p", ls.keyFn))
	assert.Equal(t, fs.DirEntries(nil), ls.entries)

	// Test Add
	err = ls.Add(fs.DirEntries{da})
	require.NoError(t, err)
	assert.Equal(t, fs.DirEntries{da}, ls.entries)
	err = ls.Add(fs.DirEntries{oA})
	require.NoError(t, err)
	assert.Equal(t, fs.DirEntries{da, oA}, ls.entries)

	// Test Send
	err = ls.Send()
	require.NoError(t, err)

	// Test Cleanup
	ls.CleanUp()
	assert.Equal(t, fs.DirEntries(nil), ls.entries)
}

func TestSorterIdentity(t *testing.T) {
	ctx := context.Background()
	cmpFn := func(a, b fs.DirEntry) int {
		return cmp.Compare(a.Remote(), b.Remote())
	}
	callback := func(entries fs.DirEntries) error {
		assert.True(t, slices.IsSortedFunc(entries, cmpFn))
		assert.Equal(t, "a", entries[0].Remote())
		return nil
	}
	ls, err := NewSorter(ctx, callback, nil)
	require.NoError(t, err)
	defer ls.CleanUp()

	// Add things in reverse alphabetical order
	for i := 'z'; i >= 'a'; i-- {
		err = ls.Add(fs.DirEntries{mockobject.Object(string(i))})
		require.NoError(t, err)
	}
	assert.Equal(t, "z", ls.entries[0].Remote())
	assert.False(t, slices.IsSortedFunc(ls.entries, cmpFn))

	// Check they get sorted
	err = ls.Send()
	require.NoError(t, err)
}

func TestSorterKeyFn(t *testing.T) {
	ctx := context.Background()
	keyFn := func(entry fs.DirEntry) string {
		s := entry.Remote()
		return string('z' - s[0])
	}
	cmpFn := func(a, b fs.DirEntry) int {
		return cmp.Compare(keyFn(a), keyFn(b))
	}
	callback := func(entries fs.DirEntries) error {
		assert.True(t, slices.IsSortedFunc(entries, cmpFn))
		assert.Equal(t, "z", entries[0].Remote())
		return nil
	}
	ls, err := NewSorter(ctx, callback, keyFn)
	require.NoError(t, err)
	defer ls.CleanUp()

	// Add things in reverse sorted order
	for i := 'a'; i <= 'z'; i++ {
		err = ls.Add(fs.DirEntries{mockobject.Object(string(i))})
		require.NoError(t, err)
	}
	assert.Equal(t, "a", ls.entries[0].Remote())
	assert.False(t, slices.IsSortedFunc(ls.entries, cmpFn))

	// Check they get sorted
	err = ls.Send()
	require.NoError(t, err)
}
