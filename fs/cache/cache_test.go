package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	called      = 0
	errSentinel = errors.New("an error")
)

func mockNewFs(t *testing.T) func(ctx context.Context, path string) (fs.Fs, error) {
	called = 0
	create := func(ctx context.Context, path string) (f fs.Fs, err error) {
		assert.Equal(t, 0, called)
		called++
		switch path {
		case "mock:/":
			return mockfs.NewFs(ctx, "mock", "/", nil)
		case "mock:/file.txt", "mock:file.txt", "mock:/file2.txt", "mock:file2.txt":
			fMock, err := mockfs.NewFs(ctx, "mock", "/", nil)
			require.NoError(t, err)
			return fMock, fs.ErrorIsFile
		case "mock:/error":
			return nil, errSentinel
		}
		t.Fatalf("Unknown path %q", path)
		panic("unreachable")
	}
	t.Cleanup(Clear)
	return create
}

func TestGet(t *testing.T) {
	create := mockNewFs(t)

	assert.Equal(t, 0, Entries())

	f, err := GetFn(context.Background(), "mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, Entries())

	f2, err := GetFn(context.Background(), "mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, f, f2)
}

func TestGetFile(t *testing.T) {
	defer ClearMappings()
	create := mockNewFs(t)

	assert.Equal(t, 0, Entries())

	f, err := GetFn(context.Background(), "mock:/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)

	assert.Equal(t, 1, Entries())

	f2, err := GetFn(context.Background(), "mock:/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)

	// check it is also found when referred to by parent name
	f2, err = GetFn(context.Background(), "mock:/", create)
	require.Nil(t, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)
}

func TestGetFile2(t *testing.T) {
	defer ClearMappings()
	create := mockNewFs(t)

	assert.Equal(t, 0, Entries())

	f, err := GetFn(context.Background(), "mock:file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)

	assert.Equal(t, 1, Entries())

	f2, err := GetFn(context.Background(), "mock:file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)

	// check it is also found when referred to by parent name
	f2, err = GetFn(context.Background(), "mock:/", create)
	require.Nil(t, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)
}

func TestGetError(t *testing.T) {
	create := mockNewFs(t)

	assert.Equal(t, 0, Entries())

	f, err := GetFn(context.Background(), "mock:/error", create)
	require.Equal(t, errSentinel, err)
	require.Equal(t, nil, f)

	assert.Equal(t, 0, Entries())
}

func TestPutErr(t *testing.T) {
	create := mockNewFs(t)

	f, err := mockfs.NewFs(context.Background(), "mock", "", nil)
	require.NoError(t, err)

	assert.Equal(t, 0, Entries())

	PutErr("mock:/", f, fs.ErrorNotFoundInConfigFile)

	assert.Equal(t, 1, Entries())

	fNew, err := GetFn(context.Background(), "mock:/", create)
	require.True(t, errors.Is(err, fs.ErrorNotFoundInConfigFile))
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, Entries())

	// Check canonicalisation

	PutErr("mock:/file.txt", f, fs.ErrorNotFoundInConfigFile)

	fNew, err = GetFn(context.Background(), "mock:/file.txt", create)
	require.True(t, errors.Is(err, fs.ErrorNotFoundInConfigFile))
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, Entries())
}

func TestPut(t *testing.T) {
	create := mockNewFs(t)

	f, err := mockfs.NewFs(context.Background(), "mock", "/alien", nil)
	require.NoError(t, err)

	assert.Equal(t, 0, Entries())

	Put("mock:/alien", f)

	assert.Equal(t, 1, Entries())

	fNew, err := GetFn(context.Background(), "mock:/alien", create)
	require.NoError(t, err)
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, Entries())

	// Check canonicalisation

	Put("mock:/alien/", f)

	fNew, err = GetFn(context.Background(), "mock:/alien/", create)
	require.NoError(t, err)
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, Entries())
}

func TestPin(t *testing.T) {
	create := mockNewFs(t)

	// Test pinning and unpinning nonexistent
	f, err := mockfs.NewFs(context.Background(), "mock", "/alien", nil)
	require.NoError(t, err)
	Pin(f)
	Unpin(f)

	// Now test pinning an existing
	f2, err := GetFn(context.Background(), "mock:/", create)
	require.NoError(t, err)
	Pin(f2)
	Unpin(f2)
}

func TestPinFile(t *testing.T) {
	defer ClearMappings()
	create := mockNewFs(t)

	// Test pinning and unpinning nonexistent
	f, err := mockfs.NewFs(context.Background(), "mock", "/file.txt", nil)
	require.NoError(t, err)
	Pin(f)
	Unpin(f)

	// Now test pinning an existing
	f2, err := GetFn(context.Background(), "mock:/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	assert.Equal(t, 1, len(childParentMap))

	Pin(f2)
	assert.Equal(t, 1, Entries())
	pinned, unpinned := EntriesWithPinCount()
	assert.Equal(t, 1, pinned)
	assert.Equal(t, 0, unpinned)

	Unpin(f2)
	assert.Equal(t, 1, Entries())
	pinned, unpinned = EntriesWithPinCount()
	assert.Equal(t, 0, pinned)
	assert.Equal(t, 1, unpinned)

	// try a different child of the same parent, and parent
	// should not add additional cache items
	called = 0 // this one does create() because we haven't seen it before and don't yet know it's a file
	f3, err := GetFn(context.Background(), "mock:/file2.txt", create)
	assert.Equal(t, fs.ErrorIsFile, err)
	assert.Equal(t, 1, Entries())
	assert.Equal(t, 2, len(childParentMap))

	parent, err := GetFn(context.Background(), "mock:/", create)
	assert.NoError(t, err)
	assert.Equal(t, 1, Entries())
	assert.Equal(t, 2, len(childParentMap))

	Pin(f3)
	assert.Equal(t, 1, Entries())
	pinned, unpinned = EntriesWithPinCount()
	assert.Equal(t, 1, pinned)
	assert.Equal(t, 0, unpinned)

	Unpin(f3)
	assert.Equal(t, 1, Entries())
	pinned, unpinned = EntriesWithPinCount()
	assert.Equal(t, 0, pinned)
	assert.Equal(t, 1, unpinned)

	Pin(parent)
	assert.Equal(t, 1, Entries())
	pinned, unpinned = EntriesWithPinCount()
	assert.Equal(t, 1, pinned)
	assert.Equal(t, 0, unpinned)

	Unpin(parent)
	assert.Equal(t, 1, Entries())
	pinned, unpinned = EntriesWithPinCount()
	assert.Equal(t, 0, pinned)
	assert.Equal(t, 1, unpinned)

	// all 3 should have equal configstrings
	assert.Equal(t, fs.ConfigString(f2), fs.ConfigString(f3))
	assert.Equal(t, fs.ConfigString(f2), fs.ConfigString(parent))
}

func TestClearConfig(t *testing.T) {
	create := mockNewFs(t)

	assert.Equal(t, 0, Entries())

	_, err := GetFn(context.Background(), "mock:/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)

	assert.Equal(t, 1, Entries())

	assert.Equal(t, 1, ClearConfig("mock"))

	assert.Equal(t, 0, Entries())
}

func TestClear(t *testing.T) {
	create := mockNewFs(t)

	// Create something
	_, err := GetFn(context.Background(), "mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, Entries())

	Clear()

	assert.Equal(t, 0, Entries())
}

func TestEntries(t *testing.T) {
	create := mockNewFs(t)

	assert.Equal(t, 0, Entries())

	// Create something
	_, err := GetFn(context.Background(), "mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, Entries())
}
