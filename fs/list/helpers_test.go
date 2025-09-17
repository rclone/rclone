package list

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock callback to collect the entries
func mockCallback(entries fs.DirEntries) error {
	// Do nothing or log for testing purposes
	return nil
}

func TestNewListRHelper(t *testing.T) {
	callback := mockCallback
	helper := NewHelper(callback)

	assert.NotNil(t, helper)
	assert.Equal(t, fmt.Sprintf("%p", callback), fmt.Sprintf("%p", helper.callback))
	assert.Empty(t, helper.entries)
}

func TestListRHelperAdd(t *testing.T) {
	callbackInvoked := false
	callback := func(entries fs.DirEntries) error {
		callbackInvoked = true
		return nil
	}

	helper := NewHelper(callback)
	entry := mockobject.Object("A")
	require.NoError(t, helper.Add(entry))

	assert.Len(t, helper.entries, 1)
	assert.False(t, callbackInvoked, "Callback should not be invoked before reaching 100 entries")

	// Check adding a nil entry doesn't change anything
	require.NoError(t, helper.Add(nil))
	assert.Len(t, helper.entries, 1)
	assert.False(t, callbackInvoked, "Callback should not be invoked before reaching 100 entries")
}

func TestListRHelperSend(t *testing.T) {
	entry := mockobject.Object("A")
	callbackInvoked := false
	callback := func(entries fs.DirEntries) error {
		callbackInvoked = true
		assert.Equal(t, 100, len(entries))
		for _, obj := range entries {
			assert.Equal(t, entry, obj)
		}
		return nil
	}

	helper := NewHelper(callback)

	// Add 100 entries to force the callback to be invoked
	for range 100 {
		require.NoError(t, helper.Add(entry))
	}

	assert.Len(t, helper.entries, 0)
	assert.True(t, callbackInvoked, "Callback should be invoked after 100 entries")
}

func TestListRHelperFlush(t *testing.T) {
	entry := mockobject.Object("A")
	callbackInvoked := false
	callback := func(entries fs.DirEntries) error {
		callbackInvoked = true
		assert.Equal(t, 1, len(entries))
		for _, obj := range entries {
			assert.Equal(t, entry, obj)
		}
		return nil
	}

	helper := NewHelper(callback)
	require.NoError(t, helper.Add(entry))
	assert.False(t, callbackInvoked, "Callback should not have been invoked yet")
	require.NoError(t, helper.Flush())

	assert.True(t, callbackInvoked, "Callback should be invoked on flush")
	assert.Len(t, helper.entries, 0, "Entries should be cleared after flush")
}

type mockListPfs struct {
	t          *testing.T
	entries    fs.DirEntries
	err        error
	errorAfter int
}

func (f *mockListPfs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	assert.Equal(f.t, "dir", dir)
	count := 0
	for entries := f.entries; len(entries) > 0; entries = entries[2:] {
		err = callback(entries[:2])
		if err != nil {
			return err
		}
		count += 2
		if f.err != nil && count >= f.errorAfter {
			return f.err
		}
	}
	return nil
}

// check interface
var _ fs.ListPer = (*mockListPfs)(nil)

func TestListWithListP(t *testing.T) {
	ctx := context.Background()
	var entries fs.DirEntries
	for i := range 26 {
		entries = append(entries, mockobject.New(fmt.Sprintf("%c", 'A'+i)))
	}
	t.Run("NoError", func(t *testing.T) {
		f := &mockListPfs{
			t:       t,
			entries: entries,
		}
		gotEntries, err := WithListP(ctx, "dir", f)
		require.NoError(t, err)
		assert.Equal(t, entries, gotEntries)
	})
	t.Run("Error", func(t *testing.T) {
		f := &mockListPfs{t: t,
			entries:    entries,
			err:        errors.New("BOOM"),
			errorAfter: 10,
		}
		gotEntries, err := WithListP(ctx, "dir", f)
		assert.Equal(t, f.err, err)
		assert.Equal(t, entries[:10], gotEntries)
	})
}
