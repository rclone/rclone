package cryptomator

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestHeaderNew(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cryptor := drawTestCryptor(t)
		h, err := cryptor.NewHeader()
		assert.NoError(t, err)

		assert.Len(t, h.Nonce, cryptor.nonceSize())
		assert.Len(t, h.ContentKey, headerContentKeySize)
		assert.Len(t, h.Reserved, headerReservedSize)

		assert.Equal(t, headerReservedValue, binary.BigEndian.Uint64(h.Reserved))
	})
}

func TestHeaderRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		buf := &bytes.Buffer{}
		cryptor := drawTestCryptor(t)

		h1, err := cryptor.NewHeader()
		assert.NoError(t, err)

		err = cryptor.marshalHeader(buf, h1)
		assert.NoError(t, err)

		assert.Len(t, buf.Bytes(), headerPayloadSize+cryptor.encryptionOverhead())

		h2, err := cryptor.unmarshalHeader(buf)
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

		var headers map[string]fileHeader
		err = json.Unmarshal(golden, &headers)
		assert.NoError(t, err)

		for name, encHeader := range encHeaders {
			t.Run(fmt.Sprintf("%s:%s", testname, name), func(t *testing.T) {
				key := masterKey{EncryptKey: encHeader.EncKey, MacKey: encHeader.MacKey}
				cryptor, err := newCryptor(key, encHeader.CipherCombo)
				assert.NoError(t, err)

				buf := bytes.NewBuffer(encHeader.Header)

				h, err := cryptor.unmarshalHeader(buf)
				assert.NoError(t, err)

				assert.Equal(t, headers[name], h)
			})
		}
	}
}
