// These are in an external package because we need to import configfile
//
// Internal tests are in crypt_internal_test.go

package config_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoadEncrypted(t *testing.T) {
	var err error
	oldConfigPath := config.ConfigPath
	config.ConfigPath = "./testdata/encrypted.conf"
	defer func() {
		config.ConfigPath = oldConfigPath
		config.ClearConfigPassword()
	}()

	// Set correct password
	err = config.SetConfigPassword("asdf")
	require.NoError(t, err)
	err = config.Data.Load()
	require.NoError(t, err)
	sections := config.Data.GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncryptedWithValidPassCommand(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.ConfigPath
	oldConfig := *ci
	config.ConfigPath = "./testdata/encrypted.conf"
	// using ci.PasswordCommand, correct password
	ci.PasswordCommand = fs.SpaceSepList{"echo", "asdf"}
	defer func() {
		config.ConfigPath = oldConfigPath
		config.ClearConfigPassword()
		*ci = oldConfig
		ci.PasswordCommand = nil
	}()

	config.ClearConfigPassword()

	err := config.Data.Load()
	require.NoError(t, err)

	sections := config.Data.GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncryptedWithInvalidPassCommand(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.ConfigPath
	oldConfig := *ci
	config.ConfigPath = "./testdata/encrypted.conf"
	// using ci.PasswordCommand, incorrect password
	ci.PasswordCommand = fs.SpaceSepList{"echo", "asdf-blurfl"}
	defer func() {
		config.ConfigPath = oldConfigPath
		config.ClearConfigPassword()
		*ci = oldConfig
		ci.PasswordCommand = nil
	}()

	config.ClearConfigPassword()

	err := config.Data.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "using --password-command derived password")
}

func TestConfigLoadEncryptedFailures(t *testing.T) {
	var err error

	// This file should be too short to be decoded.
	oldConfigPath := config.ConfigPath
	config.ConfigPath = "./testdata/enc-short.conf"
	defer func() { config.ConfigPath = oldConfigPath }()
	err = config.Data.Load()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	config.ConfigPath = "./testdata/enc-invalid.conf"
	err = config.Data.Load()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	config.ConfigPath = "./testdata/enc-too-new.conf"
	err = config.Data.Load()
	require.Error(t, err)

	// This file does not exist.
	config.ConfigPath = "./testdata/filenotfound.conf"
	err = config.Data.Load()
	assert.Equal(t, config.ErrorConfigFileNotFound, err)
}
