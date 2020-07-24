package config

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
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
	oldPassword := Password
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
		Password = oldPassword
		fs.Config = oldConfig
		configFile = oldConfigFile

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

	// script for creating remote
	ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"y",                  // type my own password
		"secret",             // password
		"secret",             // repeat
		"y",                  // looks good, save
	})
	NewRemote("test")

	assert.Equal(t, []string{"test"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("test", "type"))
	assert.Equal(t, "true", FileGet("test", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(FileGet("test", "pass")))

	// normal rename, test → asdf
	ReadLine = makeReadLine([]string{
		"asdf",
		"asdf",
		"asdf",
	})
	RenameRemote("test")

	assert.Equal(t, []string{"asdf"}, configFile.GetSectionList())
	assert.Equal(t, "config_test_remote", FileGet("asdf", "type"))
	assert.Equal(t, "true", FileGet("asdf", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(FileGet("asdf", "pass")))

	// delete remote
	DeleteRemote("asdf")
	assert.Equal(t, []string{}, configFile.GetSectionList())
}

func TestChooseOption(t *testing.T) {
	defer testConfigFile(t, "crud.conf")()

	// script for creating remote
	ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"false",              // bool value
		"x",                  // bad choice
		"g",                  // generate password
		"1024",               // very big
		"y",                  // password OK
		"y",                  // looks good, save
	})
	Password = func(bits int) (string, error) {
		assert.Equal(t, 1024, bits)
		return "not very random password", nil
	}
	NewRemote("test")

	assert.Equal(t, "false", FileGet("test", "bool"))
	assert.Equal(t, "not very random password", obscure.MustReveal(FileGet("test", "pass")))

	// script for creating remote
	ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"n",                  // not required
		"y",                  // looks good, save
	})
	NewRemote("test")

	assert.Equal(t, "true", FileGet("test", "bool"))
	assert.Equal(t, "", FileGet("test", "pass"))
}

func TestNewRemoteName(t *testing.T) {
	defer testConfigFile(t, "crud.conf")()

	// script for creating remote
	ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"n",                  // not required
		"y",                  // looks good, save
	})
	NewRemote("test")

	ReadLine = makeReadLine([]string{
		"test",           // already exists
		"",               // empty string not allowed
		"bad@characters", // bad characters
		"newname",        // OK
	})

	assert.Equal(t, "newname", NewRemoteName())
}

func TestCreateUpatePasswordRemote(t *testing.T) {
	defer testConfigFile(t, "update.conf")()

	for _, doObscure := range []bool{false, true} {
		for _, noObscure := range []bool{false, true} {
			if doObscure && noObscure {
				break
			}
			t.Run(fmt.Sprintf("doObscure=%v,noObscure=%v", doObscure, noObscure), func(t *testing.T) {
				require.NoError(t, CreateRemote("test2", "config_test_remote", rc.Params{
					"bool": true,
					"pass": "potato",
				}, doObscure, noObscure))

				assert.Equal(t, []string{"test2"}, configFile.GetSectionList())
				assert.Equal(t, "config_test_remote", FileGet("test2", "type"))
				assert.Equal(t, "true", FileGet("test2", "bool"))
				gotPw := FileGet("test2", "pass")
				if !noObscure {
					gotPw = obscure.MustReveal(gotPw)
				}
				assert.Equal(t, "potato", gotPw)

				wantPw := obscure.MustObscure("potato2")
				require.NoError(t, UpdateRemote("test2", rc.Params{
					"bool":  false,
					"pass":  wantPw,
					"spare": "spare",
				}, doObscure, noObscure))

				assert.Equal(t, []string{"test2"}, configFile.GetSectionList())
				assert.Equal(t, "config_test_remote", FileGet("test2", "type"))
				assert.Equal(t, "false", FileGet("test2", "bool"))
				gotPw = FileGet("test2", "pass")
				if doObscure {
					gotPw = obscure.MustReveal(gotPw)
				}
				assert.Equal(t, wantPw, gotPw)

				require.NoError(t, PasswordRemote("test2", rc.Params{
					"pass": "potato3",
				}))

				assert.Equal(t, []string{"test2"}, configFile.GetSectionList())
				assert.Equal(t, "config_test_remote", FileGet("test2", "type"))
				assert.Equal(t, "false", FileGet("test2", "bool"))
				assert.Equal(t, "potato3", obscure.MustReveal(FileGet("test2", "pass")))
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

func TestConfigLoadEncryptedWithValidPassCommand(t *testing.T) {
	oldConfigPath := ConfigPath
	oldConfig := fs.Config
	ConfigPath = "./testdata/encrypted.conf"
	// using fs.Config.PasswordCommand, correct password
	fs.Config.PasswordCommand = fs.SpaceSepList{"echo", "asdf"}
	defer func() {
		ConfigPath = oldConfigPath
		configKey = nil // reset password
		fs.Config = oldConfig
		fs.Config.PasswordCommand = nil
	}()

	configKey = nil // reset password

	c, err := loadConfigFile()
	require.NoError(t, err)

	sections := c.GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

func TestConfigLoadEncryptedWithInvalidPassCommand(t *testing.T) {
	oldConfigPath := ConfigPath
	oldConfig := fs.Config
	ConfigPath = "./testdata/encrypted.conf"
	// using fs.Config.PasswordCommand, incorrect password
	fs.Config.PasswordCommand = fs.SpaceSepList{"echo", "asdf-blurfl"}
	defer func() {
		ConfigPath = oldConfigPath
		configKey = nil // reset password
		fs.Config = oldConfig
		fs.Config.PasswordCommand = nil
	}()

	configKey = nil // reset password

	_, err := loadConfigFile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "using --password-command derived password")
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
	}, false, false))
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
