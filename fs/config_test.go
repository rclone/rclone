package fs

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSizeSuffixString(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{102, "102"},
		{1024, "1k"},
		{1024 * 1024, "1M"},
		{1024 * 1024 * 1024, "1G"},
		{10 * 1024 * 1024 * 1024, "10G"},
		{10.1 * 1024 * 1024 * 1024, "10.100G"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.String()
		assert.Equal(t, test.want, got)
	}
}

func TestSizeSuffixUnit(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0 Bytes"},
		{102, "102 Bytes"},
		{1024, "1 kBytes"},
		{1024 * 1024, "1 MBytes"},
		{1024 * 1024 * 1024, "1 GBytes"},
		{10 * 1024 * 1024 * 1024, "10 GBytes"},
		{10.1 * 1024 * 1024 * 1024, "10.100 GBytes"},
		{-1, "off"},
		{-100, "off"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.Unit("Bytes")
		assert.Equal(t, test.want, got)
	}
}

func TestSizeSuffixSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"1b", 1, false},
		{"102B", 102, false},
		{"0.1k", 102, false},
		{"0.1", 102, false},
		{"1K", 1024, false},
		{"1", 1024, false},
		{"2.5", 1024 * 2.5, false},
		{"1M", 1024 * 1024, false},
		{"1.g", 1024 * 1024 * 1024, false},
		{"10G", 10 * 1024 * 1024 * 1024, false},
		{"off", -1, false},
		{"OFF", -1, false},
		{"", 0, true},
		{"1p", 0, true},
		{"1.p", 0, true},
		{"1p", 0, true},
		{"-1K", 0, true},
	} {
		ss := SizeSuffix(0)
		err := ss.Set(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		assert.Equal(t, test.want, int64(ss))
	}
}

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

// Test some error cases
func TestReveal(t *testing.T) {
	for _, test := range []struct {
		in      string
		wantErr string
	}{
		{"YmJiYmJiYmJiYmJiYmJiYp*gcEWbAw", "base64 decode failed: illegal base64 data at input byte 22"},
		{"aGVsbG8", "input too short"},
		{"", "input too short"},
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
	err = setPassword("asdf")
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

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/filenotfound.conf"
	c, err := loadConfigFile()
	require.NoError(t, err)
	require.Len(t, c.GetSectionList(), 0, "Expected 0-length section")
}

func TestPassword(t *testing.T) {
	defer func() {
		configKey = nil // reset password
	}()
	var err error
	// Empty password should give error
	err = setPassword("  \t  ")
	require.Error(t, err)

	// Test invalid utf8 sequence
	err = setPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	require.Error(t, err)

	// Simple check of wrong passwords
	hashedKeyCompare(t, "mis", "match", false)

	// Check that passwords match with trimmed whitespace
	hashedKeyCompare(t, "   abcdef   \t", "abcdef", true)

	// Check that passwords match after unicode normalization
	hashedKeyCompare(t, "ﬀ\u0041\u030A", "ffÅ", true)

	// Check that passwords preserves case
	hashedKeyCompare(t, "abcdef", "ABCDEF", false)

}

func hashedKeyCompare(t *testing.T, a, b string, shouldMatch bool) {
	err := setPassword(a)
	require.NoError(t, err)
	k1 := configKey

	err = setPassword(b)
	require.NoError(t, err)
	k2 := configKey

	if shouldMatch {
		assert.Equal(t, k1, k2)
	} else {
		assert.NotEqual(t, k1, k2)
	}
}
