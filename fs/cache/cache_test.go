package cache

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	called      = 0
	errSentinel = errors.New("an error")
)

func mockNewFs(t *testing.T) func() {
	called = 0
	oldFsNewFs := fsNewFs
	fsNewFs = func(path string) (fs.Fs, error) {
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
	return func() {
		fsNewFs = oldFsNewFs
		fsCacheMu.Lock()
		fsCache = map[string]*cacheEntry{}
		expireRunning = false
		fsCacheMu.Unlock()
	}
}

func TestGet(t *testing.T) {
	defer mockNewFs(t)()

	assert.Equal(t, 0, len(fsCache))

	f, err := Get("/")
	require.NoError(t, err)

	assert.Equal(t, 1, len(fsCache))

	f2, err := Get("/")
	require.NoError(t, err)

	assert.Equal(t, f, f2)
}

func TestGetFile(t *testing.T) {
	defer mockNewFs(t)()

	assert.Equal(t, 0, len(fsCache))

	f, err := Get("/file.txt")
	require.Equal(t, fs.ErrorIsFile, err)

	assert.Equal(t, 1, len(fsCache))

	f2, err := Get("/file.txt")
	require.Equal(t, fs.ErrorIsFile, err)

	assert.Equal(t, f, f2)
}

func TestGetError(t *testing.T) {
	defer mockNewFs(t)()

	assert.Equal(t, 0, len(fsCache))

	f, err := Get("/error")
	require.Equal(t, errSentinel, err)
	require.Equal(t, nil, f)

	assert.Equal(t, 0, len(fsCache))
}

func TestPut(t *testing.T) {
	defer mockNewFs(t)()

	f := mockfs.NewFs("mock", "mock")

	assert.Equal(t, 0, len(fsCache))

	Put("/alien", f)

	assert.Equal(t, 1, len(fsCache))

	fNew, err := Get("/alien")
	require.NoError(t, err)
	require.Equal(t, f, fNew)

	assert.Equal(t, 1, len(fsCache))
}

func TestCacheExpire(t *testing.T) {
	defer mockNewFs(t)()

	cacheExpireInterval = time.Millisecond
	assert.Equal(t, false, expireRunning)

	_, err := Get("/")
	require.NoError(t, err)

	fsCacheMu.Lock()
	entry := fsCache["/"]

	assert.Equal(t, 1, len(fsCache))
	fsCacheMu.Unlock()
	cacheExpire()
	fsCacheMu.Lock()
	assert.Equal(t, 1, len(fsCache))
	entry.lastUsed = time.Now().Add(-cacheExpireDuration - 60*time.Second)
	assert.Equal(t, true, expireRunning)
	fsCacheMu.Unlock()
	time.Sleep(10 * time.Millisecond)
	fsCacheMu.Lock()
	assert.Equal(t, false, expireRunning)
	assert.Equal(t, 0, len(fsCache))
	fsCacheMu.Unlock()
}

func TestClear(t *testing.T) {
	defer mockNewFs(t)()

	assert.Equal(t, 0, len(fsCache))

	_, err := Get("/")
	require.NoError(t, err)

	assert.Equal(t, 1, len(fsCache))

	Clear()

	assert.Equal(t, 0, len(fsCache))
}
