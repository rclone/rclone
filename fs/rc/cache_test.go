package rc

import (
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var called = 0

func mockNewFs(t *testing.T) func() {
	called = 0
	oldFsNewFs := fsNewFs
	fsNewFs = func(path string) (fs.Fs, error) {
		assert.Equal(t, 0, called)
		called++
		assert.Equal(t, "/", path)
		return mockfs.NewFs("mock", "mock"), nil
	}
	return func() {
		fsNewFs = oldFsNewFs
		fsCacheMu.Lock()
		fsCache = map[string]*cacheEntry{}
		expireRunning = false
		fsCacheMu.Unlock()
	}
}

func TestGetCachedFs(t *testing.T) {
	defer mockNewFs(t)()

	assert.Equal(t, 0, len(fsCache))

	f, err := GetCachedFs("/")
	require.NoError(t, err)

	assert.Equal(t, 1, len(fsCache))

	f2, err := GetCachedFs("/")
	require.NoError(t, err)

	assert.Equal(t, f, f2)
}

func TestCacheExpire(t *testing.T) {
	defer mockNewFs(t)()

	cacheExpireInterval = time.Millisecond
	assert.Equal(t, false, expireRunning)

	_, err := GetCachedFs("/")
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

func TestGetFsNamed(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"potato": "/",
	}
	f, err := GetFsNamed(in, "potato")
	require.NoError(t, err)
	assert.NotNil(t, f)

	in = Params{
		"sausage": "/",
	}
	f, err = GetFsNamed(in, "potato")
	require.Error(t, err)
	assert.Nil(t, f)
}

func TestGetFs(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"fs": "/",
	}
	f, err := GetFs(in)
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestGetFsAndRemoteNamed(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"fs":     "/",
		"remote": "hello",
	}
	f, remote, err := GetFsAndRemoteNamed(in, "fs", "remote")
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, "hello", remote)

	f, _, err = GetFsAndRemoteNamed(in, "fsX", "remote")
	require.Error(t, err)
	assert.Nil(t, f)

	f, _, err = GetFsAndRemoteNamed(in, "fs", "remoteX")
	require.Error(t, err)
	assert.Nil(t, f)

}

func TestGetFsAndRemote(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"fs":     "/",
		"remote": "hello",
	}
	f, remote, err := GetFsAndRemote(in)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, "hello", remote)
}
