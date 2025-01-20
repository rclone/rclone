package cryptomator_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHeaderNew(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cryptor := drawTestCryptor(t)
		h, err := cryptor.NewHeader()
		assert.NoError(t, err)

		assert.Len(t, h.Nonce, cryptor.NonceSize())
		assert.Len(t, h.ContentKey, cryptomator.HeaderContentKeySize)
		assert.Len(t, h.Reserved, cryptomator.HeaderReservedSize)

		assert.Equal(t, cryptomator.HeaderReservedValue, binary.BigEndian.Uint64(h.Reserved))
	})
}

func TestHeaderRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		buf := &bytes.Buffer{}
		cryptor := drawTestCryptor(t)

		h1, err := cryptor.NewHeader()
		assert.NoError(t, err)

		err = cryptor.MarshalHeader(buf, h1)
		assert.NoError(t, err)

		assert.Len(t, buf.Bytes(), cryptomator.HeaderPayloadSize+cryptor.EncryptionOverhead())

		h2, err := cryptor.UnmarshalHeader(buf)
		assert.NoError(t, err)

		assert.Equal(t, h1, h2)
	})
}

type encHeader struct {
	CipherCombo string
	Header      []byte
	EncKey      []byte
	MacKey      []byte
}

func TestUnmarshalReferenceHeader(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "header*.input"))
	assert.NoError(t, err)

	for _, path := range paths {
		filename := filepath.Base(path)
		testname := strings.TrimSuffix(filename, filepath.Ext(filename))

		input, err := os.ReadFile(path)
		assert.NoError(t, err)

		golden, err := os.ReadFile(filepath.Join("testdata", testname+".golden"))
		assert.NoError(t, err)

		var encHeaders map[string]encHeader
		err = json.Unmarshal(input, &encHeaders)
		assert.NoError(t, err)

		var headers map[string]cryptomator.FileHeader
		err = json.Unmarshal(golden, &headers)
		assert.NoError(t, err)

		for name, encHeader := range encHeaders {
			t.Run(fmt.Sprintf("%s:%s", testname, name), func(t *testing.T) {
				key := cryptomator.MasterKey{EncryptKey: encHeader.EncKey, MacKey: encHeader.MacKey}
				cryptor, err := cryptomator.NewCryptor(key, encHeader.CipherCombo)
				assert.NoError(t, err)

				buf := bytes.NewBuffer(encHeader.Header)

				h, err := cryptor.UnmarshalHeader(buf)
				assert.NoError(t, err)

				assert.Equal(t, headers[name], h)
			})
		}
	}
}
