package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testName = "configTestNameForRc"

func TestRc(t *testing.T) {
	ctx := context.Background()
	oldConfigFile := config.GetConfigPath()
	defer func() {
		require.NoError(t, config.SetConfigPath(oldConfigFile))
	}()
	// Set a temporary config file
	require.NoError(t, config.SetConfigPath(filepath.Join(t.TempDir(), "rclone.conf")))
	configfile.Install()
	// Create the test remote
	call := rc.Calls.Get("config/create")
	assert.NotNil(t, call)
	in := rc.Params{
		"name": testName,
		"type": "local",
		"parameters": rc.Params{
			"test_key": "sausage",
		},
	}
	out, err := call.Fn(ctx, in)
	require.NoError(t, err)
	require.Nil(t, out)
	assert.Equal(t, "local", config.GetValue(testName, "type"))
	assert.Equal(t, "sausage", config.GetValue(testName, "test_key"))

	// The sub tests rely on the remote created above but they can
	// all be run independently

	t.Run("Dump", func(t *testing.T) {
		call := rc.Calls.Get("config/dump")
		assert.NotNil(t, call)
		in := rc.Params{}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, out)

		require.NotNil(t, out[testName])
		config := out[testName].(rc.Params)

		assert.Equal(t, "local", config["type"])
		assert.Equal(t, "sausage", config["test_key"])
	})

	t.Run("Get", func(t *testing.T) {
		call := rc.Calls.Get("config/get")
		assert.NotNil(t, call)
		in := rc.Params{
			"name": testName,
		}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, out)

		assert.Equal(t, "local", out["type"])
		assert.Equal(t, "sausage", out["test_key"])
	})

	t.Run("ListRemotes", func(t *testing.T) {
		assert.NoError(t, os.Setenv("RCLONE_CONFIG_MY-LOCAL_TYPE", "local"))
		defer func() {
			assert.NoError(t, os.Unsetenv("RCLONE_CONFIG_MY-LOCAL_TYPE"))
		}()
		call := rc.Calls.Get("config/listremotes")
		assert.NotNil(t, call)
		in := rc.Params{}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, out)

		var remotes []string
		err = out.GetStruct("remotes", &remotes)
		require.NoError(t, err)

		assert.Contains(t, remotes, testName)
		assert.Contains(t, remotes, "my-local")
	})

	t.Run("Update", func(t *testing.T) {
		call := rc.Calls.Get("config/update")
		assert.NotNil(t, call)
		in := rc.Params{
			"name": testName,
			"parameters": rc.Params{
				"test_key":  "rutabaga",
				"test_key2": "cabbage",
			},
		}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		assert.Nil(t, out)

		assert.Equal(t, "local", config.GetValue(testName, "type"))
		assert.Equal(t, "rutabaga", config.GetValue(testName, "test_key"))
		assert.Equal(t, "cabbage", config.GetValue(testName, "test_key2"))
	})

	t.Run("Password", func(t *testing.T) {
		call := rc.Calls.Get("config/password")
		assert.NotNil(t, call)
		pw2 := obscure.MustObscure("password")
		in := rc.Params{
			"name": testName,
			"parameters": rc.Params{
				"test_key":  "rutabaga",
				"test_key2": pw2, // check we encode an already encoded password
			},
		}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		assert.Nil(t, out)

		assert.Equal(t, "local", config.GetValue(testName, "type"))
		assert.Equal(t, "rutabaga", obscure.MustReveal(config.GetValue(testName, "test_key")))
		assert.Equal(t, pw2, obscure.MustReveal(config.GetValue(testName, "test_key2")))
	})

	// Delete the test remote
	call = rc.Calls.Get("config/delete")
	assert.NotNil(t, call)
	in = rc.Params{
		"name": testName,
	}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Nil(t, out)
	assert.Equal(t, "", config.GetValue(testName, "type"))
	assert.Equal(t, "", config.GetValue(testName, "test_key"))

	t.Run("ListRemotes empty not nil", func(t *testing.T) {
		call := rc.Calls.Get("config/listremotes")
		assert.NotNil(t, call)
		in := rc.Params{}
		out, err := call.Fn(context.Background(), in)
		require.NoError(t, err)
		require.NotNil(t, out)

		var remotes []string
		err = out.GetStruct("remotes", &remotes)
		require.NoError(t, err)

		assert.NotNil(t, remotes)
		assert.Empty(t, remotes)
	})
}

func TestRcProviders(t *testing.T) {
	call := rc.Calls.Get("config/providers")
	assert.NotNil(t, call)
	in := rc.Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	var registry []*fs.RegInfo
	err = out.GetStruct("providers", &registry)
	require.NoError(t, err)
	foundLocal := false
	for _, provider := range registry {
		if provider.Name == "local" {
			foundLocal = true
			break
		}
	}
	assert.True(t, foundLocal, "didn't find local provider")
}

func TestRcSetPath(t *testing.T) {
	oldPath := config.GetConfigPath()
	newPath := oldPath + ".newPath"
	call := rc.Calls.Get("config/setpath")
	assert.NotNil(t, call)
	in := rc.Params{
		"path": newPath,
	}
	_, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, newPath, config.GetConfigPath())

	in["path"] = oldPath
	_, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, oldPath, config.GetConfigPath())
}

func TestRcPaths(t *testing.T) {
	call := rc.Calls.Get("config/paths")
	assert.NotNil(t, call)
	out, err := call.Fn(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, config.GetConfigPath(), out["config"])
	assert.Equal(t, config.GetCacheDir(), out["cache"])
	assert.Equal(t, os.TempDir(), out["temp"])
}

func TestRcConfigUnlock(t *testing.T) {
	call := rc.Calls.Get("config/unlock")
	assert.NotNil(t, call)
	in := rc.Params{
		"config_password": "test",
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)

	assert.Nil(t, err)
	assert.Nil(t, out)

}
