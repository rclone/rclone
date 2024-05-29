package rc

import (
	"context"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockNewFs(t *testing.T) func() {
	ctx := context.Background()
	f, err := mockfs.NewFs(ctx, "/", "", nil)
	require.NoError(t, err)
	cache.Put("/", f)
	f, err = mockfs.NewFs(ctx, "mock", "/", nil)
	require.NoError(t, err)
	cache.Put("mock:/", f)
	cache.Put(":mock:/", f)
	f, err = mockfs.NewFs(ctx, "mock", "dir/file.txt", nil)
	require.NoError(t, err)
	cache.PutErr("mock:dir/file.txt", f, fs.ErrorIsFile)
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

func TestGetFsNamedStruct(t *testing.T) {
	defer mockNewFs(t)()

	in := Params{
		"potato": Params{
			"type":  "mock",
			"_root": "/",
		},
	}
	f, err := GetFsNamed(context.Background(), in, "potato")
	require.NoError(t, err)
	assert.NotNil(t, f)

	in = Params{
		"potato": Params{
			"_name": "mock",
			"_root": "/",
		},
	}
	f, err = GetFsNamed(context.Background(), in, "potato")
	require.NoError(t, err)
	assert.NotNil(t, f)
}

func TestGetFsNamedFileOK(t *testing.T) {
	defer mockNewFs(t)()
	ctx := context.Background()

	in := Params{
		"potato": "/",
	}
	newCtx, f, err := GetFsNamedFileOK(ctx, in, "potato")
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, ctx, newCtx)

	in = Params{
		"sausage": "/",
	}
	newCtx, f, err = GetFsNamedFileOK(ctx, in, "potato")
	require.Error(t, err)
	assert.Nil(t, f)
	assert.Equal(t, ctx, newCtx)

	in = Params{
		"potato": "mock:dir/file.txt",
	}
	newCtx, f, err = GetFsNamedFileOK(ctx, in, "potato")
	assert.Nil(t, err)
	assert.NotNil(t, f)
	assert.NotEqual(t, ctx, newCtx)

	fi := filter.GetConfig(newCtx)
	assert.False(t, fi.InActive())
	assert.True(t, fi.IncludeRemote("file.txt"))
	assert.False(t, fi.IncludeRemote("other.txt"))
}

func TestGetConfigMap(t *testing.T) {
	for _, test := range []struct {
		in           Params
		fsName       string
		wantFsString string
		wantErr      string
	}{
		{
			in: Params{
				"Fs": Params{},
			},
			fsName:  "Fs",
			wantErr: `couldn't find "type" or "_name" in JSON config definition`,
		},
		{
			in: Params{
				"Fs": Params{
					"notastring": true,
				},
			},
			fsName:  "Fs",
			wantErr: `cannot unmarshal bool`,
		},
		{
			in: Params{
				"Fs": Params{
					"_name": "potato",
				},
			},
			fsName:       "Fs",
			wantFsString: "potato:",
		},
		{
			in: Params{
				"Fs": Params{
					"type": "potato",
				},
			},
			fsName:       "Fs",
			wantFsString: ":potato:",
		},
		{
			in: Params{
				"Fs": Params{
					"type":       "sftp",
					"_name":      "potato",
					"parameter":  "42",
					"parameter2": "true",
					"_root":      "/path/to/somewhere",
				},
			},
			fsName:       "Fs",
			wantFsString: "potato,parameter='42',parameter2='true':/path/to/somewhere",
		},
	} {
		gotFsString, gotErr := getConfigMap(test.in, test.fsName)
		what := fmt.Sprintf("%+v", test.in)
		assert.Equal(t, test.wantFsString, gotFsString, what)
		if test.wantErr == "" {
			assert.NoError(t, gotErr)
		} else {
			require.Error(t, gotErr)
			assert.Contains(t, gotErr.Error(), test.wantErr)

		}
	}
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
