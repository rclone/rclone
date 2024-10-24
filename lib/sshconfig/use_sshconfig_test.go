package sshconfig

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sshConfigData = `Host one
  hostname 127.1.1.20
  user onerclone
  preferredauthentications publickey
  identityfile ~/.ssh/id_ed123

Host tworclone 
  hostname 127.1.1.40
  User vik
  LocalForward localhost:8090 127.0.0.1:8190

`

// Converts known custom keys to their corresponding INI keys
func TestConvertToIniKey(t *testing.T) {
	tests := map[string]string{
		"identityfile": "key_file",
		"pubkeyfile":   "pubkey_file",
		"user":         "user",
		"hostname":     "host",
		"port":         "port",
		"password":     "pass",
	}

	for customKey, expectedIniKey := range tests {
		result := convertToIniKey(customKey)
		if result != expectedIniKey {
			t.Errorf("expected: %s, got: %s", expectedIniKey, result)
		}
	}
}

func TestMapSshToRcloneConfig(t *testing.T) {
	r := strings.NewReader(sshConfigData)
	c, err := mapSSHToRcloneConfig(r)

	require.NoError(t, err)

	assert.Equal(t, "sftp", c["one"]["type"])
	assert.Equal(t, "sftp", c["tworclone"]["type"])
	assert.Equal(t, "127.1.1.20", c["one"]["host"])
	assert.Equal(t, "~/.ssh/id_ed123", c["one"]["key_file"])
	assert.Equal(t, "true", c["one"]["key_use_agent"])
	assert.Equal(t, "localhost:8090 127.0.0.1:8190", c["tworclone"]["localforward"])
	assert.Empty(t, c["towrclone"]["key_use_agent"])
}
