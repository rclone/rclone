package sshconfig

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
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
		result := ConvertToIniKey(customKey)
		if result != expectedIniKey {
			t.Errorf("expected: %s, got: %s", expectedIniKey, result)
		}
	}
}

func TestMapSshToRcloneConfig(t *testing.T) {
	r := strings.NewReader(sshConfigData)
	result, err := MapSshToRcloneConfig(r)

	require.NoError(t, err)
	c := *result

	assert.Equal(t, "127.1.1.20", c["one"]["hostname"])
	assert.Equal(t, "~/.ssh/id_ed123", c["one"]["identityfile"])
	assert.Equal(t, "localhost:8090 127.0.0.1:8190", c["tworclone"]["localforward"])
}

//func TestSshConfigFile(t *testing.T) {
//	defer setConfigFile(t, configData)()
//	data := &Storage{}
//
//	require.NoError(t, data.Load())
//
//	t.Run("Read", func(t *testing.T) {
//		t.Run("HasSection", func(t *testing.T) {
//			assert.True(t, data.HasSection("one"))
//			assert.False(t, data.HasSection("missing"))
//		})
//		t.Run("GetSectionList", func(t *testing.T) {
//			assert.Equal(t, []string{
//				"one",
//				"two",
//				"three",
//			}, data.GetSectionList())
//		})
//		t.Run("GetKeyList", func(t *testing.T) {
//			assert.Equal(t, []string{
//				"type",
//				"fruit",
//				"topping",
//			}, data.GetKeyList("two"))
//			assert.Equal(t, []string(nil), data.GetKeyList("unicorn"))
//		})
//		t.Run("GetValue", func(t *testing.T) {
//			value, ok := data.GetValue("one", "type")
//			assert.True(t, ok)
//			assert.Equal(t, "number1", value)
//			value, ok = data.GetValue("three", "fruit")
//			assert.True(t, ok)
//			assert.Equal(t, "banana", value)
//			value, ok = data.GetValue("one", "typeX")
//			assert.False(t, ok)
//			assert.Equal(t, "", value)
//			value, ok = data.GetValue("threeX", "fruit")
//			assert.False(t, ok)
//			assert.Equal(t, "", value)
//		})
//	})
//}

//func TestSshConfig(t *testing.T) {
//	defer setConfigFile(t, configData)()
//	s := &Storage{}
//
//	require.NoError(t, s.Load())
//
//	host1 := "host1"
//
//	sshConfig := useessshconfig.SshConfig{}
//	sshConfig[host1] = map[string]string{
//		"identityfile": "~/.ssh/id_rsa",
//		"hostname":     "example.com",
//		"user":         "user-1",
//		"port":         "22",
//	}
//	sshConfig["host2"] = map[string]string{
//		"hostname": "anotherexample.com",
//		"user":     "root",
//		"port":     "2222",
//	}
//
//	err := s.MergeSshConfig(sshConfig)
//	require.NoError(t, err)
//	assert.True(t, s.HasSection("one"))
//
//	assert.True(t, s.HasSection(host1))
//
//	value, ok := s.GetValue(host1, "type")
//	assert.True(t, ok)
//	assert.Equal(t, "sftp", value)
//	value, ok = s.GetValue(host1, "user")
//	assert.True(t, ok)
//	assert.Equal(t, "user-1", value)
//	value, ok = s.GetValue(host1, "host")
//	assert.True(t, ok)
//	assert.Equal(t, "example.com", value)
//	value, ok = s.GetValue(host1, "key_file")
//	assert.Equal(t, "~/.ssh/id_rsa", value)
//	//value, ok = s.GetValue(host1, "pubkey_file")
//	//assert.Equal(t, "~/.ssh/id_rsa.pub", value)
//	value, ok = s.GetValue(host1, "key_use_agent")
//	assert.Equal(t, "true", value)
//
//}
