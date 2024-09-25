package configfile

import (
	"testing"
)

var sshConfigData = `Host one
  hostname 127.1.1.20
  user onerclone
  preferredauthentications publickey
  identityfile ~/.ssh/id_ed25519

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
