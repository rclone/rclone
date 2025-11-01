package cryptomator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestNewMasterKey(t *testing.T) {
	k, err := newMasterKey()
	assert.NoError(t, err, "got an error while creating the master key")

	assert.Len(t, k.EncryptKey, masterEncryptKeySize, "invalid encryption key size")
	assert.Len(t, k.MacKey, masterMacKeySize, "invalid mac key size")
}

func TestMasterKeyRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		passphrase := rapid.String().Draw(t, "passphrase")

		k1, err := newMasterKey()
		assert.NoError(t, err, "got an error while creating the master key")

		buf := &bytes.Buffer{}

		err = k1.Marshal(buf, passphrase)
		assert.NoError(t, err, "got an error while marshalling")

		assert.NotEmpty(t, buf.Bytes(), "buffer is empty after marshalling")

		k2, err := unmarshalMasterKey(buf, passphrase)
		assert.NoError(t, err, "got an error while unmarshalling")

		assert.Empty(t, buf.Bytes(), "buffer is not empty after unmarshalling")

		assert.Equal(t, k1, k2)
	})
}

type encKey struct {
	EncryptedMasterKey []byte
	Passphrase         string
}

func TestMasterKeyUnmarshalReference(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "masterkey*.input"))
	assert.NoError(t, err)

	for _, path := range paths {
		filename := filepath.Base(path)
		testname := strings.TrimSuffix(filename, filepath.Ext(filename))

		input, err := os.ReadFile(path)
		assert.NoError(t, err)

		golden, err := os.ReadFile(filepath.Join("testdata", testname+".golden"))
		assert.NoError(t, err)

		var encKeys map[string]encKey
		err = json.Unmarshal(input, &encKeys)
		assert.NoError(t, err)

		var keys map[string]masterKey
		err = json.Unmarshal(golden, &keys)
		assert.NoError(t, err)

		for name, encKey := range encKeys {
			t.Run(fmt.Sprintf("%s:%s", testname, name), func(t *testing.T) {
				buf := bytes.NewBuffer(encKey.EncryptedMasterKey)

				h, err := unmarshalMasterKey(buf, encKey.Passphrase)
				assert.NoError(t, err)

				assert.Empty(t, buf.Bytes())

				assert.Equal(t, keys[name], h)
			})
		}
	}
}
