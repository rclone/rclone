// These are in an external package because we need to import configfile

package config_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/stretchr/testify/assert"
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
