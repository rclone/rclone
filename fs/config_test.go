package fs

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObscure(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
		iv   string
	}{
		{"", "YWFhYWFhYWFhYWFhYWFhYQ", "aaaaaaaaaaaaaaaa"},
		{"potato", "YWFhYWFhYWFhYWFhYWFhYXMaGgIlEQ", "aaaaaaaaaaaaaaaa"},
		{"potato", "YmJiYmJiYmJiYmJiYmJiYp3gcEWbAw", "bbbbbbbbbbbbbbbb"},
	} {
		cryptRand = bytes.NewBufferString(test.iv)
		got, err := Obscure(test.in)
		cryptRand = rand.Reader
		assert.NoError(t, err)
		assert.Equal(t, test.want, got)
		recoveredIn, err := Reveal(got)
		assert.NoError(t, err)
		assert.Equal(t, test.in, recoveredIn, "not bidirectional")
		// Now the Must variants
		cryptRand = bytes.NewBufferString(test.iv)
		got = MustObscure(test.in)
		cryptRand = rand.Reader
		assert.Equal(t, test.want, got)
		recoveredIn = MustReveal(got)
		assert.Equal(t, test.in, recoveredIn, "not bidirectional")

	}
}

func TestCRUD(t *testing.T) {
	configKey = nil // reset password
	// create temp config file
	tempFile, err := ioutil.TempFile("", "crud.conf")
	assert.NoError(t, err)
	path := tempFile.Name()
	defer func() {
		err := os.Remove(path)
		assert.NoError(t, err)
	}()
	assert.NoError(t, tempFile.Close())

	// temporarily adapt configuration
	oldOsStdout := os.Stdout
	oldConfigFile := configFile
	oldConfig := Config
	oldConfigData := configData
	oldReadLine := ReadLine
	os.Stdout = nil
	configFile = &path
	Config = &ConfigInfo{}
	configData = nil
	defer func() {
		os.Stdout = oldOsStdout
		configFile = oldConfigFile
		ReadLine = oldReadLine
		Config = oldConfig
		configData = oldConfigData
	}()

	LoadConfig()
	assert.Equal(t, []string{}, configData.GetSectionList())

	// add new remote
	i := 0
	ReadLine = func() string {
		answers := []string{
			"local", // type is local
			"1",     // yes, disable long filenames
			"y",     // looks good, save
		}
		i = i + 1
		return answers[i-1]
	}
	NewRemote("test")
	assert.Equal(t, []string{"test"}, configData.GetSectionList())

	// normal rename, test → asdf
	ReadLine = func() string { return "asdf" }
	RenameRemote("test")
	assert.Equal(t, []string{"asdf"}, configData.GetSectionList())

	// no-op rename, asdf → asdf
	RenameRemote("asdf")
	assert.Equal(t, []string{"asdf"}, configData.GetSectionList())

	// delete remote
	DeleteRemote("asdf")
	assert.Equal(t, []string{}, configData.GetSectionList())
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
		gotString, gotErr := Reveal(test.in)
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

func TestDumpFlagsString(t *testing.T) {
	assert.Equal(t, "", DumpFlags(0).String())
	assert.Equal(t, "headers", (DumpHeaders).String())
	assert.Equal(t, "headers,bodies", (DumpHeaders | DumpBodies).String())
	assert.Equal(t, "headers,bodies,requests,responses,auth,filters", (DumpHeaders | DumpBodies | DumpRequests | DumpResponses | DumpAuth | DumpFilters).String())
	assert.Equal(t, "headers,Unknown-0x8000", (DumpHeaders | DumpFlags(0x8000)).String())
}

func TestDumpFlagsSet(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    DumpFlags
		wantErr string
	}{
		{"", DumpFlags(0), ""},
		{"bodies", DumpBodies, ""},
		{"bodies,headers,auth", DumpBodies | DumpHeaders | DumpAuth, ""},
		{"bodies,headers,auth", DumpBodies | DumpHeaders | DumpAuth, ""},
		{"headers,bodies,requests,responses,auth,filters", DumpHeaders | DumpBodies | DumpRequests | DumpResponses | DumpAuth | DumpFilters, ""},
		{"headers,bodies,unknown,auth", 0, "Unknown dump flag \"unknown\""},
	} {
		f := DumpFlags(-1)
		initial := f
		err := f.Set(test.in)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("Got an error when not expecting one on %q: %v", test.in, err)
			} else {
				assert.Contains(t, err.Error(), test.wantErr)
			}
			assert.Equal(t, initial, f, test.want)
		} else {
			if test.wantErr != "" {
				t.Errorf("Got no error when expecting one on %q", test.in)
			} else {
				assert.Equal(t, test.want, f)
			}
		}

	}
}

func TestDumpFlagsType(t *testing.T) {
	f := DumpFlags(0)
	assert.Equal(t, "string", f.Type())
}
