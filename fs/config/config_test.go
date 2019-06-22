package config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigFile(t *testing.T, configFileName string) func() {
	configKey = nil // reset password
	_ = os.Unsetenv("_RCLONE_CONFIG_KEY_FILE")
	_ = os.Unsetenv("RCLONE_CONFIG_PASS")
	// create temp config file
	tempFile, err := ioutil.TempFile("", configFileName)
	assert.NoError(t, err)
	path := tempFile.Name()
	assert.NoError(t, tempFile.Close())

	// temporarily adapt configuration
	oldOsStdout := os.Stdout
	oldConfigPath := ConfigPath
	oldConfig := fs.Config
	oldConfigFile := configFile
	oldReadLine := ReadLine
	os.Stdout = nil
	ConfigPath = path
	fs.Config = &fs.ConfigInfo{}
	configFile = nil

	LoadConfig()
	assert.Equal(t, []string{}, getConfigData().GetSectionList())

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
		ConfigPath = oldConfigPath
		ReadLine = oldReadLine
		fs.Config = oldConfig
		configFile = oldConfigFile

		_ = os.Unsetenv("_RCLONE_CONFIG_KEY_FILE")
		_ = os.Unsetenv("RCLONE_CONFIG_PASS")
	}
}

func TestCRUD(t *testing.T) {
	defer testConfigFile(t, "crud.conf")()

	// expect script for creating remote
	i := 0
	ReadLine = func() string {
		answers := []string{
			"config_test_remote", // type
			"true",               // bool value
			"y",                  // type my own password
			"secret",             // password
			"secret",             // repeat
			"y",                  // looks good, save
		}
		i = i + 1
		return answers[i-1]
	}

	NewRemote("test")

	assert.Equal(t, []string{"test"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("test", "type"))
	assert.Equal(t, "true", FileGet("test", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(FileGet("test", "pass")))

	// normal rename, test → asdf
	ReadLine = func() string { return "asdf" }
	RenameRemote("test")

	assert.Equal(t, []string{"asdf"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("asdf", "type"))
	assert.Equal(t, "true", FileGet("asdf", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(FileGet("asdf", "pass")))

	// no-op rename, asdf → asdf
	RenameRemote("asdf")

	assert.Equal(t, []string{"asdf"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("asdf", "type"))
	assert.Equal(t, "true", FileGet("asdf", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(FileGet("asdf", "pass")))

	// delete remote
	DeleteRemote("asdf")
	assert.Equal(t, []string{}, configFile.GetSectionList())
}

func TestCreateUpatePasswordRemote(t *testing.T) {
	defer testConfigFile(t, "update.conf")()

	require.NoError(t, CreateRemote("test2", "config_test_remote", rc.Params{
		"bool": true,
		"pass": "potato",
	}))

	assert.Equal(t, []string{"test2"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("test2", "type"))
	assert.Equal(t, "true", FileGet("test2", "bool"))
	assert.Equal(t, "potato", obscure.MustReveal(FileGet("test2", "pass")))

	require.NoError(t, UpdateRemote("test2", rc.Params{
		"bool":  false,
		"pass":  obscure.MustObscure("potato2"),
		"spare": "spare",
	}))

	assert.Equal(t, []string{"test2"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("test2", "type"))
	assert.Equal(t, "false", FileGet("test2", "bool"))
	assert.Equal(t, "potato2", obscure.MustReveal(FileGet("test2", "pass")))

	require.NoError(t, PasswordRemote("test2", rc.Params{
		"pass": "potato3",
	}))

	assert.Equal(t, []string{"test2"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("test2", "type"))
	assert.Equal(t, "false", FileGet("test2", "bool"))
	assert.Equal(t, "potato3", obscure.MustReveal(FileGet("test2", "pass")))
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
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/plain.conf"
	defer func() {
		ConfigPath = oldConfigPath
	}()
	configKey = nil // reset password
	c, err := loadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	sections := c.GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncrypted(t *testing.T) {
	var err error
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/encrypted.conf"
	defer func() {
		ConfigPath = oldConfigPath
		configKey = nil // reset password
	}()

	// Set correct password
	err = setConfigPassword("asdf")
	require.NoError(t, err)
	c, err := loadConfigFile()
	require.NoError(t, err)
	sections := c.GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncryptedFailures(t *testing.T) {
	var err error

	// This file should be too short to be decoded.
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/enc-short.conf"
	defer func() { ConfigPath = oldConfigPath }()
	_, err = loadConfigFile()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/enc-invalid.conf"
	_, err = loadConfigFile()
	require.Error(t, err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/enc-too-new.conf"
	_, err = loadConfigFile()
	require.Error(t, err)

	// This file does not exist.
	ConfigPath = "./testdata/filenotfound.conf"
	c, err := loadConfigFile()
	assert.Equal(t, errorConfigFileNotFound, err)
	assert.Nil(t, c)
}

func TestPassword(t *testing.T) {
	defer func() {
		configKey = nil // reset password
	}()
	var err error
	// Empty password should give error
	err = setConfigPassword("  \t  ")
	require.Error(t, err)

	// Test invalid utf8 sequence
	err = setConfigPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	require.Error(t, err)

	// Simple check of wrong passwords
	hashedKeyCompare(t, "mis", "match", false)

	// Check that passwords match after unicode normalization
	hashedKeyCompare(t, "ﬀ\u0041\u030A", "ffÅ", true)

	// Check that passwords preserves case
	hashedKeyCompare(t, "abcdef", "ABCDEF", false)

}

func hashedKeyCompare(t *testing.T, a, b string, shouldMatch bool) {
	err := setConfigPassword(a)
	require.NoError(t, err)
	k1 := configKey

	err = setConfigPassword(b)
	require.NoError(t, err)
	k2 := configKey

	if shouldMatch {
		assert.Equal(t, k1, k2)
	} else {
		assert.NotEqual(t, k1, k2)
	}
}

func TestMatchProvider(t *testing.T) {
	for _, test := range []struct {
		config   string
		provider string
		want     bool
	}{
		{"", "", true},
		{"one", "one", true},
		{"one,two", "two", true},
		{"one,two,three", "two", true},
		{"one", "on", false},
		{"one,two,three", "tw", false},
		{"!one,two,three", "two", false},
		{"!one,two,three", "four", true},
	} {
		what := fmt.Sprintf("%q,%q", test.config, test.provider)
		got := matchProvider(test.config, test.provider)
		assert.Equal(t, test.want, got, what)
	}
}

func TestFileRefresh(t *testing.T) {
	defer testConfigFile(t, "refresh.conf")()
	require.NoError(t, CreateRemote("refresh_test", "config_test_remote", rc.Params{
		"bool": true,
	}))
	b, err := ioutil.ReadFile(ConfigPath)
	assert.NoError(t, err)

	b = bytes.Replace(b, []byte("refresh_test"), []byte("refreshed_test"), 1)
	err = ioutil.WriteFile(ConfigPath, b, 0644)
	assert.NoError(t, err)

	assert.NotEqual(t, []string{"refreshed_test"}, configFile.GetSectionList())
	err = FileRefresh()
	assert.NoError(t, err)
	assert.Equal(t, []string{"refreshed_test"}, configFile.GetSectionList())
}
