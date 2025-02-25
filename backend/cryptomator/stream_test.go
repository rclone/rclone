package cryptomator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

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
				key := masterKey{EncryptKey: make([]byte, masterEncryptKeySize), MacKey: encFile.MacKey}
				cryptor, err := newCryptor(key, encFile.CipherCombo)
				assert.NoError(t, err)

				header := fileHeader{ContentKey: encFile.ContentKey, Nonce: encFile.Nonce}
				r, err := cryptor.newContentReader(buf, header)
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
		stepSize := rapid.SampledFrom([]int{512, 600, 1000, ChunkPayloadSize}).Draw(t, "stepSize")
		// Maxlength due to memory problems when using math.MaxInt
		// maxLength := 1000000
		maxLength := 10000
		length := rapid.IntRange(0, maxLength).Draw(t, "length")

		src := fixedSizeByteArray(length).Draw(t, "src")
		cryptor := drawTestCryptor(t)
		nonce := fixedSizeByteArray(cryptor.nonceSize()).Draw(t, "nonce")
		contentKey := fixedSizeByteArray(headerContentKeySize).Draw(t, "contentKey")
		header := fileHeader{ContentKey: contentKey, Nonce: nonce}

		buf := &bytes.Buffer{}

		w, err := cryptor.newContentWriter(buf, header)
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

		r, err := cryptor.newContentReader(buf, header)
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

func TestHeaderWriter(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxLength := 10000
		length := rapid.IntRange(0, maxLength).Draw(t, "length")
		data := fixedSizeByteArray(length).Draw(t, "src")

		cryptor := drawTestCryptor(t)

		buf := &bytes.Buffer{}
		w, err := cryptor.newWriter(buf)
		assert.NoError(t, err)

		_, err = w.Write(data)
		assert.NoError(t, err)
		err = w.Close()
		assert.NoError(t, err)

		header, err := cryptor.unmarshalHeader(buf)
		assert.NoError(t, err)
		r, err := cryptor.newContentReader(buf, header)
		assert.NoError(t, err)

		readBuf := make([]byte, length)
		_, err = io.ReadFull(r, readBuf)
		assert.NoError(t, err)
		assert.Equal(t, data, readBuf)
	})
}

func TestHeaderReader(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxLength := 10000
		length := rapid.IntRange(0, maxLength).Draw(t, "length")
		data := fixedSizeByteArray(length).Draw(t, "src")

		cryptor := drawTestCryptor(t)

		buf := &bytes.Buffer{}
		header, err := cryptor.NewHeader()
		assert.NoError(t, err)
		err = cryptor.marshalHeader(buf, header)
		assert.NoError(t, err)
		w, err := cryptor.newContentWriter(buf, header)
		assert.NoError(t, err)

		_, err = w.Write(data)
		assert.NoError(t, err)
		err = w.Close()
		assert.NoError(t, err)

		r, err := cryptor.newReader(buf)
		assert.NoError(t, err)

		readBuf := make([]byte, length)
		_, err = io.ReadFull(r, readBuf)
		assert.NoError(t, err)
		assert.Equal(t, data, readBuf)
	})
}

func TestEncryptedSize(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := drawMasterKey(t)
		cryptor, err := newCryptor(key, cipherComboSivGcm)
		assert.NoError(t, err)

		assert.EqualValues(t, 196, cryptor.encryptedFileSize(100))
		assert.EqualValues(t, 100, cryptor.decryptedFileSize(196))
	})
}
