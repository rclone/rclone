package config_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigFile(t *testing.T, configFileName string) func() {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	config.ClearConfigPassword()
	_ = os.Unsetenv("_RCLONE_CONFIG_KEY_FILE")
	_ = os.Unsetenv("RCLONE_CONFIG_PASS")
	// create temp config file
	tempFile, err := ioutil.TempFile("", configFileName)
	assert.NoError(t, err)
	path := tempFile.Name()
	assert.NoError(t, tempFile.Close())

	// temporarily adapt configuration
	oldOsStdout := os.Stdout
	oldConfigPath := config.ConfigPath
	oldConfig := *ci
	oldConfigFile := config.Data
	oldReadLine := config.ReadLine
	oldPassword := config.Password
	os.Stdout = nil
	config.ConfigPath = path
	ci = &fs.ConfigInfo{}

	configfile.LoadConfig(ctx)
	assert.Equal(t, []string{}, config.Data.GetSectionList())

	// Fake a remote
	fs.Register(&fs.RegInfo{
		Name: "config_test_remote",
		Options: fs.Options{
			{
				Name:       "bool",
				Default:    false,
				IsPassword: false,
			},
			{
				Name:       "pass",
				Default:    "",
				IsPassword: true,
			},
		},
	})

	// Undo the above
	return func() {
		err := os.Remove(path)
		assert.NoError(t, err)

		os.Stdout = oldOsStdout
		config.ConfigPath = oldConfigPath
		config.ReadLine = oldReadLine
		config.Password = oldPassword
		*ci = oldConfig
		config.Data = oldConfigFile

		_ = os.Unsetenv("_RCLONE_CONFIG_KEY_FILE")
		_ = os.Unsetenv("RCLONE_CONFIG_PASS")
	}
}

// makeReadLine makes a simple readLine which returns a fixed list of
// strings
func makeReadLine(answers []string) func() string {
	i := 0
	return func() string {
		i = i + 1
		return answers[i-1]
	}
}

func TestCRUD(t *testing.T) {
	defer testConfigFile(t, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"y",                  // type my own password
		"secret",             // password
		"secret",             // repeat
		"y",                  // looks good, save
	})
	config.NewRemote(ctx, "test")

	assert.Equal(t, []string{"test"}, config.Data.GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "true", config.FileGet("test", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(config.FileGet("test", "pass")))

	// normal rename, test → asdf
	config.ReadLine = makeReadLine([]string{
		"asdf",
		"asdf",
		"asdf",
	})
	config.RenameRemote("test")

	assert.Equal(t, []string{"asdf"}, config.Data.GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("asdf", "type"))
	assert.Equal(t, "true", config.FileGet("asdf", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(config.FileGet("asdf", "pass")))

	// delete remote
	config.DeleteRemote("asdf")
	assert.Equal(t, []string{}, config.Data.GetSectionList())
}

func TestChooseOption(t *testing.T) {
	defer testConfigFile(t, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"false",              // bool value
		"x",                  // bad choice
		"g",                  // generate password
		"1024",               // very big
		"y",                  // password OK
		"y",                  // looks good, save
	})
	config.Password = func(bits int) (string, error) {
		assert.Equal(t, 1024, bits)
		return "not very random password", nil
	}
	config.NewRemote(ctx, "test")

	assert.Equal(t, "false", config.FileGet("test", "bool"))
	assert.Equal(t, "not very random password", obscure.MustReveal(config.FileGet("test", "pass")))

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"n",                  // not required
		"y",                  // looks good, save
	})
	config.NewRemote(ctx, "test")

	assert.Equal(t, "true", config.FileGet("test", "bool"))
	assert.Equal(t, "", config.FileGet("test", "pass"))
}

func TestNewRemoteName(t *testing.T) {
	defer testConfigFile(t, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"n",                  // not required
		"y",                  // looks good, save
	})
	config.NewRemote(ctx, "test")

	config.ReadLine = makeReadLine([]string{
		"test",           // already exists
		"",               // empty string not allowed
		"bad@characters", // bad characters
		"newname",        // OK
	})

	assert.Equal(t, "newname", config.NewRemoteName())
}

func TestCreateUpdatePasswordRemote(t *testing.T) {
	ctx := context.Background()
	defer testConfigFile(t, "update.conf")()

	for _, doObscure := range []bool{false, true} {
		for _, noObscure := range []bool{false, true} {
			if doObscure && noObscure {
				break
			}
			t.Run(fmt.Sprintf("doObscure=%v,noObscure=%v", doObscure, noObscure), func(t *testing.T) {
				require.NoError(t, config.CreateRemote(ctx, "test2", "config_test_remote", rc.Params{
					"bool": true,
					"pass": "potato",
				}, doObscure, noObscure))

				assert.Equal(t, []string{"test2"}, config.Data.GetSectionList())
				assert.Equal(t, "config_test_remote", config.FileGet("test2", "type"))
				assert.Equal(t, "true", config.FileGet("test2", "bool"))
				gotPw := config.FileGet("test2", "pass")
				if !noObscure {
					gotPw = obscure.MustReveal(gotPw)
				}
				assert.Equal(t, "potato", gotPw)

				wantPw := obscure.MustObscure("potato2")
				require.NoError(t, config.UpdateRemote(ctx, "test2", rc.Params{
					"bool":  false,
					"pass":  wantPw,
					"spare": "spare",
				}, doObscure, noObscure))

				assert.Equal(t, []string{"test2"}, config.Data.GetSectionList())
				assert.Equal(t, "config_test_remote", config.FileGet("test2", "type"))
				assert.Equal(t, "false", config.FileGet("test2", "bool"))
				gotPw = config.FileGet("test2", "pass")
				if doObscure {
					gotPw = obscure.MustReveal(gotPw)
				}
				assert.Equal(t, wantPw, gotPw)

				require.NoError(t, config.PasswordRemote(ctx, "test2", rc.Params{
					"pass": "potato3",
				}))

				assert.Equal(t, []string{"test2"}, config.Data.GetSectionList())
				assert.Equal(t, "config_test_remote", config.FileGet("test2", "type"))
				assert.Equal(t, "false", config.FileGet("test2", "bool"))
				assert.Equal(t, "potato3", obscure.MustReveal(config.FileGet("test2", "pass")))
			})
		}
	}

}

// Test some error cases
func TestReveal(t *testing.T) {
	for _, test := range []struct {
		in      string
		wantErr string
	}{
		{"YmJiYmJiYmJiYmJiYmJiYp*gcEWbAw", "base64 decode failed when revealing password - is it obscured?: illegal base64 data at input byte 22"},
		{"aGVsbG8", "input too short when revealing password - is it obscured?"},
		{"", "input too short when revealing password - is it obscured?"},
	} {
		gotString, gotErr := obscure.Reveal(test.in)
		assert.Equal(t, "", gotString)
		assert.Equal(t, test.wantErr, gotErr.Error())
	}
}

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
