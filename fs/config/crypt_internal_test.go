package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hashedKeyCompare(t *testing.T, a, b string, shouldMatch bool) {
	err := SetConfigPassword(a)
	require.NoError(t, err)
	k1 := configKey

	err = SetConfigPassword(b)
	require.NoError(t, err)
	k2 := configKey

	if shouldMatch {
		assert.Equal(t, k1, k2)
	} else {
		assert.NotEqual(t, k1, k2)
	}
}

func TestPassword(t *testing.T) {
	defer func() {
		configKey = nil // reset password
	}()
	var err error
	// Empty password should give error
	err = SetConfigPassword("  \t  ")
	require.Error(t, err)

	// Test invalid utf8 sequence
	err = SetConfigPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	require.Error(t, err)

	// Simple check of wrong passwords
	hashedKeyCompare(t, "mis", "match", false)

	// Check that passwords match after unicode normalization
	hashedKeyCompare(t, "ﬀ\u0041\u030A", "ffÅ", true)

	// Check that passwords preserves case
	hashedKeyCompare(t, "abcdef", "ABCDEF", false)

}

func TestChangeConfigPassword(t *testing.T) {
	ci := fs.GetConfig(context.Background())

	var err error
	oldConfigPath := GetConfigPath()
	assert.NoError(t, SetConfigPath("./testdata/encrypted.conf"))
	defer func() {
		assert.NoError(t, SetConfigPath(oldConfigPath))
		ClearConfigPassword()
		ci.PasswordCommand = nil
	}()

	// Get rid of any config password
	ClearConfigPassword()

	// Return the password, checking the state of the environment variable
	checkCode := `
package main

import (
	"fmt"
	"os"
	"log"
)

func main() {
	v := os.Getenv("RCLONE_PASSWORD_CHANGE")
	if v == "" {
		log.Fatal("Env var not found")
	} else if v != "1" {
		log.Fatal("Env var wrong value")
	} else {
		fmt.Println("asdf")
	}
}
`
	dir := t.TempDir()
	code := filepath.Join(dir, "file.go")
	require.NoError(t, os.WriteFile(code, []byte(checkCode), 0777))

	// Set correct password using --password-command
	ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
	changeConfigPassword()
	err = Data().Load()
	require.NoError(t, err)
	sections := Data().GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := Data().GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}
