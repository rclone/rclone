// These are in an external package because we need to import configfile
//
// Internal tests are in crypt_internal_test.go

package config_test

import (
	"context"
	"os"
	"path/filepath"
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
	assert.False(t, config.IsEncrypted())
	err = config.SetConfigPassword("asdf")
	require.NoError(t, err)
	assert.True(t, config.IsEncrypted())
	err = config.Data().Load()
	require.NoError(t, err)
	sections := config.Data().GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data().GetKeyList("nounc")
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

	err := config.Data().Load()
	require.NoError(t, err)

	sections := config.Data().GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := config.Data().GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

// TestConfigLoadEncryptedWithPassCommandAndDaemon checks the configKey handoff
// that --daemon relies on: the parent process leaves the obscured key in the
// temp file named by _RCLONE_CONFIG_KEY_FILE, and the daemon process uses that
// instead of running --password-command a second time.
func TestConfigLoadEncryptedWithPassCommandAndDaemon(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci
	require.NoError(t, config.SetConfigPath("./testdata/encrypted.conf"))
	config.PassConfigKeyForDaemonization = true
	t.Setenv("_RCLONE_CONFIG_KEY_FILE", "")
	defer func() {
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
		config.ClearConfigPassword()
		config.PassConfigKeyForDaemonization = false
		*ci = oldConfig
		ci.PasswordCommand = nil
	}()

	// The parent reads the password and saves the key for the daemon.
	ci.PasswordCommand = fs.SpaceSepList{"echo", "asdf"}
	config.ClearConfigPassword()
	require.NoError(t, config.Data().Load())

	keyFile := os.Getenv("_RCLONE_CONFIG_KEY_FILE")
	require.NotEmpty(t, keyFile)
	_, err := os.Stat(keyFile)
	require.NoError(t, err, "parent must leave the key file for the daemon")

	// The daemon inherits the environment but not the key. Running
	// --password-command again would yield this wrong password.
	config.ClearConfigPassword()
	ci.PasswordCommand = fs.SpaceSepList{"echo", "not-the-password"}
	require.NoError(t, config.Data().Load())

	assert.Equal(t, []string{"nounc", "unc"}, config.Data().GetSectionList())

	_, err = os.Stat(keyFile)
	assert.True(t, os.IsNotExist(err), "daemon must delete the key file once used")
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

	err := config.Data().Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "using --password-command derived password")
}

func TestConfigLoadEncryptedFailures(t *testing.T) {
	var err error

	// This file should be too short to be decoded.
	oldConfigPath := config.GetConfigPath()
	assert.NoError(t, config.SetConfigPath("./testdata/enc-short.conf"))
	defer func() { assert.NoError(t, config.SetConfigPath(oldConfigPath)) }()
	err = config.Data().Load()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	assert.NoError(t, config.SetConfigPath("./testdata/enc-invalid.conf"))
	err = config.Data().Load()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	assert.NoError(t, config.SetConfigPath("./testdata/enc-too-new.conf"))
	err = config.Data().Load()
	require.Error(t, err)

	// This file does not exist.
	assert.NoError(t, config.SetConfigPath("./testdata/filenotfound.conf"))
	err = config.Data().Load()
	assert.Equal(t, config.ErrorConfigFileNotFound, err)
}

func TestGetPasswordCommand(t *testing.T) {
	ctx, ci := fs.AddConfig(context.Background())

	// Not configured
	ci.PasswordCommand = fs.SpaceSepList{}
	pass, err := config.GetPasswordCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, "", pass)

	// With password - happy path
	ci.PasswordCommand = fs.SpaceSepList{"echo", "asdf"}
	pass, err = config.GetPasswordCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, "asdf", pass)

	// Empty password returned
	ci.PasswordCommand = fs.SpaceSepList{"echo", ""}
	_, err = config.GetPasswordCommand(ctx)
	assert.ErrorContains(t, err, "returned empty string")

	// Error when running command
	ci.PasswordCommand = fs.SpaceSepList{"XXX non-existent command XXX", ""}
	_, err = config.GetPasswordCommand(ctx)
	assert.ErrorContains(t, err, "not found")

	// Check the state of the environment variable in --password-command
	checkCode := `
package main

import (
	"fmt"
	"os"
)

func main() {
	if _, found := os.LookupEnv("RCLONE_PASSWORD_CHANGE"); found {
		fmt.Println("Env var set")
	} else {
		fmt.Println("OK")
	}
}
`
	dir := t.TempDir()
	code := filepath.Join(dir, "file.go")
	require.NoError(t, os.WriteFile(code, []byte(checkCode), 0777))

	// Check the environment variable unset when called directly
	ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
	pass, err = config.GetPasswordCommand(ctx)
	require.NoError(t, err)
	assert.Equal(t, "OK", pass)
}
