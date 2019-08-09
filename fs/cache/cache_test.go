package cache

import (
	"errors"
	"fmt"
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
	create := func(path string) (fs.Fs, error) {
		assert.Equal(t, 0, called)
		called++
		switch path {
		case "/":
			return mockfs.NewFs("mock", "mock"), nil
		case "/file.txt":
			return mockfs.NewFs("mock", "mock"), fs.ErrorIsFile
		case "/error":
			return nil, errSentinel
		}
		panic(fmt.Sprintf("Unknown path %q", path))
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

	f, err := GetFn("/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, c.Entries())

	f2, err := GetFn("/", create)
	require.NoError(t, err)

	assert.Equal(t, f, f2)
}

func TestGetFile(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	assert.Equal(t, 0, c.Entries())

	f, err := GetFn("/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f)

	assert.Equal(t, 1, c.Entries())

	f2, err := GetFn("/file.txt", create)
	require.Equal(t, fs.ErrorIsFile, err)
	require.NotNil(t, f2)

	assert.Equal(t, f, f2)
}

func TestGetError(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	assert.Equal(t, 0, c.Entries())

	f, err := GetFn("/error", create)
	require.Equal(t, errSentinel, err)
	require.Equal(t, nil, f)

	assert.Equal(t, 0, c.Entries())
}

func TestPut(t *testing.T) {
	cleanup, create := mockNewFs(t)
	defer cleanup()

	f := mockfs.NewFs("mock", "mock")

	assert.Equal(t, 0, c.Entries())

	Put("/alien", f)

	assert.Equal(t, 1, c.Entries())

	fNew, err := GetFn("/alien", create)
	require.NoError(t, err)
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, c.Entries())
}
