package cache

import (
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

func mockNewFs(t *testing.T) (func(), func(path string) (fs.Fs, error)) {
	called = 0
	create := func(path string) (f fs.Fs, err error) {
		assert.Equal(t, 0, called)
		called++
		switch path {
		case "mock:/":
			return mockfs.NewFs("mock", "/"), nil
		case "mock:/file.txt", "mock:file.txt":
			return mockfs.NewFs("mock", "/"), fs.ErrorIsFile
		case "mock:/error":
			return nil, errSentinel
		}
		t.Fatalf("Unknown path %q", path)
		panic("unreachable")
	}
	cleanup := func() {
		c.Clear()
	}
	return cleanup, create
}

func TestGet(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	assert.Equal(t, 0, c.Entries())

	f, err := GetFn("mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, c.Entries())

	f2, err := GetFn("mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, f, f2)
}

func TestGetFile(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	assert.Equal(t, 0, c.Entries())

	f, err := GetFn("mock:/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)

	assert.Equal(t, 2, c.Entries())

	f2, err := GetFn("mock:/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)

	// check parent is there too
	f2, err = GetFn("mock:/", create)
	require.Nil(t, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)
}

func TestGetFile2(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	assert.Equal(t, 0, c.Entries())

	f, err := GetFn("mock:file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)

	assert.Equal(t, 2, c.Entries())

	f2, err := GetFn("mock:file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)

	// check parent is there too
	f2, err = GetFn("mock:/", create)
	require.Nil(t, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)
}

func TestGetError(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	assert.Equal(t, 0, c.Entries())

	f, err := GetFn("mock:/error", create)
	require.Equal(t, errSentinel, err)
	require.Equal(t, nil, f)

	assert.Equal(t, 0, c.Entries())
}

func TestPut(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	f := mockfs.NewFs("mock", "/alien")

	assert.Equal(t, 0, c.Entries())

	Put("mock:/alien", f)

	assert.Equal(t, 1, c.Entries())

	fNew, err := GetFn("mock:/alien", create)
	require.NoError(t, err)
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, c.Entries())

	// Check canonicalisation

	Put("mock:/alien/", f)

	fNew, err = GetFn("mock:/alien/", create)
	require.NoError(t, err)
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, c.Entries())

}

func TestPin(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	// Test pinning and unpinning non existent
	f := mockfs.NewFs("mock", "/alien")
	Pin(f)
	Unpin(f)

	// Now test pinning an existing
	f2, err := GetFn("mock:/", create)
	require.NoError(t, err)
	Pin(f2)
	Unpin(f2)
}

func TestClear(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	// Create something
	_, err := GetFn("mock:/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, c.Entries())

	Clear()

	assert.Equal(t, 0, c.Entries())
}
