// These are in an external package because we need to import configfile

package config_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
)

func TestConfigLoad(t *testing.T) {
	oldConfigPath := config.GetConfigPath()
	assert.NoError(t, config.SetConfigPath("./testdata/plain.conf"))
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
	}()
	config.ClearConfigPassword()
	configfile.Install()
	sections := config.Data().GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data().GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigCommandInLoadConfig(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci
	path := "./testdata/plain.conf"
	assert.NoError(t, config.SetConfigPath(""))
	// using ci.ConfigCommandIn, same .conf
	ci.ConfigCommandIn = fs.SpaceSepList{"cat", path}
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
		config.ClearConfigPassword()
		*ci = oldConfig
		ci.ConfigCommandIn = nil
	}()
	config.ClearConfigPassword()
	configfile.Install()

	sections := config.Data().GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data().GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)

	testConfigData := `[one]
type = number1
fruit = potato

`
	// using ci.ConfigCommandIn, different config
	ci.ConfigCommandIn = fs.SpaceSepList{"echo", testConfigData}

	sections = config.Data().GetSectionList()
	expect = []string{"one"}
	assert.Equal(t, expect, sections)

	keys = config.Data().GetKeyList("one")
	expect = []string{"type", "fruit"}
	assert.Equal(t, expect, keys)
}

func TestConfigCommandInLoadFailures(t *testing.T) {
	var err error
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci

	// Empty command should give an error
	ci.ConfigCommandIn = fs.SpaceSepList{""}
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
		*ci = oldConfig
		ci.ConfigCommandIn = nil
	}()
	err = config.Data().Load()
	require.Error(t, err)

	// Invalid command
	ci.ConfigCommandIn = fs.SpaceSepList{"NotAFunction", "NotParams"}
	err = config.Data().Load()
	require.Error(t, err)
}

func TestConfigCommandOutSaveConfig(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci
	path := "./testdata/save-test.conf"
	assert.NoError(t, config.SetConfigPath("./testdata/plain.conf"))
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
		config.ClearConfigPassword()
		*ci = oldConfig
		ci.ConfigCommandOut = nil
		assert.NoError(t, exec.Command("/bin/bash", "-c", "echo > "+path).Run())
	}()
	config.ClearConfigPassword()

	// clear save-test.conf
	assert.NoError(t, exec.Command("/bin/bash", "-c", "echo > "+path).Run())
	// using ci.ConfigCommandOut, save plain.conf data to save-test.conf
	ci.ConfigCommandOut = fs.SpaceSepList{"/bin/bash", "-c", "cat > " + path}
	configfile.Install()
	assert.NoError(t, config.Data().Save())

	plain, err := os.Stat("./testdata/plain.conf")
	assert.NoError(t, err)
	test, err := os.Stat(path)
	assert.NoError(t, err)
	assert.True(t, os.SameFile(plain, test))

	testConfigData := `[one]
type = number1
fruit = potato

`
	// using ci.ConfigCommandOut, different config
	ci.ConfigCommandOut = fs.SpaceSepList{"/bin/bash", "-c", "echo " + testConfigData + " > " + path}
	assert.NoError(t, config.Data().Save())

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.True(t, bytes.Equal([]byte(testConfigData), data))
}

func TestConfigCommandOutSaveFailures(t *testing.T) {
	var err error
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci

	// Empty command should give an error
	ci.ConfigCommandOut = fs.SpaceSepList{""}
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
		*ci = oldConfig
		ci.ConfigCommandOut = nil
	}()
	err = config.Data().Save()
	require.Error(t, err)

	// Invalid command
	ci.ConfigCommandOut = fs.SpaceSepList{"NotAFunction", "NotParams"}
	err = config.Data().Save()
	require.Error(t, err)
}
