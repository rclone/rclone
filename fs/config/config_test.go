// These are in an external package because we need to import configfile

package config_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoad(t *testing.T) {
	oldConfigPath := config.ConfigPath
	config.ConfigPath = "./testdata/plain.conf"
	defer func() {
		config.ConfigPath = oldConfigPath
	}()
	config.ClearConfigPassword()
	configfile.LoadConfig(context.Background())
	sections := config.Data.GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestFileRefresh(t *testing.T) {
	ctx := context.Background()
	defer testConfigFile(t, "refresh.conf")()
	require.NoError(t, config.CreateRemote(ctx, "refresh_test", "config_test_remote", rc.Params{
		"bool": true,
	}, false, false))
	b, err := ioutil.ReadFile(config.ConfigPath)
	assert.NoError(t, err)

	b = bytes.Replace(b, []byte("refresh_test"), []byte("refreshed_test"), 1)
	err = ioutil.WriteFile(config.ConfigPath, b, 0644)
	assert.NoError(t, err)

	assert.NotEqual(t, []string{"refreshed_test"}, config.Data.GetSectionList())
	err = config.FileRefresh()
	assert.NoError(t, err)
	assert.Equal(t, []string{"refreshed_test"}, config.Data.GetSectionList())
}
