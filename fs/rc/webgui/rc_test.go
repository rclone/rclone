package webgui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPluginName = "rclone-test-plugin"
const testPluginAuthor = "rclone"
const testPluginKey = testPluginAuthor + "/" + testPluginName
const testPluginURL = "https://github.com/" + testPluginAuthor + "/" + testPluginName + "/"

func init() {
	rc.Opt.WebUI = true
}

func setCacheDir(t *testing.T) {
	cacheDir := t.TempDir()
	PluginsPath = filepath.Join(cacheDir, "plugins")
	pluginsConfigPath = filepath.Join(cacheDir, "config")

	loadedPlugins = newPlugins(availablePluginsJSONPath)
	err := loadedPlugins.readFromFile()
	assert.Nil(t, err)
}

func addPlugin(t *testing.T) {
	addPlugin := rc.Calls.Get("pluginsctl/addPlugin")
	assert.NotNil(t, addPlugin)
	in := rc.Params{
		"url": testPluginURL,
	}
	out, err := addPlugin.Fn(context.Background(), in)
	if err != nil && strings.Contains(err.Error(), "bad HTTP status") {
		t.Skipf("skipping test as plugin download failed: %v", err)
	}
	require.Nil(t, err)
	assert.Nil(t, out)

}

func removePlugin(t *testing.T) {
	addPlugin := rc.Calls.Get("pluginsctl/removePlugin")
	assert.NotNil(t, addPlugin)

	in := rc.Params{
		"name": testPluginKey,
	}
	out, err := addPlugin.Fn(context.Background(), in)
	assert.NotNil(t, err)
	assert.Nil(t, out)
}

//func TestListTestPlugins(t *testing.T) {
//	addPlugin := rc.Calls.Get("pluginsctl/listTestPlugins")
//	assert.NotNil(t, addPlugin)
//	in := rc.Params{}
//	out, err := addPlugin.Fn(context.Background(), in)
//	assert.Nil(t, err)
//	expected := rc.Params{
//		"loadedTestPlugins": map[string]PackageJSON{},
//	}
//	assert.Equal(t, expected, out)
//}

//func TestRemoveTestPlugin(t *testing.T) {
//	addPlugin := rc.Calls.Get("pluginsctl/removeTestPlugin")
//	assert.NotNil(t, addPlugin)
//	in := rc.Params{
//		"name": "",
//	}
//	out, err := addPlugin.Fn(context.Background(), in)
//	assert.NotNil(t, err)
//	assert.Nil(t, out)
//}

func TestAddPlugin(t *testing.T) {
	setCacheDir(t)

	addPlugin(t)
	_, ok := loadedPlugins.LoadedPlugins[testPluginKey]
	assert.True(t, ok)

	//removePlugin(t)
	//_, ok = loadedPlugins.LoadedPlugins[testPluginKey]
	//assert.False(t, ok)
}

func TestListPlugins(t *testing.T) {
	setCacheDir(t)

	addPlugin := rc.Calls.Get("pluginsctl/listPlugins")
	assert.NotNil(t, addPlugin)
	in := rc.Params{}
	out, err := addPlugin.Fn(context.Background(), in)
	assert.Nil(t, err)
	expected := rc.Params{
		"loadedPlugins":     map[string]PackageJSON{},
		"loadedTestPlugins": map[string]PackageJSON{},
	}
	assert.Equal(t, expected, out)
}

func TestRemovePlugin(t *testing.T) {
	setCacheDir(t)

	addPlugin(t)
	removePluginCall := rc.Calls.Get("pluginsctl/removePlugin")
	assert.NotNil(t, removePlugin)

	in := rc.Params{
		"name": testPluginKey,
	}
	out, err := removePluginCall.Fn(context.Background(), in)
	assert.Nil(t, err)
	assert.Nil(t, out)
	removePlugin(t)
	assert.Equal(t, len(loadedPlugins.LoadedPlugins), 0)

}

func TestPluginsForType(t *testing.T) {
	addPlugin := rc.Calls.Get("pluginsctl/getPluginsForType")
	assert.NotNil(t, addPlugin)
	in := rc.Params{
		"type":       "",
		"pluginType": "FileHandler",
	}
	out, err := addPlugin.Fn(context.Background(), in)
	assert.Nil(t, err)
	assert.NotNil(t, out)

	in = rc.Params{
		"type":       "video/mp4",
		"pluginType": "",
	}
	_, err = addPlugin.Fn(context.Background(), in)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}
