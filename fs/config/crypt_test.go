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
	oldConfigPath := config.GetConfigPath()
	assert.NoError(t, config.SetConfigPath("./testdata/encrypted.conf"))
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
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
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci
	assert.NoError(t, config.SetConfigPath("./testdata/encrypted.conf"))
	// using ci.PasswordCommand, correct password
	ci.PasswordCommand = fs.SpaceSepList{"echo", "asdf"}
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
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
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci
	assert.NoError(t, config.SetConfigPath("./testdata/encrypted.conf"))
	// using ci.PasswordCommand, incorrect password
	ci.PasswordCommand = fs.SpaceSepList{"echo", "asdf-blurfl"}
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
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
	oldConfigPath := config.GetConfigPath()
	assert.NoError(t, config.SetConfigPath("./testdata/enc-short.conf"))
	defer func() { assert.NoError(t, config.SetConfigPath(oldConfigPath)) }()
	err = config.Data.Load()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	assert.NoError(t, config.SetConfigPath("./testdata/enc-invalid.conf"))
	err = config.Data.Load()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	assert.NoError(t, config.SetConfigPath("./testdata/enc-too-new.conf"))
	err = config.Data.Load()
	require.Error(t, err)

	// This file does not exist.
	assert.NoError(t, config.SetConfigPath("./testdata/filenotfound.conf"))
	err = config.Data.Load()
	assert.Equal(t, config.ErrorConfigFileNotFound, err)
}
