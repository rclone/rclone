package rc

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockNewFs(t *testing.T) func() {
	f := mockfs.NewFs(context.Background(), "mock", "mock")
	cache.Put("/", f)
	return func() {
		cache.Clear()
	}
}

func TestGetFsNamed(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"potato": "/",
	}
	f, err := GetFsNamed(context.Background(), in, "potato")
	require.NoError(t, err)
	assert.NotNil(t, f)

	in = Params{
		"sausage": "/",
	}
	f, err = GetFsNamed(context.Background(), in, "potato")
	require.Error(t, err)
	assert.Nil(t, f)
}

func TestGetFs(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"fs": "/",
	}
	f, err := GetFs(context.Background(), in)
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestGetFsAndRemoteNamed(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"fs":     "/",
		"remote": "hello",
	}
	f, remote, err := GetFsAndRemoteNamed(context.Background(), in, "fs", "remote")
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, "hello", remote)

	f, _, err = GetFsAndRemoteNamed(context.Background(), in, "fsX", "remote")
	require.Error(t, err)
	assert.Nil(t, f)

	f, _, err = GetFsAndRemoteNamed(context.Background(), in, "fs", "remoteX")
	require.Error(t, err)
	assert.Nil(t, f)

}

func TestGetFsAndRemote(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"fs":     "/",
		"remote": "hello",
	}
	f, remote, err := GetFsAndRemote(context.Background(), in)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, "hello", remote)

	t.Run("RcFscache", func(t *testing.T) {
		getEntries := func() int {
			call := Calls.Get("fscache/entries")
			require.NotNil(t, call)

			in := Params{}
			out, err := call.Fn(context.Background(), in)
			require.NoError(t, err)
			require.NotNil(t, out)
			return out["entries"].(int)
		}

		t.Run("Entries", func(t *testing.T) {
			assert.NotEqual(t, 0, getEntries())
		})

		t.Run("Clear", func(t *testing.T) {
			call := Calls.Get("fscache/clear")
			require.NotNil(t, call)

			in := Params{}
			out, err := call.Fn(context.Background(), in)
			require.NoError(t, err)
			require.Nil(t, out)
		})

		t.Run("Entries2", func(t *testing.T) {
			assert.Equal(t, 0, getEntries())
		})
	})
}
