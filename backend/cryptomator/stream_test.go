package cryptomator_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

const cs = cryptomator.ChunkPayloadSize

type encryptedFile struct {
	CipherCombo string
	ContentKey  []byte
	Nonce       []byte
	MacKey      []byte
	Ciphertext  []byte
}

func TestDecryptReferenceStream(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "stream*.input"))
	assert.NoError(t, err)

	for _, path := range paths {
		filename := filepath.Base(path)
		testname := strings.TrimSuffix(filename, filepath.Ext(filename))

		input, err := os.ReadFile(path)
		assert.NoError(t, err)

		golden, err := os.ReadFile(filepath.Join("testdata", testname+".golden"))
		assert.NoError(t, err)

		var encFiles map[string]encryptedFile
		err = json.Unmarshal(input, &encFiles)
		assert.NoError(t, err)

		var plainTexts map[string][]byte
		err = json.Unmarshal(golden, &plainTexts)
		assert.NoError(t, err)

		for name, encFile := range encFiles {
			t.Run(fmt.Sprintf("%s:%s", testname, name), func(t *testing.T) {
				buf := bytes.NewBuffer(encFile.Ciphertext)
				key := cryptomator.MasterKey{EncryptKey: make([]byte, cryptomator.MasterEncryptKeySize), MacKey: encFile.MacKey}
				cryptor, err := cryptomator.NewCryptor(key, encFile.CipherCombo)
				assert.NoError(t, err)

				header := cryptomator.FileHeader{ContentKey: encFile.ContentKey, Nonce: encFile.Nonce}
				r, err := cryptor.NewReader(buf, header)
				assert.NoError(t, err)

				output, err := io.ReadAll(r)
				assert.NoError(t, err)

				assert.Equal(t, plainTexts[name], output)
			})
		}
	}
}

func TestStreamRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		stepSize := rapid.SampledFrom([]int{512, 600, 1000, cs}).Draw(t, "stepSize")
		// Maxlength due to memory problems when using math.MaxInt
		// maxLength := 1000000
		maxLength := 10000
		length := rapid.IntRange(0, maxLength).Draw(t, "length")

		src := fixedSizeByteArray(length).Draw(t, "src")
		cryptor := drawTestCryptor(t)
		nonce := fixedSizeByteArray(cryptor.NonceSize()).Draw(t, "nonce")
		contentKey := fixedSizeByteArray(cryptomator.HeaderContentKeySize).Draw(t, "contentKey")
		header := cryptomator.FileHeader{ContentKey: contentKey, Nonce: nonce}

		buf := &bytes.Buffer{}

		w, err := cryptor.NewWriter(buf, header)
		assert.NoError(t, err)

		n := 0
		for n < length {
			b := length - n
			if b > stepSize {
				b = stepSize
			}

			nn, err := w.Write(src[n : n+b])
			assert.NoError(t, err)
			assert.Equal(t, b, nn, "wrong number of bytes written")

			n += nn

			nn, err = w.Write(src[n:n])
			assert.NoError(t, err)
			assert.Zero(t, nn, "more than 0 bytes written")
		}

		err = w.Close()
		assert.NoError(t, err, "close returned an error")

		t.Logf("buffer size: %d", buf.Len())

		r, err := cryptor.NewReader(buf, header)
		assert.NoError(t, err)

		n = 0
		readBuf := make([]byte, stepSize)
		for n < length {
			nn, err := r.Read(readBuf)
			assert.NoErrorf(t, err, "read error at index %d", n)

			assert.Equalf(t, readBuf[:nn], src[n:n+nn], "wrong data at indexes %d - %d", n, n+nn)

			if nn == 0 {
				t.Fatal() // Avoid infinite loop
			}
			n += nn
		}
	})
}
